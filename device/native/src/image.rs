// SPDX-License-Identifier: MIT

//! Bounded PNG normalization for native Xochitl image insertion.
//!
//! The model output is untrusted. This module decodes it with explicit memory
//! limits, discards metadata, converts every supported source to straight
//! RGBA8, optionally downsizes it, and publishes the result without following
//! symlinks or replacing an existing destination.

use png::{BitDepth, ColorType, Decoder, Encoder, Limits, Transformations};
use std::fs::{self, File, OpenOptions};
use std::io::{self, BufReader, Read, Write};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

const MAX_INPUT_BYTES: u64 = 32 * 1024 * 1024;
const MAX_SOURCE_DIMENSION: u32 = 8_192;
const MAX_SOURCE_PIXELS: u64 = 16_777_216;
const MAX_DECODED_BYTES: usize = 64 * 1024 * 1024;
const MAX_DECODER_WORK_BYTES: usize = 32 * 1024 * 1024;
const MAX_REQUESTED_DIMENSION: u32 = 4_096;
const MAX_OUTPUT_BYTES: usize = 32 * 1024 * 1024;

static TEMP_COUNTER: AtomicU64 = AtomicU64::new(0);

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct PreparedImage {
    pub width: u32,
    pub height: u32,
}

#[derive(Debug, Eq, PartialEq)]
struct RgbaImage {
    width: u32,
    height: u32,
    pixels: Vec<u8>,
}

/// Normalize `input` to a new, bounded RGBA8 PNG at `output`.
///
/// The destination must not already exist. The final file is published with a
/// same-directory hard link, giving create-if-absent semantics without ever
/// exposing a partially encoded PNG.
pub fn prepare_image(
    input: &Path,
    output: &Path,
    max_width: u32,
    max_height: u32,
) -> Result<PreparedImage, String> {
    validate_requested_bounds(max_width, max_height)?;
    ensure_destination_absent(output)?;

    let decoded = decode_png(input)?;
    let (width, height) = fitted_dimensions(decoded.width, decoded.height, max_width, max_height)?;
    let pixels = if (width, height) == (decoded.width, decoded.height) {
        decoded.pixels
    } else {
        resize_rgba8_premultiplied(
            &decoded.pixels,
            decoded.width,
            decoded.height,
            width,
            height,
        )?
    };

    encode_png_atomic(output, width, height, &pixels)?;
    Ok(PreparedImage { width, height })
}

fn validate_requested_bounds(width: u32, height: u32) -> Result<(), String> {
    if width == 0 || height == 0 {
        return Err("MAX_WIDTH and MAX_HEIGHT must both be greater than zero".into());
    }
    if width > MAX_REQUESTED_DIMENSION || height > MAX_REQUESTED_DIMENSION {
        return Err(format!(
            "requested image bounds exceed {}x{}",
            MAX_REQUESTED_DIMENSION, MAX_REQUESTED_DIMENSION
        ));
    }
    let pixels = u64::from(width)
        .checked_mul(u64::from(height))
        .ok_or_else(|| "requested image bounds overflow".to_string())?;
    if pixels > MAX_SOURCE_PIXELS {
        return Err(format!(
            "requested image bounds exceed the {} pixel limit",
            MAX_SOURCE_PIXELS
        ));
    }
    Ok(())
}

fn ensure_destination_absent(path: &Path) -> Result<(), String> {
    if path.file_name().is_none() {
        return Err(format!("output path has no file name: {}", path.display()));
    }
    match fs::symlink_metadata(path) {
        Ok(_) => Err(format!(
            "refusing to replace existing output: {}",
            path.display()
        )),
        Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(format!("cannot inspect output {}: {error}", path.display())),
    }
}

fn open_bounded_regular_file(path: &Path) -> Result<File, String> {
    let link_metadata = fs::symlink_metadata(path)
        .map_err(|e| format!("cannot inspect input {}: {e}", path.display()))?;
    if link_metadata.file_type().is_symlink() {
        return Err(format!("refusing symlink input: {}", path.display()));
    }
    if !link_metadata.file_type().is_file() {
        return Err(format!("input is not a regular file: {}", path.display()));
    }

    let mut options = OpenOptions::new();
    options.read(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.custom_flags(libc::O_CLOEXEC | libc::O_NOFOLLOW);
    }
    let file = options
        .open(path)
        .map_err(|e| format!("cannot open input {}: {e}", path.display()))?;
    let metadata = file
        .metadata()
        .map_err(|e| format!("cannot inspect open input {}: {e}", path.display()))?;
    if !metadata.file_type().is_file() {
        return Err(format!(
            "open input is not a regular file: {}",
            path.display()
        ));
    }
    if metadata.len() == 0 {
        return Err(format!("input PNG is empty: {}", path.display()));
    }
    if metadata.len() > MAX_INPUT_BYTES {
        return Err(format!(
            "input PNG exceeds the {} byte limit",
            MAX_INPUT_BYTES
        ));
    }
    Ok(file)
}

fn decode_png(path: &Path) -> Result<RgbaImage, String> {
    let file = open_bounded_regular_file(path)?;
    // The Take guard also bounds a file that grows after the fstat above.
    let input = BufReader::new(file.take(MAX_INPUT_BYTES + 1));
    let limits = Limits {
        bytes: MAX_DECODER_WORK_BYTES,
    };
    let mut decoder = Decoder::new_with_limits(input, limits);
    decoder.set_transformations(Transformations::normalize_to_color8());
    decoder.set_ignore_text_chunk(true);
    decoder.set_ignore_iccp_chunk(true);

    let mut reader = decoder
        .read_info()
        .map_err(|e| format!("invalid PNG header: {e}"))?;
    if reader.info().animation_control.is_some() {
        return Err("animated PNG is not supported".into());
    }
    validate_source_dimensions(reader.info().width, reader.info().height)?;

    let buffer_size = reader.output_buffer_size();
    if buffer_size == 0 || buffer_size > MAX_DECODED_BYTES {
        return Err(format!(
            "decoded PNG requires {buffer_size} bytes; limit is {MAX_DECODED_BYTES}"
        ));
    }
    let mut decoded = vec![0u8; buffer_size];
    let info = reader
        .next_frame(&mut decoded)
        .map_err(|e| format!("cannot decode PNG pixels: {e}"))?;
    reader
        .finish()
        .map_err(|e| format!("invalid PNG trailer: {e}"))?;
    if info.bit_depth != BitDepth::Eight {
        return Err(format!(
            "PNG normalization returned unsupported {:?} depth",
            info.bit_depth
        ));
    }
    validate_source_dimensions(info.width, info.height)?;
    decoded.truncate(info.buffer_size());
    let pixels = convert_to_rgba8(&decoded, info.color_type, info.width, info.height)?;
    Ok(RgbaImage {
        width: info.width,
        height: info.height,
        pixels,
    })
}

fn validate_source_dimensions(width: u32, height: u32) -> Result<(), String> {
    if width == 0 || height == 0 {
        return Err("PNG dimensions must be non-zero".into());
    }
    if width > MAX_SOURCE_DIMENSION || height > MAX_SOURCE_DIMENSION {
        return Err(format!(
            "PNG dimensions {width}x{height} exceed the {} pixel side limit",
            MAX_SOURCE_DIMENSION
        ));
    }
    let pixels = u64::from(width)
        .checked_mul(u64::from(height))
        .ok_or_else(|| "PNG dimensions overflow".to_string())?;
    if pixels > MAX_SOURCE_PIXELS {
        return Err(format!(
            "PNG dimensions {width}x{height} exceed the {} pixel limit",
            MAX_SOURCE_PIXELS
        ));
    }
    Ok(())
}

fn convert_to_rgba8(
    input: &[u8],
    color_type: ColorType,
    width: u32,
    height: u32,
) -> Result<Vec<u8>, String> {
    let pixel_count = usize::try_from(u64::from(width) * u64::from(height))
        .map_err(|_| "PNG pixel count does not fit memory".to_string())?;
    let expected = pixel_count
        .checked_mul(color_type.samples())
        .ok_or_else(|| "PNG sample count overflow".to_string())?;
    if input.len() != expected {
        return Err(format!(
            "decoded PNG length mismatch: got {}, expected {expected}",
            input.len()
        ));
    }
    let output_len = pixel_count
        .checked_mul(4)
        .ok_or_else(|| "RGBA output size overflow".to_string())?;
    if output_len > MAX_DECODED_BYTES {
        return Err("RGBA output exceeds the decoded byte limit".into());
    }
    let mut output = Vec::with_capacity(output_len);
    match color_type {
        ColorType::Rgba => output.extend_from_slice(input),
        ColorType::Rgb => {
            for pixel in input.chunks_exact(3) {
                output.extend_from_slice(&[pixel[0], pixel[1], pixel[2], 255]);
            }
        }
        ColorType::Grayscale => {
            for &gray in input {
                output.extend_from_slice(&[gray, gray, gray, 255]);
            }
        }
        ColorType::GrayscaleAlpha => {
            for pixel in input.chunks_exact(2) {
                output.extend_from_slice(&[pixel[0], pixel[0], pixel[0], pixel[1]]);
            }
        }
        ColorType::Indexed => {
            return Err("indexed PNG was not expanded by the decoder".into());
        }
    }
    if output.len() != output_len {
        return Err("RGBA conversion produced an unexpected byte count".into());
    }
    Ok(output)
}

fn fitted_dimensions(
    width: u32,
    height: u32,
    max_width: u32,
    max_height: u32,
) -> Result<(u32, u32), String> {
    validate_source_dimensions(width, height)?;
    validate_requested_bounds(max_width, max_height)?;
    if width <= max_width && height <= max_height {
        return Ok((width, height));
    }

    let width_limited =
        u64::from(max_width) * u64::from(height) <= u64::from(max_height) * u64::from(width);
    let (out_width, out_height) = if width_limited {
        let scaled_height =
            (u64::from(height) * u64::from(max_width) + u64::from(width) / 2) / u64::from(width);
        (
            max_width,
            u32::try_from(scaled_height).unwrap_or(max_height),
        )
    } else {
        let scaled_width =
            (u64::from(width) * u64::from(max_height) + u64::from(height) / 2) / u64::from(height);
        (u32::try_from(scaled_width).unwrap_or(max_width), max_height)
    };
    Ok((
        out_width.clamp(1, max_width),
        out_height.clamp(1, max_height),
    ))
}

fn resize_rgba8_premultiplied(
    input: &[u8],
    source_width: u32,
    source_height: u32,
    target_width: u32,
    target_height: u32,
) -> Result<Vec<u8>, String> {
    if target_width == 0
        || target_height == 0
        || target_width > source_width
        || target_height > source_height
    {
        return Err("resize target must be non-zero and may not enlarge the image".into());
    }
    let source_len = usize::try_from(u64::from(source_width) * u64::from(source_height) * 4)
        .map_err(|_| "source RGBA size overflow".to_string())?;
    if input.len() != source_len {
        return Err("source RGBA byte count does not match its dimensions".into());
    }
    let target_len = usize::try_from(u64::from(target_width) * u64::from(target_height) * 4)
        .map_err(|_| "target RGBA size overflow".to_string())?;
    if target_len > MAX_DECODED_BYTES {
        return Err("resized RGBA output exceeds the decoded byte limit".into());
    }

    let mut output = vec![0u8; target_len];
    let source_width_usize = source_width as usize;
    for y in 0..target_height {
        let source_y = ((f64::from(y) + 0.5) * f64::from(source_height) / f64::from(target_height)
            - 0.5)
            .clamp(0.0, f64::from(source_height - 1));
        let y0 = source_y.floor() as usize;
        let y1 = (y0 + 1).min(source_height as usize - 1);
        let wy = source_y - y0 as f64;
        for x in 0..target_width {
            let source_x =
                ((f64::from(x) + 0.5) * f64::from(source_width) / f64::from(target_width) - 0.5)
                    .clamp(0.0, f64::from(source_width - 1));
            let x0 = source_x.floor() as usize;
            let x1 = (x0 + 1).min(source_width as usize - 1);
            let wx = source_x - x0 as f64;
            let samples = [
                ((y0 * source_width_usize + x0) * 4, (1.0 - wx) * (1.0 - wy)),
                ((y0 * source_width_usize + x1) * 4, wx * (1.0 - wy)),
                ((y1 * source_width_usize + x0) * 4, (1.0 - wx) * wy),
                ((y1 * source_width_usize + x1) * 4, wx * wy),
            ];

            let mut alpha = 0.0f64;
            let mut premultiplied = [0.0f64; 3];
            for (offset, weight) in samples {
                let sample_alpha = f64::from(input[offset + 3]) / 255.0;
                alpha += weight * sample_alpha;
                for channel in 0..3 {
                    premultiplied[channel] +=
                        weight * sample_alpha * (f64::from(input[offset + channel]) / 255.0);
                }
            }
            let destination = ((y as usize * target_width as usize) + x as usize) * 4;
            output[destination + 3] = (alpha * 255.0).round().clamp(0.0, 255.0) as u8;
            if alpha > f64::EPSILON {
                for channel in 0..3 {
                    output[destination + channel] = (premultiplied[channel] / alpha * 255.0)
                        .round()
                        .clamp(0.0, 255.0)
                        as u8;
                }
            }
        }
    }
    Ok(output)
}

fn output_parent(path: &Path) -> &Path {
    match path.parent() {
        Some(parent) if !parent.as_os_str().is_empty() => parent,
        _ => Path::new("."),
    }
}

struct TempFileGuard {
    path: PathBuf,
    armed: bool,
}

impl Drop for TempFileGuard {
    fn drop(&mut self) {
        if self.armed {
            let _ = fs::remove_file(&self.path);
        }
    }
}

fn create_private_temp(output: &Path) -> Result<(File, TempFileGuard), String> {
    let parent = output_parent(output);
    let parent_metadata = fs::metadata(parent)
        .map_err(|e| format!("cannot inspect output directory {}: {e}", parent.display()))?;
    if !parent_metadata.is_dir() {
        return Err(format!(
            "output parent is not a directory: {}",
            parent.display()
        ));
    }

    for _ in 0..32 {
        let nonce = TEMP_COUNTER.fetch_add(1, Ordering::Relaxed);
        let path = parent.join(format!(
            ".paper-agent-image-{}-{nonce}.tmp",
            std::process::id()
        ));
        let mut options = OpenOptions::new();
        options.write(true).create_new(true);
        #[cfg(unix)]
        {
            use std::os::unix::fs::OpenOptionsExt;
            options
                .mode(0o600)
                .custom_flags(libc::O_CLOEXEC | libc::O_NOFOLLOW);
        }
        match options.open(&path) {
            Ok(file) => return Ok((file, TempFileGuard { path, armed: true })),
            Err(error) if error.kind() == io::ErrorKind::AlreadyExists => continue,
            Err(error) => {
                return Err(format!("cannot create private output temp file: {error}"));
            }
        }
    }
    Err("cannot allocate a unique output temp file".into())
}

struct LimitedWriter<W> {
    inner: W,
    written: usize,
    limit: usize,
}

impl<W: Write> Write for LimitedWriter<W> {
    fn write(&mut self, buffer: &[u8]) -> io::Result<usize> {
        let remaining = self.limit.saturating_sub(self.written);
        if buffer.len() > remaining {
            return Err(io::Error::new(
                io::ErrorKind::Other,
                "encoded PNG exceeds output byte limit",
            ));
        }
        let count = self.inner.write(buffer)?;
        self.written = self
            .written
            .checked_add(count)
            .ok_or_else(|| io::Error::new(io::ErrorKind::Other, "output byte count overflow"))?;
        Ok(count)
    }

    fn flush(&mut self) -> io::Result<()> {
        self.inner.flush()
    }
}

fn encode_png_atomic(path: &Path, width: u32, height: u32, pixels: &[u8]) -> Result<(), String> {
    let expected = usize::try_from(u64::from(width) * u64::from(height) * 4)
        .map_err(|_| "encoded RGBA size overflow".to_string())?;
    if pixels.len() != expected {
        return Err("encoded RGBA byte count does not match its dimensions".into());
    }
    ensure_destination_absent(path)?;
    let (mut file, mut guard) = create_private_temp(path)?;
    {
        let mut limited = LimitedWriter {
            inner: &mut file,
            written: 0,
            limit: MAX_OUTPUT_BYTES,
        };
        {
            let mut encoder = Encoder::new(&mut limited, width, height);
            encoder.set_color(ColorType::Rgba);
            encoder.set_depth(BitDepth::Eight);
            encoder.set_compression(png::Compression::Fast);
            let mut writer = encoder
                .write_header()
                .map_err(|e| format!("cannot encode PNG header: {e}"))?;
            writer
                .write_image_data(pixels)
                .map_err(|e| format!("cannot encode PNG pixels: {e}"))?;
            writer
                .finish()
                .map_err(|e| format!("cannot finish PNG: {e}"))?;
        }
        limited
            .flush()
            .map_err(|e| format!("cannot flush encoded PNG: {e}"))?;
    }
    file.sync_all()
        .map_err(|e| format!("cannot sync encoded PNG: {e}"))?;
    drop(file);

    fs::hard_link(&guard.path, path).map_err(|e| {
        format!(
            "cannot publish PNG without replacing {}: {e}",
            path.display()
        )
    })?;
    fs::remove_file(&guard.path).map_err(|e| format!("cannot remove output temp file: {e}"))?;
    guard.armed = false;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicU64, Ordering};

    static TEST_COUNTER: AtomicU64 = AtomicU64::new(0);

    struct TestDir(PathBuf);

    impl TestDir {
        fn new() -> Self {
            let nonce = TEST_COUNTER.fetch_add(1, Ordering::Relaxed);
            let path = std::env::temp_dir().join(format!(
                "paper-agent-image-test-{}-{nonce}",
                std::process::id()
            ));
            fs::create_dir(&path).unwrap();
            Self(path)
        }

        fn path(&self, name: &str) -> PathBuf {
            self.0.join(name)
        }
    }

    impl Drop for TestDir {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }

    fn write_fixture(
        path: &Path,
        width: u32,
        height: u32,
        color: ColorType,
        depth: BitDepth,
        pixels: &[u8],
    ) {
        let file = File::create(path).unwrap();
        let mut encoder = Encoder::new(file, width, height);
        encoder.set_color(color);
        encoder.set_depth(depth);
        let mut writer = encoder.write_header().unwrap();
        writer.write_image_data(pixels).unwrap();
        writer.finish().unwrap();
    }

    fn decode_output(path: &Path) -> RgbaImage {
        decode_png(path).unwrap()
    }

    #[test]
    fn fitted_dimensions_never_enlarge_and_preserve_ratio() {
        assert_eq!(fitted_dimensions(400, 200, 800, 800).unwrap(), (400, 200));
        assert_eq!(fitted_dimensions(800, 400, 300, 300).unwrap(), (300, 150));
        assert_eq!(fitted_dimensions(400, 800, 300, 200).unwrap(), (100, 200));
        assert_eq!(fitted_dimensions(8, 4, 3, 3).unwrap(), (3, 2));
    }

    #[test]
    fn normalizes_common_8_and_16_bit_color_types() {
        let cases: &[(ColorType, BitDepth, &[u8], &[u8])] = &[
            (
                ColorType::Rgb,
                BitDepth::Eight,
                &[10, 20, 30],
                &[10, 20, 30, 255],
            ),
            (
                ColorType::Rgba,
                BitDepth::Eight,
                &[10, 20, 30, 40],
                &[10, 20, 30, 40],
            ),
            (
                ColorType::Grayscale,
                BitDepth::Eight,
                &[10],
                &[10, 10, 10, 255],
            ),
            (
                ColorType::GrayscaleAlpha,
                BitDepth::Eight,
                &[10, 40],
                &[10, 10, 10, 40],
            ),
            (
                ColorType::Rgb,
                BitDepth::Sixteen,
                &[10, 1, 20, 2, 30, 3],
                &[10, 20, 30, 255],
            ),
            (
                ColorType::Rgba,
                BitDepth::Sixteen,
                &[10, 1, 20, 2, 30, 3, 40, 4],
                &[10, 20, 30, 40],
            ),
            (
                ColorType::Grayscale,
                BitDepth::Sixteen,
                &[10, 1],
                &[10, 10, 10, 255],
            ),
            (
                ColorType::GrayscaleAlpha,
                BitDepth::Sixteen,
                &[10, 1, 40, 4],
                &[10, 10, 10, 40],
            ),
        ];
        for (index, (color, depth, source, expected)) in cases.iter().enumerate() {
            let dir = TestDir::new();
            let input = dir.path(&format!("input-{index}.png"));
            write_fixture(&input, 1, 1, *color, *depth, source);
            let image = decode_png(&input).unwrap();
            assert_eq!(image.width, 1);
            assert_eq!(image.height, 1);
            assert_eq!(image.pixels, *expected, "case {index}: {color:?} {depth:?}");
        }
    }

    #[test]
    fn prepare_image_downsizes_and_writes_rgba8() {
        let dir = TestDir::new();
        let input = dir.path("source.png");
        let output = dir.path("prepared.png");
        let pixels = vec![90u8; 8 * 4 * 3];
        write_fixture(&input, 8, 4, ColorType::Rgb, BitDepth::Eight, &pixels);

        let result = prepare_image(&input, &output, 3, 3).unwrap();
        assert_eq!(
            result,
            PreparedImage {
                width: 3,
                height: 2
            }
        );
        let decoded = decode_output(&output);
        assert_eq!((decoded.width, decoded.height), (3, 2));
        assert_eq!(decoded.pixels.len(), 3 * 2 * 4);
        assert!(decoded
            .pixels
            .chunks_exact(4)
            .all(|p| p == [90, 90, 90, 255]));
    }

    #[test]
    fn transparent_resize_uses_premultiplied_color() {
        let input = [255, 0, 0, 255, 0, 0, 255, 0];
        let output = resize_rgba8_premultiplied(&input, 2, 1, 1, 1).unwrap();
        assert_eq!(output[3], 128);
        assert!(output[0] >= 254);
        assert_eq!(output[1], 0);
        assert_eq!(output[2], 0);
    }

    #[test]
    fn refuses_corrupt_or_oversized_input_without_output() {
        let dir = TestDir::new();
        let corrupt = dir.path("corrupt.png");
        let output = dir.path("output.png");
        fs::write(&corrupt, b"not a PNG").unwrap();
        assert!(prepare_image(&corrupt, &output, 100, 100).is_err());
        assert!(!output.exists());

        let oversized = dir.path("oversized.png");
        let file = File::create(&oversized).unwrap();
        file.set_len(MAX_INPUT_BYTES + 1).unwrap();
        assert!(prepare_image(&oversized, &output, 100, 100).is_err());
        assert!(!output.exists());
    }

    #[test]
    fn refuses_to_replace_existing_destination() {
        let dir = TestDir::new();
        let input = dir.path("source.png");
        let output = dir.path("existing.png");
        write_fixture(&input, 1, 1, ColorType::Rgb, BitDepth::Eight, &[1, 2, 3]);
        fs::write(&output, b"keep me").unwrap();
        assert!(prepare_image(&input, &output, 10, 10).is_err());
        assert_eq!(fs::read(&output).unwrap(), b"keep me");
    }

    #[cfg(unix)]
    #[test]
    fn refuses_symlink_input_and_creates_private_output() {
        use std::os::unix::fs::{symlink, MetadataExt, PermissionsExt};

        let dir = TestDir::new();
        let input = dir.path("source.png");
        let link = dir.path("link.png");
        let output = dir.path("prepared.png");
        write_fixture(&input, 1, 1, ColorType::Rgb, BitDepth::Eight, &[1, 2, 3]);
        symlink(&input, &link).unwrap();
        assert!(prepare_image(&link, &output, 10, 10).is_err());
        assert!(!output.exists());

        prepare_image(&input, &output, 10, 10).unwrap();
        let metadata = fs::metadata(&output).unwrap();
        assert!(metadata.is_file());
        assert_eq!(metadata.permissions().mode() & 0o777, 0o600);
        assert_eq!(metadata.nlink(), 1);
    }

    #[test]
    fn validates_requested_and_source_bounds() {
        assert!(validate_requested_bounds(0, 1).is_err());
        assert!(validate_requested_bounds(1, 0).is_err());
        assert!(validate_requested_bounds(MAX_REQUESTED_DIMENSION + 1, 1).is_err());
        assert!(validate_source_dimensions(0, 1).is_err());
        assert!(validate_source_dimensions(MAX_SOURCE_DIMENSION + 1, 1).is_err());
        assert!(validate_source_dimensions(4_097, 4_097).is_err());
    }
}

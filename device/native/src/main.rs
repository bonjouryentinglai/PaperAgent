// SPDX-License-Identifier: MIT

use paper_agent_native::geometry::{canvas_point_to_axes, AxisRange};
use paper_agent_native::job::StrokeJob;
use paper_agent_native::renderer::{self, Target};
use std::env;
use std::fs;
use std::path::Path;

#[cfg(target_os = "linux")]
use std::io::{BufRead, BufReader, Write};

const USAGE: &str = "\
paper-agent-native 0.1.0

USAGE:
  paper-agent-native probe
  paper-agent-native dry-run JOB [X_MIN X_MAX Y_MIN Y_MAX]
  paper-agent-native render-text INPUT JOB X Y WIDTH HEIGHT [CANVAS_WIDTH CANVAS_HEIGHT]
  paper-agent-native render-text-scaled INPUT JOB X Y WIDTH HEIGHT SCALE_PERCENT [CANVAS_WIDTH CANVAS_HEIGHT]
  paper-agent-native render-beautify-text INPUT JOB X Y WIDTH HEIGHT [CANVAS_WIDTH CANVAS_HEIGHT]
  paper-agent-native render-document INPUT JOB X Y WIDTH HEIGHT [CANVAS_WIDTH CANVAS_HEIGHT]
  paper-agent-native render-document-scaled INPUT JOB X Y WIDTH HEIGHT SCALE_PERCENT [CANVAS_WIDTH CANVAS_HEIGHT]
  paper-agent-native render-table INPUT JOB X Y WIDTH HEIGHT [CANVAS_WIDTH CANVAS_HEIGHT]
  paper-agent-native render-table-scaled INPUT JOB X Y WIDTH HEIGHT SCALE_PERCENT [CANVAS_WIDTH CANVAS_HEIGHT]
  paper-agent-native render-vector INPUT JOB X Y WIDTH HEIGHT [CANVAS_WIDTH CANVAS_HEIGHT]
  paper-agent-native prepare-image INPUT OUTPUT MAX_WIDTH MAX_HEIGHT
  paper-agent-native write JOB --confirm PAPER_AGENT_NATIVE_WRITE_V1
  paper-agent-native write-stream --confirm PAPER_AGENT_NATIVE_WRITE_V1

Safety: write requires Chiappa detection, active Xochitl, a single-writer lock,
and the exact confirmation string shown above.
";

const STREAMING_JOB_DIR: &str = "/home/root/paper-agent/native/jobs";

fn main() {
    if let Err(e) = dispatch() {
        eprintln!("paper-agent-native: {e}");
        std::process::exit(1);
    }
}

fn dispatch() -> Result<(), String> {
    let args: Vec<String> = env::args().collect();
    match args.get(1).map(String::as_str) {
        Some("probe") => probe(),
        Some("dry-run") => dry_run(&args[2..]),
        Some("render-text") => render_text(&args[2..]),
        Some("render-text-scaled") => render_text_scaled(&args[2..]),
        Some("render-beautify-text") => render_beautify_text(&args[2..]),
        Some("render-document") => render_structured(&args[2..], "document"),
        Some("render-document-scaled") => render_structured_scaled(&args[2..], "document"),
        Some("render-table") => render_structured(&args[2..], "table"),
        Some("render-table-scaled") => render_structured_scaled(&args[2..], "table"),
        Some("render-vector") => render_structured(&args[2..], "vector"),
        Some("prepare-image") => prepare_image(&args[2..]),
        Some("write") => write(&args[2..]),
        Some("write-stream") => write_stream(&args[2..]),
        Some("--version" | "-V") => {
            println!("paper-agent-native {}", env!("CARGO_PKG_VERSION"));
            Ok(())
        }
        Some("--help" | "-h") | None => {
            print!("{USAGE}");
            Ok(())
        }
        Some(other) => Err(format!("unknown command '{other}'\n\n{USAGE}")),
    }
}

fn prepare_image(args: &[String]) -> Result<(), String> {
    if args.len() != 4 {
        return Err(format!("prepare-image requires 4 arguments\n\n{USAGE}"));
    }
    let prepared = paper_agent_native::image::prepare_image(
        Path::new(&args[0]),
        Path::new(&args[1]),
        parse_u32(&args[2], "MAX_WIDTH")?,
        parse_u32(&args[3], "MAX_HEIGHT")?,
    )?;
    println!("image_size={}x{}", prepared.width, prepared.height);
    Ok(())
}

fn render_text(args: &[String]) -> Result<(), String> {
    render_text_kind(args, false)
}

fn render_beautify_text(args: &[String]) -> Result<(), String> {
    render_text_kind(args, true)
}

fn render_text_scaled(args: &[String]) -> Result<(), String> {
    if args.len() != 7 && args.len() != 9 {
        return Err(format!(
            "render-text-scaled requires 7 or 9 arguments\n\n{USAGE}"
        ));
    }
    let input =
        fs::read_to_string(&args[0]).map_err(|e| format!("cannot read {}: {e}", args[0]))?;
    let target = parse_target(args)?;
    let scale_percent = parse_scale_percent(&args[6])?;
    let (canvas_width, canvas_height) = parse_canvas(args, 7)?;
    let job =
        renderer::text_to_job_scaled(&input, target, canvas_width, canvas_height, scale_percent)?;
    write_rendered_job(&args[1], &job, "render-text-scaled", Some(scale_percent))
}

fn render_text_kind(args: &[String], beautify: bool) -> Result<(), String> {
    if args.len() != 6 && args.len() != 8 {
        let command = if beautify {
            "render-beautify-text"
        } else {
            "render-text"
        };
        return Err(format!("{command} requires 6 or 8 arguments\n\n{USAGE}"));
    }
    let input =
        fs::read_to_string(&args[0]).map_err(|e| format!("cannot read {}: {e}", args[0]))?;
    let target = Target {
        x: parse_i32(&args[2], "X")?,
        y: parse_i32(&args[3], "Y")?,
        width: parse_u32(&args[4], "WIDTH")?,
        height: parse_u32(&args[5], "HEIGHT")?,
    };
    let (canvas_width, canvas_height) = if args.len() == 8 {
        (
            parse_u32(&args[6], "CANVAS_WIDTH")?,
            parse_u32(&args[7], "CANVAS_HEIGHT")?,
        )
    } else {
        (954, 1696)
    };
    let job = if beautify {
        renderer::beautify_text_to_job(&input, target, canvas_width, canvas_height)?
    } else {
        renderer::text_to_job(&input, target, canvas_width, canvas_height)?
    };
    job.write_to(Path::new(&args[1]))?;
    println!(
        "mode={}",
        if beautify {
            "render-beautify-text"
        } else {
            "render-text"
        }
    );
    println!("writes_performed=0");
    println!("output={}", args[1]);
    println!("canvas={}x{}", job.canvas_width, job.canvas_height);
    println!("strokes={}", job.strokes.len());
    println!("points={}", job.point_count());
    let (min_x, min_y, max_x, max_y) = job_bounds(&job);
    println!("pixel_bounds={min_x},{min_y}..{max_x},{max_y}");
    Ok(())
}

fn render_structured(args: &[String], kind: &str) -> Result<(), String> {
    if args.len() != 6 && args.len() != 8 {
        return Err(format!(
            "render-{kind} requires 6 or 8 arguments\n\n{USAGE}"
        ));
    }
    let input =
        fs::read_to_string(&args[0]).map_err(|e| format!("cannot read {}: {e}", args[0]))?;
    let target = Target {
        x: parse_i32(&args[2], "X")?,
        y: parse_i32(&args[3], "Y")?,
        width: parse_u32(&args[4], "WIDTH")?,
        height: parse_u32(&args[5], "HEIGHT")?,
    };
    let (canvas_width, canvas_height) = if args.len() == 8 {
        (
            parse_u32(&args[6], "CANVAS_WIDTH")?,
            parse_u32(&args[7], "CANVAS_HEIGHT")?,
        )
    } else {
        (954, 1696)
    };
    let job = match kind {
        "document" => renderer::document_to_job(&input, target, canvas_width, canvas_height)?,
        "table" => renderer::table_to_job(&input, target, canvas_width, canvas_height)?,
        "vector" => renderer::vector_to_job(&input, target, canvas_width, canvas_height)?,
        _ => return Err(format!("unsupported renderer '{kind}'")),
    };
    job.write_to(Path::new(&args[1]))?;
    println!("mode=render-{kind}");
    println!("writes_performed=0");
    println!("output={}", args[1]);
    println!("canvas={}x{}", job.canvas_width, job.canvas_height);
    println!("strokes={}", job.strokes.len());
    println!("points={}", job.point_count());
    let (min_x, min_y, max_x, max_y) = job_bounds(&job);
    println!("pixel_bounds={min_x},{min_y}..{max_x},{max_y}");
    Ok(())
}

fn render_structured_scaled(args: &[String], kind: &str) -> Result<(), String> {
    if args.len() != 7 && args.len() != 9 {
        return Err(format!(
            "render-{kind}-scaled requires 7 or 9 arguments\n\n{USAGE}"
        ));
    }
    let input =
        fs::read_to_string(&args[0]).map_err(|e| format!("cannot read {}: {e}", args[0]))?;
    let target = parse_target(args)?;
    let scale_percent = parse_scale_percent(&args[6])?;
    let (canvas_width, canvas_height) = parse_canvas(args, 7)?;
    let job = match kind {
        "document" => renderer::document_to_job_scaled(
            &input,
            target,
            canvas_width,
            canvas_height,
            scale_percent,
        )?,
        "table" => renderer::table_to_job_scaled(
            &input,
            target,
            canvas_width,
            canvas_height,
            scale_percent,
        )?,
        _ => return Err(format!("unsupported scaled renderer '{kind}'")),
    };
    write_rendered_job(
        &args[1],
        &job,
        &format!("render-{kind}-scaled"),
        Some(scale_percent),
    )
}

fn parse_target(args: &[String]) -> Result<Target, String> {
    Ok(Target {
        x: parse_i32(&args[2], "X")?,
        y: parse_i32(&args[3], "Y")?,
        width: parse_u32(&args[4], "WIDTH")?,
        height: parse_u32(&args[5], "HEIGHT")?,
    })
}

fn parse_canvas(args: &[String], optional_start: usize) -> Result<(u32, u32), String> {
    if args.len() == optional_start + 2 {
        Ok((
            parse_u32(&args[optional_start], "CANVAS_WIDTH")?,
            parse_u32(&args[optional_start + 1], "CANVAS_HEIGHT")?,
        ))
    } else {
        Ok((954, 1696))
    }
}

fn parse_scale_percent(value: &str) -> Result<u8, String> {
    let scale = value
        .parse::<u8>()
        .map_err(|_| format!("invalid SCALE_PERCENT '{value}'"))?;
    if !(60..=100).contains(&scale) {
        return Err("SCALE_PERCENT must be within 60..=100".into());
    }
    Ok(scale)
}

fn write_rendered_job(
    output: &str,
    job: &StrokeJob,
    mode: &str,
    scale_percent: Option<u8>,
) -> Result<(), String> {
    job.write_to(Path::new(output))?;
    println!("mode={mode}");
    if let Some(scale) = scale_percent {
        println!("scale_percent={scale}");
    }
    println!("writes_performed=0");
    println!("output={output}");
    println!("canvas={}x{}", job.canvas_width, job.canvas_height);
    println!("strokes={}", job.strokes.len());
    println!("points={}", job.point_count());
    let (min_x, min_y, max_x, max_y) = job_bounds(job);
    println!("pixel_bounds={min_x},{min_y}..{max_x},{max_y}");
    Ok(())
}

fn job_bounds(job: &StrokeJob) -> (i32, i32, i32, i32) {
    let mut min_x = i32::MAX;
    let mut min_y = i32::MAX;
    let mut max_x = i32::MIN;
    let mut max_y = i32::MIN;
    for point in job.strokes.iter().flatten() {
        min_x = min_x.min(point.x);
        min_y = min_y.min(point.y);
        max_x = max_x.max(point.x);
        max_y = max_y.max(point.y);
    }
    (min_x, min_y, max_x, max_y)
}

fn write(args: &[String]) -> Result<(), String> {
    if args.len() != 3 || args[1] != "--confirm" || args[2] != "PAPER_AGENT_NATIVE_WRITE_V1" {
        return Err(format!(
            "native writeback requires the exact confirmation string\n\n{USAGE}"
        ));
    }
    let job = StrokeJob::from_path(Path::new(&args[0]))?;
    #[cfg(target_os = "linux")]
    {
        paper_agent_native::writer::write_job(&job)
    }
    #[cfg(not(target_os = "linux"))]
    {
        let _ = job;
        Err("native writeback is available only on the Linux-based Move".into())
    }
}

#[cfg(target_os = "linux")]
fn write_stream(args: &[String]) -> Result<(), String> {
    if args.len() != 2 || args[0] != "--confirm" || args[1] != "PAPER_AGENT_NATIVE_WRITE_V1" {
        return Err(format!(
            "native streaming writeback requires the exact confirmation string\n\n{USAGE}"
        ));
    }
    let stdin = std::io::stdin();
    let mut stdout = std::io::stdout().lock();
    let mut session = paper_agent_native::writer::WriterSession::open()?;
    writeln!(stdout, "ready").map_err(|e| format!("stream stdout: {e}"))?;
    stdout.flush().map_err(|e| format!("stream stdout: {e}"))?;

    for line in BufReader::new(stdin.lock()).lines() {
        let line = line.map_err(|e| format!("stream stdin: {e}"))?;
        let path = line.trim();
        if path == "done" {
            writeln!(stdout, "done").map_err(|e| format!("stream stdout: {e}"))?;
            stdout.flush().map_err(|e| format!("stream stdout: {e}"))?;
            return Ok(());
        }
        if !is_allowed_streaming_job_path(path) {
            return Err(format!("refusing unexpected streaming job path: {path}"));
        }
        let job = StrokeJob::from_path(Path::new(path))?;
        session.write_job(&job)?;
        writeln!(stdout, "written={path}").map_err(|e| format!("stream stdout: {e}"))?;
        stdout.flush().map_err(|e| format!("stream stdout: {e}"))?;
    }
    Err("stream input closed before 'done'".into())
}

fn is_allowed_streaming_job_path(path: &str) -> bool {
    let path = Path::new(path);
    if path.parent() != Some(Path::new(STREAMING_JOB_DIR)) {
        return false;
    }
    let Some(file_name) = path.file_name().and_then(|name| name.to_str()) else {
        return false;
    };
    file_name.starts_with("stream-")
        && file_name.ends_with(".strokes")
        && file_name
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'.'))
}

#[cfg(not(target_os = "linux"))]
fn write_stream(_args: &[String]) -> Result<(), String> {
    Err("native streaming writeback is available only on the Linux-based Move".into())
}

#[cfg(target_os = "linux")]
fn probe() -> Result<(), String> {
    let p = paper_agent_native::device::probe()?;
    println!("mode=read-only");
    println!("machine={}", p.machine);
    println!("marker_path={}", p.marker_path);
    println!("marker_name={}", p.marker_name);
    println!("abs_x={}..{}", p.x_axis.minimum, p.x_axis.maximum);
    println!("abs_y={}..{}", p.y_axis.minimum, p.y_axis.maximum);
    println!("uinput_available={}", p.uinput_available);
    println!("write_commands=guarded");
    Ok(())
}

#[cfg(not(target_os = "linux"))]
fn probe() -> Result<(), String> {
    Err("probe is available only on the Linux-based Move".into())
}

fn dry_run(args: &[String]) -> Result<(), String> {
    let Some(job_path) = args.first() else {
        return Err(format!("dry-run requires JOB\n\n{USAGE}"));
    };
    let job = StrokeJob::from_path(Path::new(job_path))?;
    let (x_axis, y_axis, source) = match args.len() {
        1 => runtime_axes()?,
        5 => (
            AxisRange::new(parse_i32(&args[1], "X_MIN")?, parse_i32(&args[2], "X_MAX")?)?,
            AxisRange::new(parse_i32(&args[3], "Y_MIN")?, parse_i32(&args[4], "Y_MAX")?)?,
            "command-line",
        ),
        _ => return Err(format!("expected zero or four axis values\n\n{USAGE}")),
    };

    let canvas = (job.canvas_width, job.canvas_height);
    let mut min = (i32::MAX, i32::MAX);
    let mut max = (i32::MIN, i32::MIN);
    for stroke in &job.strokes {
        for point in stroke {
            let mapped = canvas_point_to_axes((point.x, point.y), canvas, x_axis, y_axis)?;
            min.0 = min.0.min(mapped.0);
            min.1 = min.1.min(mapped.1);
            max.0 = max.0.max(mapped.0);
            max.1 = max.1.max(mapped.1);
        }
    }

    println!("mode=dry-run");
    println!("writes_performed=0");
    println!("job={job_path}");
    println!("canvas={}x{}", job.canvas_width, job.canvas_height);
    println!("strokes={}", job.strokes.len());
    println!("points={}", job.point_count());
    println!("axes_source={source}");
    println!("abs_x={}..{}", x_axis.minimum, x_axis.maximum);
    println!("abs_y={}..{}", y_axis.minimum, y_axis.maximum);
    println!("mapped_bounds={},{}..{},{}", min.0, min.1, max.0, max.1);
    Ok(())
}

#[cfg(target_os = "linux")]
fn runtime_axes() -> Result<(AxisRange, AxisRange, &'static str), String> {
    let p = paper_agent_native::device::probe()?;
    Ok((p.x_axis, p.y_axis, "kernel-probe"))
}

#[cfg(not(target_os = "linux"))]
fn runtime_axes() -> Result<(AxisRange, AxisRange, &'static str), String> {
    Err("off-device dry-run requires X_MIN X_MAX Y_MIN Y_MAX".into())
}

fn parse_i32(value: &str, label: &str) -> Result<i32, String> {
    value
        .parse()
        .map_err(|_| format!("invalid {label} '{value}'"))
}

fn parse_u32(value: &str, label: &str) -> Result<u32, String> {
    value
        .parse()
        .map_err(|_| format!("invalid {label} '{value}'"))
}

#[cfg(test)]
mod tests {
    use super::{is_allowed_streaming_job_path, parse_scale_percent};

    #[test]
    fn streaming_job_path_accepts_only_stream_jobs() {
        assert!(is_allowed_streaming_job_path(
            "/home/root/paper-agent/native/jobs/stream-1784379886992-1.strokes"
        ));
        assert!(!is_allowed_streaming_job_path(
            "/home/root/paper-agent/native/jobs/replace-1784379886992-1-1.strokes"
        ));
    }

    #[test]
    fn streaming_job_path_rejects_escape_and_unknown_jobs() {
        assert!(!is_allowed_streaming_job_path(
            "/home/root/paper-agent/native/jobs/stream-../../escape.strokes"
        ));
        assert!(!is_allowed_streaming_job_path(
            "/home/root/paper-agent/native/jobs/other-1784379886992.strokes"
        ));
        assert!(!is_allowed_streaming_job_path(
            "/tmp/stream-1784379886992-1.strokes"
        ));
        assert!(!is_allowed_streaming_job_path(
            "/home/root/paper-agent/native/jobs/stream-1784379886992-1.json"
        ));
    }

    #[test]
    fn scaled_layout_accepts_only_sixty_through_one_hundred_percent() {
        assert_eq!(parse_scale_percent("60").unwrap(), 60);
        assert_eq!(parse_scale_percent("100").unwrap(), 100);
        assert!(parse_scale_percent("59").is_err());
        assert!(parse_scale_percent("101").is_err());
    }
}

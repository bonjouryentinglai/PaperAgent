// SPDX-License-Identifier: MIT
//
// Deterministic renderers for Paper Agent result kinds. Model output never
// reaches the Marker device directly: every format is parsed, bounded, turned
// into a StrokeJob, then validated again by the native writer.

use crate::handwriting;
use crate::job::{Point, StrokeJob};
use crate::traditional;
use std::f32::consts::PI;

const TEXT_MIN_PX: i32 = 24;
const TEXT_MAX_PX: i32 = 84;
const TEXT_MARGIN: i32 = 12;
const MAX_INPUT_CHARS: usize = 16_000;
const MAX_OUTPUT_STROKES: usize = 4096;
const BOLD_OFFSET: (i32, i32) = (1, 1);
const VECTOR_HEADER: &str = "paper-agent-vector 1";
const VECTOR_COORD_MAX: i32 = 1000;
const VECTOR_ANGLE_MAX: i32 = 360;
const MAX_VECTOR_COMMANDS: usize = 256;
const MAX_VECTOR_PATH_POINTS: usize = 64;
const MAX_VECTOR_LABEL_CHARS: usize = 240;
const MAX_VECTOR_OUTPUT_STROKES: usize = 512;
const MAX_VECTOR_OUTPUT_POINTS: usize = 24_000;
const MAX_VECTOR_PATH_STEPS: usize = 400_000;
const QUADRATIC_SAMPLES: usize = 32;
const CUBIC_SAMPLES: usize = 40;
const MAX_ARC_SAMPLES: usize = 64;
const HATCH_SPACING: i32 = 45;
const MAX_HATCH_STROKES_PER_SHAPE: usize = 32;
const DOT_RADIUS: i32 = 9;
const DOCUMENT_HEADER: &str = "paper-agent-document 1";
const MAX_DOCUMENT_BLOCKS: usize = 48;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Target {
    pub x: i32,
    pub y: i32,
    pub width: u32,
    pub height: u32,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum DocumentBlockKind {
    Rich,
    Table,
    Vector,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct DocumentBlock {
    kind: DocumentBlockKind,
    body: String,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum RichLineKind {
    Paragraph,
    Heading(u8),
    List,
    Code,
    Blank,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct RichSpan {
    text: String,
    bold: bool,
    code: bool,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct RichLine {
    kind: RichLineKind,
    indent: u8,
    spans: Vec<RichSpan>,
}

#[derive(Clone, Debug)]
struct VisualLine {
    kind: RichLineKind,
    indent_px: i32,
    px: i32,
    height: i32,
    spans: Vec<RichSpan>,
}

#[derive(Clone, Debug)]
struct RichLayout {
    lines: Vec<VisualLine>,
    height: i32,
}

pub fn text_to_job(
    text: &str,
    target: Target,
    canvas_width: u32,
    canvas_height: u32,
) -> Result<StrokeJob, String> {
    validate_target(target, canvas_width, canvas_height)?;
    let display = normalized_input(text)?;
    let strokes = render_text_box(
        &display,
        target,
        TEXT_MIN_PX,
        TEXT_MAX_PX,
        TEXT_MARGIN,
        true,
    )?;
    build_job(strokes, canvas_width, canvas_height)
}

pub fn table_to_job(
    text: &str,
    target: Target,
    canvas_width: u32,
    canvas_height: u32,
) -> Result<StrokeJob, String> {
    validate_target(target, canvas_width, canvas_height)?;
    let rows = parse_table(&normalized_input(text)?)?;
    let columns = rows.iter().map(Vec::len).max().unwrap_or(0);
    if columns == 0 || columns > 8 || rows.len() > 14 {
        return Err("table must contain 1..=8 columns and 1..=14 rows".into());
    }
    if target.width < columns as u32 * 72 || target.height < rows.len() as u32 * 52 {
        return Err("table does not fit the requested placement box".into());
    }

    let right = target.x + target.width as i32 - 1;
    let bottom = target.y + target.height as i32 - 1;
    let mut strokes = Vec::new();
    for column in 0..=columns {
        let x = target.x + (target.width as i32 - 1) * column as i32 / columns as i32;
        strokes.push(vec![Point { x, y: target.y }, Point { x, y: bottom }]);
    }
    for row in 0..=rows.len() {
        let y = target.y + (target.height as i32 - 1) * row as i32 / rows.len() as i32;
        strokes.push(vec![Point { x: target.x, y }, Point { x: right, y }]);
    }

    for (row_index, row) in rows.iter().enumerate() {
        let y0 = target.y + target.height as i32 * row_index as i32 / rows.len() as i32;
        let y1 = target.y + target.height as i32 * (row_index + 1) as i32 / rows.len() as i32;
        for column_index in 0..columns {
            let Some(cell) = row.get(column_index).filter(|cell| !cell.is_empty()) else {
                continue;
            };
            let x0 = target.x + target.width as i32 * column_index as i32 / columns as i32;
            let x1 = target.x + target.width as i32 * (column_index + 1) as i32 / columns as i32;
            let inset = 4;
            let cell_target = Target {
                x: x0 + inset,
                y: y0 + inset,
                width: (x1 - x0 - inset * 2).max(48) as u32,
                height: (y1 - y0 - inset * 2).max(48) as u32,
            };
            let mut cell_strokes = render_text_box(cell, cell_target, 14, 42, 3, row_index == 0)?;
            strokes.append(&mut cell_strokes);
        }
    }
    build_job(strokes, canvas_width, canvas_height)
}

pub fn vector_to_job(
    text: &str,
    target: Target,
    canvas_width: u32,
    canvas_height: u32,
) -> Result<StrokeJob, String> {
    validate_target(target, canvas_width, canvas_height)?;
    if text.chars().count() > MAX_INPUT_CHARS {
        return Err("vector input is too large".into());
    }
    let mut lines = text.lines().map(str::trim).filter(|line| !line.is_empty());
    if lines.next() != Some(VECTOR_HEADER) {
        return Err(format!("vector data must start with '{VECTOR_HEADER}'"));
    }
    let mut strokes = Vec::new();
    for (index, line) in lines.enumerate() {
        if index >= MAX_VECTOR_COMMANDS {
            return Err(format!(
                "vector data exceeds {MAX_VECTOR_COMMANDS} commands"
            ));
        }
        let fields: Vec<&str> = line.split_ascii_whitespace().collect();
        match fields.first().copied() {
            Some("line") if fields.len() == 5 => {
                let values = parse_vector_numbers(&fields[1..])?;
                strokes.push(map_polyline(
                    &[(values[0], values[1]), (values[2], values[3])],
                    target,
                ));
            }
            Some("polyline") if fields.len() >= 5 && fields.len() % 2 == 1 => {
                let values = parse_vector_numbers(&fields[1..])?;
                let points = vector_pairs(&values, 2, "polyline")?;
                strokes.push(map_polyline(&points, target));
            }
            Some("polygon") if fields.len() >= 7 && fields.len() % 2 == 1 => {
                let values = parse_vector_numbers(&fields[1..])?;
                let mut points = vector_pairs(&values, 3, "polygon")?;
                close_path(&mut points);
                strokes.push(map_polyline(&points, target));
            }
            Some("fillpoly") if fields.len() >= 7 && fields.len() % 2 == 1 => {
                let values = parse_vector_numbers(&fields[1..])?;
                let points = vector_pairs(&values, 3, "fillpoly")?;
                append_hatched_polygon(&mut strokes, &points, target);
            }
            Some("curve") if fields.len() == 7 || fields.len() == 9 => {
                let values = parse_vector_numbers(&fields[1..])?;
                let points = if values.len() == 6 {
                    quadratic_points(&values, QUADRATIC_SAMPLES)
                } else {
                    cubic_points(&values, CUBIC_SAMPLES)
                };
                strokes.push(map_polyline(&points, target));
            }
            Some("rect") if fields.len() == 5 => {
                let values = parse_vector_numbers(&fields[1..])?;
                strokes.push(map_polyline(&rect_points(&values, "rect")?, target));
            }
            Some("box") if fields.len() == 5 => {
                let values = parse_vector_numbers(&fields[1..])?;
                let points = rect_points(&values, "box")?;
                append_hatched_polygon(&mut strokes, &points[..4], target);
            }
            Some("rrect") if fields.len() == 6 => {
                let values = parse_vector_numbers(&fields[1..])?;
                strokes.push(map_polyline(&rounded_rect_points(&values)?, target));
            }
            Some("circle") if fields.len() == 4 => {
                let values = parse_vector_numbers(&fields[1..])?;
                validate_ellipse_geometry(values[0], values[1], values[2], values[2], "circle")?;
                strokes.push(map_polyline(
                    &ellipse_points(values[0], values[1], values[2], values[2]),
                    target,
                ));
            }
            Some("disc") if fields.len() == 4 => {
                let values = parse_vector_numbers(&fields[1..])?;
                validate_ellipse_geometry(values[0], values[1], values[2], values[2], "disc")?;
                append_hatched_ellipse(
                    &mut strokes,
                    values[0],
                    values[1],
                    values[2],
                    values[2],
                    target,
                );
            }
            Some("ellipse") if fields.len() == 5 => {
                let values = parse_vector_numbers(&fields[1..])?;
                validate_ellipse_geometry(values[0], values[1], values[2], values[3], "ellipse")?;
                strokes.push(map_polyline(
                    &ellipse_points(values[0], values[1], values[2], values[3]),
                    target,
                ));
            }
            Some("fillellipse") if fields.len() == 5 => {
                let values = parse_vector_numbers(&fields[1..])?;
                validate_ellipse_geometry(
                    values[0],
                    values[1],
                    values[2],
                    values[3],
                    "fillellipse",
                )?;
                append_hatched_ellipse(
                    &mut strokes,
                    values[0],
                    values[1],
                    values[2],
                    values[3],
                    target,
                );
            }
            Some("arc") if fields.len() == 6 => {
                let values = parse_arc_values(&fields[1..], "arc")?;
                validate_ellipse_geometry(values[0], values[1], values[2], values[2], "arc")?;
                validate_nonzero_sweep(values[3], values[4], "arc")?;
                strokes.push(map_polyline(
                    &arc_points(values[0], values[1], values[2], values[3], values[4]),
                    target,
                ));
            }
            Some("wedge") if fields.len() == 6 => {
                let values = parse_arc_values(&fields[1..], "wedge")?;
                validate_ellipse_geometry(values[0], values[1], values[2], values[2], "wedge")?;
                validate_nonzero_sweep(values[3], values[4], "wedge")?;
                let mut points = vec![(values[0], values[1])];
                points.extend(arc_points(
                    values[0], values[1], values[2], values[3], values[4],
                ));
                append_hatched_polygon(&mut strokes, &points, target);
            }
            Some("dot") if fields.len() == 3 => {
                let values = parse_vector_numbers(&fields[1..])?;
                append_dot(&mut strokes, values[0], values[1], target);
            }
            Some("arrow") if fields.len() == 5 => {
                let values = parse_vector_numbers(&fields[1..])?;
                let (x0, y0, x1, y1) = (values[0], values[1], values[2], values[3]);
                strokes.push(map_polyline(&[(x0, y0), (x1, y1)], target));
                let angle = ((y1 - y0) as f32).atan2((x1 - x0) as f32);
                let length = 55.0;
                for offset in [PI * 0.82, -PI * 0.82] {
                    let head_x = (x1 as f32 + (angle + offset).cos() * length).round() as i32;
                    let head_y = (y1 as f32 + (angle + offset).sin() * length).round() as i32;
                    strokes.push(map_polyline(&[(x1, y1), (head_x, head_y)], target));
                }
            }
            Some("label") if fields.len() >= 6 => {
                let values = parse_vector_numbers(&fields[1..5])?;
                if values[2] == 0 || values[3] == 0 {
                    return Err("vector label width and height must be positive".into());
                }
                validate_rect_geometry(values[0], values[1], values[2], values[3], "label")?;
                let raw_text = fields[5..].join(" ");
                if raw_text.chars().count() > MAX_VECTOR_LABEL_CHARS {
                    return Err(format!(
                        "vector label exceeds {MAX_VECTOR_LABEL_CHARS} characters"
                    ));
                }
                let text = traditional::for_display(&raw_text);
                let label_target =
                    map_vector_target(values[0], values[1], values[2], values[3], target)?;
                let mut label_strokes = render_text_box(&text, label_target, 10, 36, 2, true)?;
                strokes.append(&mut label_strokes);
            }
            Some(command) => {
                return Err(format!(
                    "unsupported or malformed vector command '{command}'"
                ))
            }
            None => {}
        }
    }
    if strokes.is_empty() {
        return Err("vector data contains no drawable commands".into());
    }
    validate_vector_complexity(&strokes)?;
    build_job(strokes, canvas_width, canvas_height)
}

pub fn document_to_job(
    text: &str,
    target: Target,
    canvas_width: u32,
    canvas_height: u32,
) -> Result<StrokeJob, String> {
    validate_target(target, canvas_width, canvas_height)?;
    let blocks = parse_document_manifest(text)?;
    let mut chosen = None;
    for base_px in (22..=42).rev() {
        let heights = document_block_heights(&blocks, target, base_px)?;
        let gap = (base_px / 2).max(10);
        let total: i32 = heights.iter().sum::<i32>() + gap * (heights.len() as i32 - 1).max(0);
        if total <= target.height as i32 {
            chosen = Some((base_px, heights, gap));
            break;
        }
    }
    let Some((base_px, heights, gap)) = chosen else {
        return Err("rich document does not fit the requested placement box".into());
    };

    let mut strokes = Vec::new();
    let mut y = target.y;
    for (index, block) in blocks.iter().enumerate() {
        let block_target = Target {
            x: target.x,
            y,
            width: target.width,
            height: heights[index] as u32,
        };
        match block.kind {
            DocumentBlockKind::Rich => {
                let mut block_strokes = render_rich_markdown(&block.body, block_target, base_px)?;
                strokes.append(&mut block_strokes);
            }
            DocumentBlockKind::Table => {
                let block_job =
                    table_to_job(&block.body, block_target, canvas_width, canvas_height)?;
                strokes.extend(block_job.strokes);
            }
            DocumentBlockKind::Vector => {
                let block_job =
                    vector_to_job(&block.body, block_target, canvas_width, canvas_height)?;
                strokes.extend(block_job.strokes);
            }
        }
        y += heights[index] + gap;
    }
    build_job(strokes, canvas_width, canvas_height)
}

fn parse_document_manifest(text: &str) -> Result<Vec<DocumentBlock>, String> {
    let mut lines = text.lines().map(str::trim).filter(|line| !line.is_empty());
    if lines.next() != Some(DOCUMENT_HEADER) {
        return Err(format!("document data must start with '{DOCUMENT_HEADER}'"));
    }
    let mut blocks = Vec::new();
    let mut total_chars = 0usize;
    for line in lines {
        if blocks.len() >= MAX_DOCUMENT_BLOCKS {
            return Err(format!(
                "document data exceeds {MAX_DOCUMENT_BLOCKS} blocks"
            ));
        }
        let mut fields = line.splitn(3, ' ');
        if fields.next() != Some("block") {
            return Err("document record must start with 'block'".into());
        }
        let kind = match fields.next() {
            Some("rich") => DocumentBlockKind::Rich,
            Some("table") => DocumentBlockKind::Table,
            Some("vector") => DocumentBlockKind::Vector,
            Some(other) => return Err(format!("unsupported document block '{other}'")),
            None => return Err("document record is missing its block kind".into()),
        };
        let encoded = fields
            .next()
            .ok_or_else(|| "document record is missing its encoded body".to_string())?;
        let body = decode_hex_utf8(encoded)?;
        if body.trim().is_empty() {
            return Err("document contains an empty block".into());
        }
        total_chars += body.chars().count();
        if total_chars > MAX_INPUT_CHARS {
            return Err(format!(
                "document content exceeds {MAX_INPUT_CHARS} characters"
            ));
        }
        blocks.push(DocumentBlock { kind, body });
    }
    if blocks.is_empty() {
        return Err("document contains no blocks".into());
    }
    Ok(blocks)
}

fn decode_hex_utf8(encoded: &str) -> Result<String, String> {
    if encoded.is_empty() || encoded.len() % 2 != 0 || !encoded.is_ascii() {
        return Err("document block has invalid hex data".into());
    }
    let bytes = encoded.as_bytes();
    let mut decoded = Vec::with_capacity(bytes.len() / 2);
    for pair in bytes.chunks_exact(2) {
        let high =
            hex_nibble(pair[0]).ok_or_else(|| "document block has invalid hex data".to_string())?;
        let low =
            hex_nibble(pair[1]).ok_or_else(|| "document block has invalid hex data".to_string())?;
        decoded.push((high << 4) | low);
    }
    String::from_utf8(decoded).map_err(|_| "document block is not valid UTF-8".into())
}

fn hex_nibble(value: u8) -> Option<u8> {
    match value {
        b'0'..=b'9' => Some(value - b'0'),
        b'a'..=b'f' => Some(value - b'a' + 10),
        b'A'..=b'F' => Some(value - b'A' + 10),
        _ => None,
    }
}

fn document_block_heights(
    blocks: &[DocumentBlock],
    target: Target,
    base_px: i32,
) -> Result<Vec<i32>, String> {
    let mut heights = Vec::with_capacity(blocks.len());
    for block in blocks {
        let height = match block.kind {
            DocumentBlockKind::Rich => {
                layout_rich_markdown(&block.body, target.width as i32, base_px)?.height
            }
            DocumentBlockKind::Table => {
                let display = normalized_input(&block.body)?;
                let rows = parse_table(&display)?;
                let columns = rows.iter().map(Vec::len).max().unwrap_or(0);
                if columns == 0 || columns > 8 || rows.len() > 14 {
                    return Err("table must contain 1..=8 columns and 1..=14 rows".into());
                }
                if target.width < columns as u32 * 72 {
                    return Err("table is too wide for the document placement box".into());
                }
                rows.len() as i32 * (base_px * 3 / 2).max(52)
            }
            DocumentBlockKind::Vector => {
                let scaled = target.width as i32 * 3 / 5 * base_px / 38;
                scaled.clamp(160, 360)
            }
        };
        heights.push(height.max(48));
    }
    Ok(heights)
}

fn render_rich_markdown(
    markdown: &str,
    target: Target,
    base_px: i32,
) -> Result<Vec<Vec<Point>>, String> {
    let layout = layout_rich_markdown(markdown, target.width as i32, base_px)?;
    if layout.height > target.height as i32 {
        return Err("rich text block exceeds its document placement".into());
    }
    let margin = 10;
    let right = target.x + target.width as i32 - margin - 1;
    let mut y = target.y + margin;
    let mut strokes = Vec::new();
    for line in layout.lines {
        if line.kind == RichLineKind::Blank {
            y += line.height;
            continue;
        }
        let mut x = target.x + margin + line.indent_px;
        if line.kind == RichLineKind::Code {
            let left = (x - 5).max(target.x + 2);
            let bottom = (y + line.height - 2).min(target.y + target.height as i32 - 2);
            strokes.push(vec![
                Point { x: left, y },
                Point { x: right, y },
                Point {
                    x: right,
                    y: bottom,
                },
                Point { x: left, y: bottom },
                Point { x: left, y },
            ]);
            x += 6;
        }
        for span in line.spans {
            if span.text.is_empty() {
                continue;
            }
            let run_width = (handwriting::measure(&span.text, line.px as f32).ceil() as i32 + 7)
                .max(8)
                .min((right - x).max(8));
            if span.code && line.kind != RichLineKind::Code {
                let box_bottom = (y + line.height - 3).min(target.y + target.height as i32 - 2);
                let box_right = (x + run_width).min(right);
                strokes.push(vec![
                    Point { x, y },
                    Point { x: box_right, y },
                    Point {
                        x: box_right,
                        y: box_bottom,
                    },
                    Point { x, y: box_bottom },
                    Point { x, y },
                ]);
            }
            let raster = handwriting::rasterize_line(&span.text, line.px as f32);
            let raster_height = raster.height as i32;
            let line_y = y + ((line.height - raster_height) / 2).max(0);
            let weight = if span.bold || matches!(line.kind, RichLineKind::Heading(_)) {
                3
            } else {
                2
            };
            for stroke in handwriting::trace_line(raster) {
                let mapped: Vec<Point> = stroke
                    .into_iter()
                    .map(|(run_x, run_y)| Point {
                        x: x + run_x,
                        y: line_y + run_y,
                    })
                    .filter(|point| contains(target, *point))
                    .collect();
                add_stroke_weight(&mut strokes, mapped, target, weight);
            }
            x += run_width;
        }
        if matches!(line.kind, RichLineKind::Heading(1..=3)) {
            let underline_y = (y + line.height - 2).min(target.y + target.height as i32 - 2);
            strokes.push(vec![
                Point {
                    x: target.x + margin,
                    y: underline_y,
                },
                Point {
                    x: right,
                    y: underline_y,
                },
            ]);
        }
        y += line.height;
    }
    if strokes.is_empty() {
        return Err("rich Markdown produced no drawable strokes".into());
    }
    Ok(strokes)
}

fn layout_rich_markdown(
    markdown: &str,
    target_width: i32,
    base_px: i32,
) -> Result<RichLayout, String> {
    let display = normalized_input(markdown)?;
    let logical = parse_rich_lines(&display);
    let margin = 10;
    let available_width = target_width - margin * 2;
    if available_width < 32 {
        return Err("rich document has no usable text width".into());
    }
    let mut lines = Vec::new();
    for line in logical {
        if line.kind == RichLineKind::Blank {
            lines.push(VisualLine {
                kind: RichLineKind::Blank,
                indent_px: 0,
                px: base_px,
                height: (base_px / 2).max(8),
                spans: Vec::new(),
            });
            continue;
        }
        let px = rich_line_px(line.kind, base_px);
        let height = (px * 5 / 4
            + if matches!(line.kind, RichLineKind::Heading(_)) {
                4
            } else {
                0
            })
        .max(20);
        let indent_px = line.indent as i32 * (base_px * 3 / 4).max(12);
        let code_inset = if line.kind == RichLineKind::Code {
            12
        } else {
            0
        };
        let line_width = (available_width - indent_px - code_inset).max(24);
        let wrapped = wrap_rich_spans(&line.spans, px, line_width);
        if wrapped.is_empty() {
            lines.push(VisualLine {
                kind: line.kind,
                indent_px,
                px,
                height,
                spans: Vec::new(),
            });
        } else {
            for spans in wrapped {
                lines.push(VisualLine {
                    kind: line.kind,
                    indent_px,
                    px,
                    height,
                    spans,
                });
            }
        }
    }
    let height = margin * 2 + lines.iter().map(|line| line.height).sum::<i32>();
    Ok(RichLayout { lines, height })
}

fn rich_line_px(kind: RichLineKind, base_px: i32) -> i32 {
    match kind {
        RichLineKind::Heading(1) => base_px * 3 / 2,
        RichLineKind::Heading(2) => base_px * 4 / 3,
        RichLineKind::Heading(3) => base_px * 6 / 5,
        RichLineKind::Heading(_) => base_px * 11 / 10,
        RichLineKind::Code => (base_px * 4 / 5).max(18),
        _ => base_px,
    }
    .clamp(18, 72)
}

fn parse_rich_lines(markdown: &str) -> Vec<RichLine> {
    let mut result = Vec::new();
    let mut code_marker: Option<(char, usize)> = None;
    for raw in markdown.lines() {
        let trimmed = raw.trim();
        if let Some((marker, length)) = code_marker {
            let closing =
                trimmed.chars().all(|ch| ch == marker) && trimmed.chars().count() >= length;
            if closing {
                code_marker = None;
            } else {
                result.push(RichLine {
                    kind: RichLineKind::Code,
                    indent: 0,
                    spans: vec![RichSpan {
                        text: raw.to_string(),
                        bold: false,
                        code: true,
                    }],
                });
            }
            continue;
        }
        if let Some((marker, length)) = markdown_fence(trimmed) {
            code_marker = Some((marker, length));
            continue;
        }
        if trimmed.is_empty() {
            result.push(RichLine {
                kind: RichLineKind::Blank,
                indent: 0,
                spans: Vec::new(),
            });
            continue;
        }

        let leading = raw.chars().take_while(|ch| *ch == ' ').count();
        let content = raw.trim_start_matches(' ');
        let hashes = content.chars().take_while(|ch| *ch == '#').count();
        if (1..=6).contains(&hashes) && content.as_bytes().get(hashes) == Some(&b' ') {
            result.push(RichLine {
                kind: RichLineKind::Heading(hashes as u8),
                indent: 0,
                spans: parse_inline_spans(content[hashes + 1..].trim()),
            });
            continue;
        }

        if let Some((prefix, body)) = list_item(content) {
            let mut spans = vec![RichSpan {
                text: prefix,
                bold: true,
                code: false,
            }];
            append_spans(&mut spans, parse_inline_spans(body));
            result.push(RichLine {
                kind: RichLineKind::List,
                indent: (leading / 2).min(4) as u8,
                spans,
            });
            continue;
        }

        result.push(RichLine {
            kind: RichLineKind::Paragraph,
            indent: 0,
            spans: parse_inline_spans(content),
        });
    }
    result
}

fn markdown_fence(line: &str) -> Option<(char, usize)> {
    let marker = line.chars().next()?;
    if marker != '`' && marker != '~' {
        return None;
    }
    let length = line.chars().take_while(|ch| *ch == marker).count();
    (length >= 3).then_some((marker, length))
}

fn list_item(line: &str) -> Option<(String, &str)> {
    for marker in ["- ", "* ", "+ "] {
        if let Some(body) = line.strip_prefix(marker) {
            return Some(("• ".into(), body));
        }
    }
    let digits = line.bytes().take_while(u8::is_ascii_digit).count();
    if digits == 0 {
        return None;
    }
    let delimiter = line.as_bytes().get(digits).copied()?;
    if !matches!(delimiter, b'.' | b')') || line.as_bytes().get(digits + 1) != Some(&b' ') {
        return None;
    }
    Some((
        format!("{}{} ", &line[..digits], delimiter as char),
        &line[digits + 2..],
    ))
}

fn parse_inline_spans(text: &str) -> Vec<RichSpan> {
    let chars: Vec<char> = text.chars().collect();
    let mut result = Vec::new();
    let mut current = String::new();
    let mut bold = false;
    let mut code = false;
    let mut index = 0;
    while index < chars.len() {
        if chars[index] == '\\' && index + 1 < chars.len() {
            current.push(chars[index + 1]);
            index += 2;
            continue;
        }
        if chars[index] == '`' {
            push_rich_span(&mut result, std::mem::take(&mut current), bold, code);
            code = !code;
            index += 1;
            continue;
        }
        if !code
            && index + 1 < chars.len()
            && ((chars[index] == '*' && chars[index + 1] == '*')
                || (chars[index] == '_' && chars[index + 1] == '_'))
        {
            push_rich_span(&mut result, std::mem::take(&mut current), bold, code);
            bold = !bold;
            index += 2;
            continue;
        }
        current.push(chars[index]);
        index += 1;
    }
    push_rich_span(&mut result, current, bold, code);
    result
}

fn push_rich_span(result: &mut Vec<RichSpan>, text: String, bold: bool, code: bool) {
    if text.is_empty() {
        return;
    }
    if let Some(last) = result
        .last_mut()
        .filter(|last| last.bold == bold && last.code == code)
    {
        last.text.push_str(&text);
    } else {
        result.push(RichSpan { text, bold, code });
    }
}

fn append_spans(target: &mut Vec<RichSpan>, spans: Vec<RichSpan>) {
    for span in spans {
        push_rich_span(target, span.text, span.bold, span.code);
    }
}

fn wrap_rich_spans(spans: &[RichSpan], px: i32, max_width: i32) -> Vec<Vec<RichSpan>> {
    let mut result = Vec::new();
    let mut line = Vec::new();
    let mut width = 0.0f32;
    for token in rich_tokens(spans) {
        let token_width = handwriting::measure(&token.text, px as f32) + 5.0;
        if !line.is_empty() && width + token_width > max_width as f32 {
            trim_rich_line(&mut line);
            if !line.is_empty() {
                result.push(std::mem::take(&mut line));
            }
            width = 0.0;
        }
        if token_width <= max_width as f32 {
            if line.is_empty() && token.text.chars().all(char::is_whitespace) {
                continue;
            }
            width += token_width;
            push_rich_span(&mut line, token.text, token.bold, token.code);
            continue;
        }
        for ch in token.text.chars() {
            let part = ch.to_string();
            let part_width = handwriting::measure(&part, px as f32) + 2.0;
            if !line.is_empty() && width + part_width > max_width as f32 {
                trim_rich_line(&mut line);
                if !line.is_empty() {
                    result.push(std::mem::take(&mut line));
                }
                width = 0.0;
            }
            width += part_width;
            push_rich_span(&mut line, part, token.bold, token.code);
        }
    }
    trim_rich_line(&mut line);
    if !line.is_empty() {
        result.push(line);
    }
    result
}

fn rich_tokens(spans: &[RichSpan]) -> Vec<RichSpan> {
    let mut result = Vec::new();
    for span in spans {
        let mut token = String::new();
        let mut token_class = None;
        for ch in span.text.chars() {
            let class = if ch.is_whitespace() {
                0
            } else if is_cjk_for_wrap(ch) {
                1
            } else {
                2
            };
            if token_class != Some(class) || class == 1 {
                push_rich_token(
                    &mut result,
                    std::mem::take(&mut token),
                    span.bold,
                    span.code,
                );
                token_class = Some(class);
            }
            if class == 0 {
                if token.is_empty() {
                    token.push(' ');
                }
            } else {
                token.push(ch);
            }
            if class == 1 {
                push_rich_token(
                    &mut result,
                    std::mem::take(&mut token),
                    span.bold,
                    span.code,
                );
                token_class = None;
            }
        }
        push_rich_token(&mut result, token, span.bold, span.code);
    }
    result
}

fn push_rich_token(result: &mut Vec<RichSpan>, text: String, bold: bool, code: bool) {
    if !text.is_empty() {
        result.push(RichSpan { text, bold, code });
    }
}

fn is_cjk_for_wrap(ch: char) -> bool {
    matches!(ch as u32,
        0x3400..=0x4dbf | 0x4e00..=0x9fff | 0x3000..=0x303f | 0xff00..=0xffef)
}

fn trim_rich_line(spans: &mut Vec<RichSpan>) {
    while let Some(last) = spans.last_mut() {
        let trimmed = last.text.trim_end().to_string();
        if trimmed.is_empty() {
            spans.pop();
        } else {
            last.text = trimmed;
            break;
        }
    }
}

fn add_stroke_weight(
    strokes: &mut Vec<Vec<Point>>,
    mapped: Vec<Point>,
    target: Target,
    weight: usize,
) {
    if mapped.len() < 2 {
        return;
    }
    strokes.push(mapped.clone());
    for offset in 1..weight {
        let shifted: Vec<Point> = mapped
            .iter()
            .map(|point| Point {
                x: point.x + offset as i32,
                y: point.y + ((offset + 1) / 2) as i32,
            })
            .filter(|point| contains(target, *point))
            .collect();
        if shifted.len() >= 2 {
            strokes.push(shifted);
        }
    }
}

fn normalized_input(text: &str) -> Result<String, String> {
    let text = text.trim();
    if text.is_empty() {
        return Err("cannot render empty content".into());
    }
    if text.chars().count() > MAX_INPUT_CHARS {
        return Err(format!("content exceeds {MAX_INPUT_CHARS} characters"));
    }
    Ok(traditional::for_display(text))
}

fn render_text_box(
    text: &str,
    target: Target,
    min_px: i32,
    max_px: i32,
    margin: i32,
    add_weight: bool,
) -> Result<Vec<Vec<Point>>, String> {
    let available_width = target.width as i32 - margin * 2;
    let available_height = target.height as i32 - margin * 2;
    if available_width <= 0 || available_height <= 0 {
        return Err("text placement has no usable area".into());
    }
    let (px, lines) = choose_layout(text, min_px, max_px, available_width, available_height)?;
    let line_height = (px * 5 / 4).max(1);
    let mut y = target.y + margin;
    let mut strokes = Vec::new();

    for line_text in lines {
        let raster = handwriting::rasterize_line(&line_text, px as f32);
        let raster_height = raster.height as i32;
        let line_y = y + ((line_height - raster_height) / 2).max(0);
        for stroke in handwriting::trace_line(raster) {
            let mapped: Vec<Point> = stroke
                .into_iter()
                .map(|(x, y)| Point {
                    x: target.x + margin + x,
                    y: line_y + y,
                })
                .filter(|point| contains(target, *point))
                .collect();
            add_stroke(&mut strokes, mapped, target, add_weight);
        }
        y += line_height;
    }
    if strokes.is_empty() {
        return Err("content produced no drawable strokes".into());
    }
    Ok(strokes)
}

fn choose_layout(
    text: &str,
    min_px: i32,
    max_px: i32,
    available_width: i32,
    available_height: i32,
) -> Result<(i32, Vec<String>), String> {
    for px in (min_px..=max_px).rev() {
        let mut lines = Vec::new();
        for paragraph in text.lines() {
            if !paragraph.trim().is_empty() {
                lines.extend(handwriting::wrap(
                    paragraph,
                    px as f32,
                    available_width as f32,
                ));
            }
        }
        if !lines.is_empty() && (px * 5 / 4) * lines.len() as i32 <= available_height {
            return Ok((px, lines));
        }
    }
    Err("content does not fit the requested placement box".into())
}

fn add_stroke(strokes: &mut Vec<Vec<Point>>, mapped: Vec<Point>, target: Target, add_weight: bool) {
    if mapped.len() < 2 {
        return;
    }
    if add_weight {
        let bold: Vec<Point> = mapped
            .iter()
            .map(|point| Point {
                x: point.x + BOLD_OFFSET.0,
                y: point.y + BOLD_OFFSET.1,
            })
            .filter(|point| contains(target, *point))
            .collect();
        if bold.len() >= 2 {
            strokes.push(bold);
        }
    }
    strokes.push(mapped);
}

fn parse_table(text: &str) -> Result<Vec<Vec<String>>, String> {
    let mut rows = Vec::new();
    for line in text.lines().map(str::trim).filter(|line| !line.is_empty()) {
        let cells: Vec<String> = line
            .trim_matches('|')
            .split('|')
            .map(|cell| cell.trim().to_string())
            .collect();
        if cells.is_empty() {
            continue;
        }
        let separator = cells.iter().all(|cell| {
            let value = cell.trim_matches(':');
            value.len() >= 3 && value.chars().all(|ch| ch == '-')
        });
        if !separator {
            rows.push(cells);
        }
    }
    if rows.is_empty() {
        return Err("table contains no rows".into());
    }
    Ok(rows)
}

fn parse_vector_numbers(fields: &[&str]) -> Result<Vec<i32>, String> {
    fields
        .iter()
        .map(|value| {
            let number = value
                .parse::<i32>()
                .map_err(|_| format!("invalid vector coordinate '{value}'"))?;
            if !(0..=VECTOR_COORD_MAX).contains(&number) {
                return Err(format!(
                    "vector coordinate {number} is outside 0..={VECTOR_COORD_MAX}"
                ));
            }
            Ok(number)
        })
        .collect()
}

fn parse_arc_values(fields: &[&str], command: &str) -> Result<Vec<i32>, String> {
    if fields.len() != 5 {
        return Err(format!("malformed vector {command} command"));
    }
    let mut values = parse_vector_numbers(&fields[..3])?;
    for value in &fields[3..] {
        let angle = value
            .parse::<i32>()
            .map_err(|_| format!("invalid vector angle '{value}'"))?;
        if !(0..=VECTOR_ANGLE_MAX).contains(&angle) {
            return Err(format!(
                "vector angle {angle} is outside 0..={VECTOR_ANGLE_MAX}"
            ));
        }
        values.push(angle);
    }
    Ok(values)
}

fn vector_pairs(
    values: &[i32],
    minimum_points: usize,
    command: &str,
) -> Result<Vec<(i32, i32)>, String> {
    let point_count = values.len() / 2;
    if values.len() % 2 != 0 || point_count < minimum_points {
        return Err(format!("vector {command} has too few points"));
    }
    if point_count > MAX_VECTOR_PATH_POINTS {
        return Err(format!(
            "vector {command} exceeds {MAX_VECTOR_PATH_POINTS} points"
        ));
    }
    Ok(values
        .chunks_exact(2)
        .map(|pair| (pair[0], pair[1]))
        .collect())
}

fn close_path(points: &mut Vec<(i32, i32)>) {
    if points.first() != points.last() {
        if let Some(first) = points.first().copied() {
            points.push(first);
        }
    }
}

fn validate_rect_geometry(
    x: i32,
    y: i32,
    width: i32,
    height: i32,
    command: &str,
) -> Result<(), String> {
    if width <= 0 || height <= 0 {
        return Err(format!(
            "vector {command} width and height must be positive"
        ));
    }
    if x + width > VECTOR_COORD_MAX || y + height > VECTOR_COORD_MAX {
        return Err(format!(
            "vector {command} extends outside 0..={VECTOR_COORD_MAX}"
        ));
    }
    Ok(())
}

fn rect_points(values: &[i32], command: &str) -> Result<Vec<(i32, i32)>, String> {
    let (x, y, width, height) = (values[0], values[1], values[2], values[3]);
    validate_rect_geometry(x, y, width, height, command)?;
    let (x1, y1) = (x + width, y + height);
    Ok(vec![(x, y), (x1, y), (x1, y1), (x, y1), (x, y)])
}

fn rounded_rect_points(values: &[i32]) -> Result<Vec<(i32, i32)>, String> {
    let (x, y, width, height, radius) = (values[0], values[1], values[2], values[3], values[4]);
    validate_rect_geometry(x, y, width, height, "rrect")?;
    if radius <= 0 || radius > width / 2 || radius > height / 2 {
        return Err(
            "vector rrect radius must be positive and no larger than half either side".into(),
        );
    }
    let (right, bottom) = (x + width, y + height);
    let corners = [
        (right - radius, y + radius, 270, 360),
        (right - radius, bottom - radius, 0, 90),
        (x + radius, bottom - radius, 90, 180),
        (x + radius, y + radius, 180, 270),
    ];
    let mut points = Vec::with_capacity(37);
    for (index, (cx, cy, start, end)) in corners.into_iter().enumerate() {
        let corner = fixed_arc_points(cx, cy, radius, start, end, 8);
        if index == 0 {
            points.extend(corner);
        } else {
            points.extend(corner.into_iter().skip(1));
        }
    }
    close_path(&mut points);
    Ok(points)
}

fn validate_ellipse_geometry(
    cx: i32,
    cy: i32,
    rx: i32,
    ry: i32,
    command: &str,
) -> Result<(), String> {
    if rx <= 0 || ry <= 0 {
        return Err(format!("vector {command} radius must be positive"));
    }
    if cx - rx < 0 || cy - ry < 0 || cx + rx > VECTOR_COORD_MAX || cy + ry > VECTOR_COORD_MAX {
        return Err(format!(
            "vector {command} extends outside 0..={VECTOR_COORD_MAX}"
        ));
    }
    Ok(())
}

fn validate_nonzero_sweep(start: i32, end: i32, command: &str) -> Result<(), String> {
    if start == end {
        return Err(format!("vector {command} angle sweep must be nonzero"));
    }
    Ok(())
}

fn quadratic_points(values: &[i32], samples: usize) -> Vec<(i32, i32)> {
    let (p0, control, p1) = (
        (values[0] as f32, values[1] as f32),
        (values[2] as f32, values[3] as f32),
        (values[4] as f32, values[5] as f32),
    );
    (0..=samples)
        .map(|step| {
            let t = step as f32 / samples as f32;
            let u = 1.0 - t;
            (
                (u * u * p0.0 + 2.0 * u * t * control.0 + t * t * p1.0).round() as i32,
                (u * u * p0.1 + 2.0 * u * t * control.1 + t * t * p1.1).round() as i32,
            )
        })
        .collect()
}

fn cubic_points(values: &[i32], samples: usize) -> Vec<(i32, i32)> {
    let (p0, control0, control1, p1) = (
        (values[0] as f32, values[1] as f32),
        (values[2] as f32, values[3] as f32),
        (values[4] as f32, values[5] as f32),
        (values[6] as f32, values[7] as f32),
    );
    (0..=samples)
        .map(|step| {
            let t = step as f32 / samples as f32;
            let u = 1.0 - t;
            let (u2, t2) = (u * u, t * t);
            (
                (u2 * u * p0.0
                    + 3.0 * u2 * t * control0.0
                    + 3.0 * u * t2 * control1.0
                    + t2 * t * p1.0)
                    .round() as i32,
                (u2 * u * p0.1
                    + 3.0 * u2 * t * control0.1
                    + 3.0 * u * t2 * control1.1
                    + t2 * t * p1.1)
                    .round() as i32,
            )
        })
        .collect()
}

fn fixed_arc_points(
    cx: i32,
    cy: i32,
    radius: i32,
    start: i32,
    end: i32,
    samples: usize,
) -> Vec<(i32, i32)> {
    (0..=samples)
        .map(|step| {
            let degrees = start as f32 + (end - start) as f32 * step as f32 / samples as f32;
            let angle = degrees * PI / 180.0;
            (
                (cx as f32 + radius as f32 * angle.cos()).round() as i32,
                (cy as f32 + radius as f32 * angle.sin()).round() as i32,
            )
        })
        .collect()
}

fn arc_points(cx: i32, cy: i32, radius: i32, start: i32, end: i32) -> Vec<(i32, i32)> {
    let samples = (((end - start).unsigned_abs() as usize + 5) / 6).clamp(4, MAX_ARC_SAMPLES);
    fixed_arc_points(cx, cy, radius, start, end, samples)
}

fn ellipse_points(cx: i32, cy: i32, rx: i32, ry: i32) -> Vec<(i32, i32)> {
    (0..=48)
        .map(|step| {
            let angle = step as f32 / 48.0 * PI * 2.0;
            (
                (cx as f32 + rx as f32 * angle.cos())
                    .round()
                    .clamp(0.0, VECTOR_COORD_MAX as f32) as i32,
                (cy as f32 + ry as f32 * angle.sin())
                    .round()
                    .clamp(0.0, VECTOR_COORD_MAX as f32) as i32,
            )
        })
        .collect()
}

fn append_hatched_polygon(strokes: &mut Vec<Vec<Point>>, points: &[(i32, i32)], target: Target) {
    let mut outline = points.to_vec();
    close_path(&mut outline);
    strokes.push(map_polyline(&outline, target));
    for hatch in polygon_hatches(points) {
        strokes.push(map_polyline(&hatch, target));
    }
}

fn polygon_hatches(points: &[(i32, i32)]) -> Vec<Vec<(i32, i32)>> {
    if points.len() < 3 {
        return Vec::new();
    }
    let (min_y, max_y) = points.iter().fold((i32::MAX, i32::MIN), |bounds, point| {
        (bounds.0.min(point.1), bounds.1.max(point.1))
    });
    if min_y >= max_y {
        return Vec::new();
    }
    let span = max_y - min_y;
    let spacing = HATCH_SPACING
        .max((span + MAX_HATCH_STROKES_PER_SHAPE as i32 - 1) / MAX_HATCH_STROKES_PER_SHAPE as i32);
    let mut scanlines = Vec::new();
    let mut y = min_y + spacing / 2;
    while y < max_y && scanlines.len() < MAX_HATCH_STROKES_PER_SHAPE {
        let mut intersections = Vec::new();
        for index in 0..points.len() {
            let (x0, y0) = points[index];
            let (x1, y1) = points[(index + 1) % points.len()];
            if (y0 <= y && y < y1) || (y1 <= y && y < y0) {
                let x = x0 as f64 + (y - y0) as f64 * (x1 - x0) as f64 / (y1 - y0) as f64;
                intersections.push(x.round() as i32);
            }
        }
        intersections.sort_unstable();
        for pair in intersections.chunks_exact(2) {
            if scanlines.len() >= MAX_HATCH_STROKES_PER_SHAPE {
                break;
            }
            let (mut x0, mut x1) = (pair[0], pair[1]);
            if x1 - x0 > 6 {
                x0 += 3;
                x1 -= 3;
            }
            if x0 < x1 {
                scanlines.push(vec![(x0, y), (x1, y)]);
            }
        }
        y += spacing;
    }
    if scanlines.is_empty() {
        let y = min_y + span / 2;
        let mut intersections = Vec::new();
        for index in 0..points.len() {
            let (x0, y0) = points[index];
            let (x1, y1) = points[(index + 1) % points.len()];
            if (y0 <= y && y < y1) || (y1 <= y && y < y0) {
                let x = x0 as f64 + (y - y0) as f64 * (x1 - x0) as f64 / (y1 - y0) as f64;
                intersections.push(x.round() as i32);
            }
        }
        intersections.sort_unstable();
        for pair in intersections.chunks_exact(2) {
            if pair[0] < pair[1] {
                scanlines.push(vec![(pair[0], y), (pair[1], y)]);
                break;
            }
        }
    }
    scanlines
}

fn append_hatched_ellipse(
    strokes: &mut Vec<Vec<Point>>,
    cx: i32,
    cy: i32,
    rx: i32,
    ry: i32,
    target: Target,
) {
    strokes.push(map_polyline(&ellipse_points(cx, cy, rx, ry), target));
    let spacing = HATCH_SPACING.max(
        (ry * 2 + MAX_HATCH_STROKES_PER_SHAPE as i32 - 1) / MAX_HATCH_STROKES_PER_SHAPE as i32,
    );
    let mut y = cy - ry + spacing / 2;
    let mut count = 0usize;
    while y < cy + ry && count < MAX_HATCH_STROKES_PER_SHAPE {
        let normalized = (y - cy) as f32 / ry as f32;
        let half_width =
            (rx as f32 * (1.0 - normalized * normalized).max(0.0).sqrt()).round() as i32;
        if half_width > 1 {
            strokes.push(map_polyline(
                &[(cx - half_width + 1, y), (cx + half_width - 1, y)],
                target,
            ));
            count += 1;
        }
        y += spacing;
    }
    if count == 0 {
        strokes.push(map_polyline(&[(cx - rx, cy), (cx + rx, cy)], target));
    }
}

fn append_dot(strokes: &mut Vec<Vec<Point>>, x: i32, y: i32, target: Target) {
    let left = (x - DOT_RADIUS).max(0);
    let right = (x + DOT_RADIUS).min(VECTOR_COORD_MAX);
    let top = (y - DOT_RADIUS).max(0);
    let bottom = (y + DOT_RADIUS).min(VECTOR_COORD_MAX);
    strokes.push(map_polyline(&[(left, y), (right, y)], target));
    strokes.push(map_polyline(&[(x, top), (x, bottom)], target));
}

fn validate_vector_complexity(strokes: &[Vec<Point>]) -> Result<(), String> {
    if strokes.len() > MAX_VECTOR_OUTPUT_STROKES {
        return Err(format!(
            "vector expands past {MAX_VECTOR_OUTPUT_STROKES} strokes"
        ));
    }
    let mut points = 0usize;
    let mut path_steps = strokes.len();
    for stroke in strokes {
        points = points.saturating_add(stroke.len());
        if points > MAX_VECTOR_OUTPUT_POINTS {
            return Err(format!(
                "vector expands past {MAX_VECTOR_OUTPUT_POINTS} source points"
            ));
        }
        for pair in stroke.windows(2) {
            let dx = (pair[1].x - pair[0].x).unsigned_abs() as usize;
            let dy = (pair[1].y - pair[0].y).unsigned_abs() as usize;
            path_steps = path_steps.saturating_add(dx.max(dy).max(1));
            if path_steps > MAX_VECTOR_PATH_STEPS {
                return Err(format!(
                    "vector expands past {MAX_VECTOR_PATH_STEPS} planned path points"
                ));
            }
        }
    }
    Ok(())
}

fn map_polyline(points: &[(i32, i32)], target: Target) -> Vec<Point> {
    let margin = 10;
    let width = (target.width as i32 - margin * 2 - 1).max(1);
    let height = (target.height as i32 - margin * 2 - 1).max(1);
    points
        .iter()
        .map(|(x, y)| Point {
            x: target.x + margin + (*x).clamp(0, VECTOR_COORD_MAX) * width / VECTOR_COORD_MAX,
            y: target.y + margin + (*y).clamp(0, VECTOR_COORD_MAX) * height / VECTOR_COORD_MAX,
        })
        .collect()
}

fn map_vector_target(
    x: i32,
    y: i32,
    width: i32,
    height: i32,
    target: Target,
) -> Result<Target, String> {
    let top_left = map_polyline(&[(x, y)], target)[0];
    let bottom_right = map_polyline(
        &[(
            (x + width).min(VECTOR_COORD_MAX),
            (y + height).min(VECTOR_COORD_MAX),
        )],
        target,
    )[0];
    let available_width = target.x + target.width as i32 - top_left.x;
    let available_height = target.y + target.height as i32 - top_left.y;
    if available_width < 24 || available_height < 24 {
        return Err("vector label is too close to the placement edge".into());
    }
    Ok(Target {
        x: top_left.x,
        y: top_left.y,
        width: (bottom_right.x - top_left.x).max(24).min(available_width) as u32,
        height: (bottom_right.y - top_left.y).max(24).min(available_height) as u32,
    })
}

fn contains(target: Target, point: Point) -> bool {
    point.x >= target.x
        && point.y >= target.y
        && point.x < target.x + target.width as i32
        && point.y < target.y + target.height as i32
}

fn build_job(
    strokes: Vec<Vec<Point>>,
    canvas_width: u32,
    canvas_height: u32,
) -> Result<StrokeJob, String> {
    let strokes: Vec<Vec<Point>> = strokes
        .into_iter()
        .filter(|stroke| stroke.len() >= 2)
        .collect();
    if strokes.is_empty() {
        return Err("renderer produced no drawable strokes".into());
    }
    if strokes.len() > MAX_OUTPUT_STROKES {
        return Err(format!(
            "renderer produced too many strokes ({})",
            strokes.len()
        ));
    }
    Ok(StrokeJob {
        canvas_width,
        canvas_height,
        strokes,
    })
}

fn validate_target(target: Target, canvas_width: u32, canvas_height: u32) -> Result<(), String> {
    if !(2..=4096).contains(&canvas_width) || !(2..=4096).contains(&canvas_height) {
        return Err("canvas must be within 2..=4096 pixels per axis".into());
    }
    if target.x < 0 || target.y < 0 || target.width < 48 || target.height < 48 {
        return Err("placement box must be positive and at least 48 x 48".into());
    }
    if target.x as u32 + target.width > canvas_width
        || target.y as u32 + target.height > canvas_height
    {
        return Err("placement box extends outside the canvas".into());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn target() -> Target {
        Target {
            x: 80,
            y: 420,
            width: 794,
            height: 540,
        }
    }

    #[test]
    fn renders_mixed_traditional_chinese_inside_target() {
        let box_target = target();
        let job = text_to_job("你好，Paper Agent。", box_target, 954, 1696).unwrap();
        assert!(!job.strokes.is_empty());
        assert!(job
            .strokes
            .iter()
            .flatten()
            .all(|point| contains(box_target, *point)));
    }

    #[test]
    fn renders_a_real_table_grid() {
        let job = table_to_job(
            "| 項目 | 數值 |\n| --- | --- |\n| A | 10 |",
            target(),
            954,
            1696,
        )
        .unwrap();
        assert!(job.strokes.len() > 6);
    }

    #[test]
    fn accepts_only_the_bounded_vector_language() {
        let vector = "paper-agent-vector 1\nrect 100 100 300 200\narrow 400 200 800 700\ncircle 500 500 120\nlabel 100 50 300 100 Start\npolygon 100 400 250 300 400 400\ncurve 100 500 250 350 400 500\ncurve 450 500 550 350 650 650 750 500\nrrect 500 100 300 200 40\narc 500 500 120 0 180\nwedge 500 500 120 180 360\ndot 500 500\nfillpoly 100 700 250 550 400 700\nbox 600 700 200 150\ndisc 700 500 80\nfillellipse 300 800 160 80";
        let job = vector_to_job(vector, target(), 954, 1696).unwrap();
        assert!(job.strokes.len() >= 30);
        assert!(job
            .strokes
            .iter()
            .flatten()
            .all(|point| contains(target(), *point)));
        assert!(vector_to_job("<svg><script/></svg>", target(), 954, 1696).is_err());
    }

    #[test]
    fn expanded_vector_language_is_strictly_bounded() {
        let mut path = String::from("paper-agent-vector 1\npolygon");
        for coordinate in 0..65 {
            path.push_str(&format!(" {coordinate} {coordinate}"));
        }
        assert!(vector_to_job(&path, target(), 954, 1696)
            .unwrap_err()
            .contains("64 points"));
        assert!(vector_to_job(
            "paper-agent-vector 1\narc 500 500 100 0 361",
            target(),
            954,
            1696,
        )
        .unwrap_err()
        .contains("angle 361"));
        assert!(vector_to_job(
            "paper-agent-vector 1\nrrect 900 900 200 100 20",
            target(),
            954,
            1696,
        )
        .unwrap_err()
        .contains("extends outside"));

        let many_fills = format!(
            "paper-agent-vector 1\n{}",
            std::iter::repeat("box 0 0 1000 1000")
                .take(24)
                .collect::<Vec<_>>()
                .join("\n")
        );
        assert!(vector_to_job(&many_fills, target(), 954, 1696)
            .unwrap_err()
            .contains("512 strokes"));
    }

    #[test]
    fn renders_a_mixed_rich_document_in_block_order() {
        fn encoded(value: &str) -> String {
            value
                .as_bytes()
                .iter()
                .map(|byte| format!("{byte:02x}"))
                .collect()
        }
        let rich = "# 結果\n\n這是 **粗體** 和 `code`。\n\n- 第一項\n- 第二項\n\n```rs\nlet answer = 42;\n```";
        let table = "| 項目 | 數值 |\n| --- | ---: |\n| A | 10 |";
        let vector = "paper-agent-vector 1\nrect 100 100 300 200\narrow 400 200 800 700";
        let manifest = format!(
            "{DOCUMENT_HEADER}\nblock rich {}\nblock table {}\nblock vector {}\n",
            encoded(rich),
            encoded(table),
            encoded(vector)
        );
        let job = document_to_job(
            &manifest,
            Target {
                x: 50,
                y: 180,
                width: 854,
                height: 1400,
            },
            954,
            1696,
        )
        .unwrap();
        assert!(job.strokes.len() > 20);
        assert!(job
            .strokes
            .iter()
            .flatten()
            .all(|point| point.x >= 50 && point.x < 904 && point.y >= 180 && point.y < 1580));
    }

    #[test]
    fn parses_markdown_styles_without_executing_code() {
        let lines = parse_rich_lines("## Title\n- **bold** and `inline`\n```svg\n<script/>\n```");
        assert!(matches!(lines[0].kind, RichLineKind::Heading(2)));
        assert_eq!(lines[1].kind, RichLineKind::List);
        assert!(lines[1]
            .spans
            .iter()
            .any(|span| span.bold && span.text.contains("bold")));
        assert!(lines[1]
            .spans
            .iter()
            .any(|span| span.code && span.text == "inline"));
        assert_eq!(lines[2].kind, RichLineKind::Code);
        assert_eq!(lines[2].spans[0].text, "<script/>");
    }

    #[test]
    fn rejects_non_allowlisted_document_blocks() {
        assert!(document_to_job(
            "paper-agent-document 1\nblock image 00\n",
            target(),
            954,
            1696
        )
        .is_err());
        assert!(document_to_job(
            "paper-agent-document 1\nblock rich not-hex\n",
            target(),
            954,
            1696
        )
        .is_err());
    }
}

// SPDX-License-Identifier: MIT

use std::fmt::Write as _;
use std::fs;
use std::path::Path;

const HEADER: &str = "paper-agent-strokes 1";
const MAX_CANVAS: u32 = 4096;
const MAX_STROKES: usize = 4096;
const MAX_POINTS: usize = 250_000;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Point {
    pub x: i32,
    pub y: i32,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StrokeJob {
    pub canvas_width: u32,
    pub canvas_height: u32,
    pub strokes: Vec<Vec<Point>>,
}

impl StrokeJob {
    pub fn from_path(path: &Path) -> Result<Self, String> {
        let text =
            fs::read_to_string(path).map_err(|e| format!("cannot read {}: {e}", path.display()))?;
        Self::parse(&text)
    }

    /// Parse the deliberately small, non-executable stroke format:
    ///
    /// ```text
    /// paper-agent-strokes 1
    /// canvas 954 1696
    /// stroke
    /// 100 200
    /// 110 210
    /// end
    /// ```
    pub fn parse(text: &str) -> Result<Self, String> {
        let mut meaningful = text
            .lines()
            .enumerate()
            .map(|(i, line)| (i + 1, line.trim()))
            .filter(|(_, line)| !line.is_empty() && !line.starts_with('#'));

        let Some((line_no, header)) = meaningful.next() else {
            return Err("empty stroke job".into());
        };
        if header != HEADER {
            return Err(format!("line {line_no}: expected '{HEADER}'"));
        }

        let Some((line_no, canvas_line)) = meaningful.next() else {
            return Err("missing canvas line".into());
        };
        let canvas = fields(canvas_line);
        if canvas.len() != 3 || canvas[0] != "canvas" {
            return Err(format!("line {line_no}: expected 'canvas WIDTH HEIGHT'"));
        }
        let canvas_width = parse_u32(canvas[1], line_no, "canvas width")?;
        let canvas_height = parse_u32(canvas[2], line_no, "canvas height")?;
        if !(2..=MAX_CANVAS).contains(&canvas_width) || !(2..=MAX_CANVAS).contains(&canvas_height) {
            return Err(format!(
                "line {line_no}: canvas must be within 2..={MAX_CANVAS} pixels per axis"
            ));
        }

        let mut strokes: Vec<Vec<Point>> = Vec::new();
        let mut current: Option<Vec<Point>> = None;
        let mut point_count = 0usize;

        for (line_no, line) in meaningful {
            match line {
                "stroke" => {
                    if current.is_some() {
                        return Err(format!("line {line_no}: nested stroke"));
                    }
                    if strokes.len() >= MAX_STROKES {
                        return Err(format!("line {line_no}: more than {MAX_STROKES} strokes"));
                    }
                    current = Some(Vec::new());
                }
                "end" => {
                    let Some(stroke) = current.take() else {
                        return Err(format!("line {line_no}: end without stroke"));
                    };
                    if stroke.is_empty() {
                        return Err(format!("line {line_no}: empty stroke"));
                    }
                    strokes.push(stroke);
                }
                _ => {
                    let Some(stroke) = current.as_mut() else {
                        return Err(format!("line {line_no}: point outside a stroke"));
                    };
                    let point = fields(line);
                    if point.len() != 2 {
                        return Err(format!("line {line_no}: expected 'X Y'"));
                    }
                    let x = parse_i32(point[0], line_no, "x")?;
                    let y = parse_i32(point[1], line_no, "y")?;
                    if x < 0 || x >= canvas_width as i32 || y < 0 || y >= canvas_height as i32 {
                        return Err(format!(
                            "line {line_no}: point ({x},{y}) outside {}x{} canvas",
                            canvas_width, canvas_height
                        ));
                    }
                    point_count += 1;
                    if point_count > MAX_POINTS {
                        return Err(format!("line {line_no}: more than {MAX_POINTS} points"));
                    }
                    stroke.push(Point { x, y });
                }
            }
        }
        if current.is_some() {
            return Err("unterminated stroke".into());
        }
        if strokes.is_empty() {
            return Err("stroke job contains no strokes".into());
        }
        Ok(Self {
            canvas_width,
            canvas_height,
            strokes,
        })
    }

    pub fn point_count(&self) -> usize {
        self.strokes.iter().map(Vec::len).sum()
    }

    pub fn to_text(&self) -> String {
        let mut out = String::new();
        let _ = writeln!(out, "{HEADER}");
        let _ = writeln!(out, "canvas {} {}", self.canvas_width, self.canvas_height);
        for stroke in &self.strokes {
            let _ = writeln!(out, "stroke");
            for point in stroke {
                let _ = writeln!(out, "{} {}", point.x, point.y);
            }
            let _ = writeln!(out, "end");
        }
        out
    }

    pub fn write_to(&self, path: &Path) -> Result<(), String> {
        fs::write(path, self.to_text()).map_err(|e| format!("cannot write {}: {e}", path.display()))
    }
}

fn fields(line: &str) -> Vec<&str> {
    line.split_ascii_whitespace().collect()
}

fn parse_u32(value: &str, line: usize, label: &str) -> Result<u32, String> {
    value
        .parse()
        .map_err(|_| format!("line {line}: invalid {label} '{value}'"))
}

fn parse_i32(value: &str, line: usize, label: &str) -> Result<i32, String> {
    value
        .parse()
        .map_err(|_| format!("line {line}: invalid {label} '{value}'"))
}

#[cfg(test)]
mod tests {
    use super::*;

    const VALID: &str = "\
paper-agent-strokes 1
canvas 954 1696
stroke
100 200
110 210
end
stroke
953 1695
end
";

    #[test]
    fn parses_a_bounded_job() {
        let job = StrokeJob::parse(VALID).unwrap();
        assert_eq!((job.canvas_width, job.canvas_height), (954, 1696));
        assert_eq!(job.strokes.len(), 2);
        assert_eq!(job.point_count(), 3);
    }

    #[test]
    fn rejects_an_unknown_version() {
        assert!(StrokeJob::parse(&VALID.replace("strokes 1", "strokes 2")).is_err());
    }

    #[test]
    fn rejects_points_outside_the_canvas() {
        assert!(StrokeJob::parse(&VALID.replace("953 1695", "954 1695")).is_err());
    }

    #[test]
    fn rejects_unterminated_strokes() {
        assert!(StrokeJob::parse(VALID.trim_end_matches("end\n")).is_err());
    }

    #[test]
    fn serialization_round_trips() {
        let original = StrokeJob::parse(VALID).unwrap();
        assert_eq!(StrokeJob::parse(&original.to_text()).unwrap(), original);
    }
}

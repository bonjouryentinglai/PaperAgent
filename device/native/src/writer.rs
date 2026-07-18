// SPDX-License-Identifier: MIT
//
// Pen-event sequencing and pacing are adapted from smart_remarkable's MIT
// `src/pen.rs`. This Chiappa port discovers the marker and ABS ranges at
// runtime and never edits a reMarkable document file.

use crate::device;
use crate::geometry::canvas_point_to_axes;
use crate::job::{Point, StrokeJob};
use crate::protocol;
use std::ffi::CString;
use std::fs::{self, OpenOptions};
use std::io;
use std::os::fd::RawFd;
use std::process::Command;
use std::thread::sleep;
use std::time::Duration;

const LOCK_PATH: &str = "/run/paper-agent-native.lock";
const MAX_INTERPOLATED_POINTS: usize = 500_000;
const SESSION_SETTLE: Duration = Duration::from_millis(150);

pub fn write_job(job: &StrokeJob) -> Result<(), String> {
    let mut session = WriterSession::open()?;
    session.write_job(job)
}

/// Keep one Marker descriptor and one writer lock across sentence-sized jobs.
/// The streaming oracle uses this so the first complete sentence can appear
/// immediately without reopening the device (and without creating an obvious
/// Undo boundary) for every following sentence.
pub struct WriterSession {
    _lock: WriterLock,
    probe: device::Probe,
    pen: NativePen,
}

impl WriterSession {
    pub fn open() -> Result<Self, String> {
        ensure_xochitl_active()?;
        let lock = WriterLock::acquire()?;
        let probe = device::probe()?;
        let fd = open_writer(&probe.marker_path)
            .map_err(|e| format!("cannot open {} for writeback: {e}", probe.marker_path))?;
        // The QMD restores the primary pen as soon as its selection closes.
        // Keep one small guard for direct/manual callers, but do not pay the
        // previous 900 ms delay before every streamed sentence.
        sleep(SESSION_SETTLE);
        Ok(Self {
            _lock: lock,
            probe,
            pen: NativePen {
                fd,
                touching: false,
            },
        })
    }

    pub fn write_job(&mut self, job: &StrokeJob) -> Result<(), String> {
        let estimated = estimated_points(job)?;
        let canvas = (job.canvas_width, job.canvas_height);
        eprintln!(
            "paper-agent-native: streaming {} strokes, {} source points, {estimated} planned points",
            job.strokes.len(),
            job.point_count()
        );

        for stroke in &job.strokes {
            let first = canvas_point_to_axes(
                (stroke[0].x, stroke[0].y),
                canvas,
                self.probe.x_axis,
                self.probe.y_axis,
            )?;
            self.pen.begin(first)?;
            let mut previous = stroke[0];
            let mut paced = 0usize;
            for &next in stroke.iter().skip(1) {
                interpolate(previous, next, |point| {
                    let mapped = canvas_point_to_axes(
                        (point.x, point.y),
                        canvas,
                        self.probe.x_axis,
                        self.probe.y_axis,
                    )?;
                    self.pen.move_to(mapped)?;
                    paced += 1;
                    if paced % 100 == 0 {
                        sleep(Duration::from_millis(1));
                    }
                    Ok(())
                })?;
                previous = next;
            }
            self.pen.end()?;
            sleep(Duration::from_millis(3));
        }
        Ok(())
    }
}

fn ensure_xochitl_active() -> Result<(), String> {
    let status = Command::new("systemctl")
        .args(["is-active", "--quiet", "xochitl"])
        .status()
        .map_err(|e| format!("cannot check xochitl state: {e}"))?;
    if status.success() {
        Ok(())
    } else {
        Err("xochitl is not active; refusing native writeback".into())
    }
}

fn estimated_points(job: &StrokeJob) -> Result<usize, String> {
    let mut total = job.strokes.len();
    for stroke in &job.strokes {
        for pair in stroke.windows(2) {
            let dx = (pair[1].x - pair[0].x).unsigned_abs() as usize;
            let dy = (pair[1].y - pair[0].y).unsigned_abs() as usize;
            total = total.saturating_add(dx.max(dy).max(1));
            if total > MAX_INTERPOLATED_POINTS {
                return Err(format!(
                    "job expands past {MAX_INTERPOLATED_POINTS} interpolated points"
                ));
            }
        }
    }
    Ok(total)
}

fn interpolate(
    from: Point,
    to: Point,
    mut emit: impl FnMut(Point) -> Result<(), String>,
) -> Result<(), String> {
    let dx = to.x - from.x;
    let dy = to.y - from.y;
    let steps = dx.unsigned_abs().max(dy.unsigned_abs()).max(1) as i32;
    for i in 1..=steps {
        emit(Point {
            x: from.x + dx * i / steps,
            y: from.y + dy * i / steps,
        })?;
    }
    Ok(())
}

struct WriterLock;

impl WriterLock {
    fn acquire() -> Result<Self, String> {
        OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(LOCK_PATH)
            .map_err(|e| format!("native writer already active or lock unavailable: {e}"))?;
        Ok(Self)
    }
}

impl Drop for WriterLock {
    fn drop(&mut self) {
        let _ = fs::remove_file(LOCK_PATH);
    }
}

struct NativePen {
    fd: RawFd,
    touching: bool,
}

impl NativePen {
    fn begin(&mut self, point: (i32, i32)) -> Result<(), String> {
        self.send(&protocol::hover_at(point.0, point.1))?;
        sleep(Duration::from_millis(2));
        self.send(&protocol::touch_down())?;
        self.touching = true;
        sleep(Duration::from_millis(2));
        Ok(())
    }

    fn move_to(&mut self, point: (i32, i32)) -> Result<(), String> {
        self.send(&protocol::move_to(point.0, point.1))
    }

    fn end(&mut self) -> Result<(), String> {
        self.send(&protocol::pen_up())?;
        self.touching = false;
        Ok(())
    }

    fn send(&mut self, events: &[protocol::Event]) -> Result<(), String> {
        let native: Vec<InputEvent> = events
            .iter()
            .map(|event| InputEvent {
                time: libc::timeval {
                    tv_sec: 0,
                    tv_usec: 0,
                },
                event_type: event.event_type,
                code: event.code,
                value: event.value,
            })
            .collect();
        let bytes = native.len() * std::mem::size_of::<InputEvent>();
        let written = unsafe { libc::write(self.fd, native.as_ptr().cast(), bytes) };
        if written < 0 {
            return Err(format!(
                "marker write failed: {}",
                io::Error::last_os_error()
            ));
        }
        if written as usize != bytes {
            return Err(format!("short marker write: {written} of {bytes} bytes"));
        }
        Ok(())
    }
}

impl Drop for NativePen {
    fn drop(&mut self) {
        if self.touching {
            let _ = self.send(&protocol::pen_up());
        }
        unsafe { libc::close(self.fd) };
    }
}

#[repr(C)]
struct InputEvent {
    time: libc::timeval,
    event_type: u16,
    code: u16,
    value: i32,
}

fn open_writer(path: &str) -> io::Result<RawFd> {
    let cpath = CString::new(path).map_err(|_| io::Error::other("invalid device path"))?;
    let fd = unsafe { libc::open(cpath.as_ptr(), libc::O_WRONLY | libc::O_NONBLOCK) };
    if fd < 0 {
        Err(io::Error::last_os_error())
    } else {
        Ok(fd)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn interpolation_includes_target_not_source() {
        let mut points = Vec::new();
        interpolate(Point { x: 10, y: 20 }, Point { x: 13, y: 20 }, |p| {
            points.push(p);
            Ok(())
        })
        .unwrap();
        assert_eq!(
            points,
            vec![
                Point { x: 11, y: 20 },
                Point { x: 12, y: 20 },
                Point { x: 13, y: 20 }
            ]
        );
    }

    #[test]
    fn rejects_pathological_interpolation_work() {
        let job = StrokeJob {
            canvas_width: 4096,
            canvas_height: 4096,
            strokes: vec![(0..130)
                .map(|i| Point {
                    x: if i % 2 == 0 { 0 } else { 4095 },
                    y: 0,
                })
                .collect()],
        };
        assert!(estimated_points(&job).is_err());
    }

    #[test]
    fn input_event_layout_matches_chiappa_kernel_abi() {
        assert_eq!(std::mem::size_of::<InputEvent>(), 24);
    }
}

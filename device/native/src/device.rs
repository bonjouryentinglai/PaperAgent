// SPDX-License-Identifier: MIT
//
// Device discovery replaces smart_remarkable's fixed `/dev/input/event1` or
// `event2` choice. The event device and ABS ranges are queried from the running
// Chiappa kernel, so an OTA may change numbering without moving the ink.

use crate::geometry::AxisRange;
use std::ffi::CString;
use std::fs;
use std::io;
use std::os::fd::RawFd;

const EVIOCGABS_X: libc::c_ulong = 0x8018_4540;
const EVIOCGABS_Y: libc::c_ulong = 0x8018_4541;

#[derive(Debug)]
pub struct Probe {
    pub machine: String,
    pub marker_path: String,
    pub marker_name: String,
    pub x_axis: AxisRange,
    pub y_axis: AxisRange,
    pub uinput_available: bool,
}

pub fn probe() -> Result<Probe, String> {
    let machine = read_trimmed("/sys/devices/soc0/machine")
        .or_else(|| read_trimmed("/etc/hwrevision"))
        .unwrap_or_else(|| "unknown".to_string());
    if !machine.to_ascii_lowercase().contains("chiappa") {
        return Err(format!(
            "unsupported device '{machine}'; expected reMarkable Chiappa"
        ));
    }

    let (marker_path, marker_name) = find_marker_device()?;
    let fd = open_read_only(&marker_path)
        .map_err(|e| format!("cannot open {marker_path} read-only: {e}"))?;
    let x_axis = query_axis(fd, EVIOCGABS_X, "ABS_X");
    let y_axis = query_axis(fd, EVIOCGABS_Y, "ABS_Y");
    unsafe { libc::close(fd) };

    Ok(Probe {
        machine,
        marker_path,
        marker_name,
        x_axis: x_axis?,
        y_axis: y_axis?,
        uinput_available: std::path::Path::new("/dev/uinput").exists(),
    })
}

fn read_trimmed(path: &str) -> Option<String> {
    fs::read_to_string(path)
        .ok()
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty())
}

fn find_marker_device() -> Result<(String, String), String> {
    for i in 0..32 {
        let name_path = format!("/sys/class/input/event{i}/device/name");
        let Some(name) = read_trimmed(&name_path) else {
            continue;
        };
        if name.to_ascii_lowercase().contains("marker") {
            return Ok((format!("/dev/input/event{i}"), name));
        }
    }
    Err("no input device with 'marker' in its kernel name".into())
}

fn open_read_only(path: &str) -> io::Result<RawFd> {
    let cpath = CString::new(path).map_err(|_| io::Error::other("invalid device path"))?;
    let fd = unsafe { libc::open(cpath.as_ptr(), libc::O_RDONLY | libc::O_NONBLOCK) };
    if fd < 0 {
        Err(io::Error::last_os_error())
    } else {
        Ok(fd)
    }
}

fn query_axis(fd: RawFd, request: libc::c_ulong, name: &str) -> Result<AxisRange, String> {
    let mut info = [0i32; 6]; // value, min, max, fuzz, flat, resolution
    let rc = unsafe { libc::ioctl(fd, request, info.as_mut_ptr()) };
    if rc < 0 {
        return Err(format!(
            "cannot query {name}: {}",
            io::Error::last_os_error()
        ));
    }
    AxisRange::new(info[1], info[2]).map_err(|e| format!("{name}: {e}"))
}

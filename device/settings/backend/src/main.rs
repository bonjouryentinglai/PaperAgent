// SPDX-License-Identifier: GPL-3.0-only
//
// AppLoad protocol adapter derived from rm-appload's Rust backend client:
// https://github.com/asivery/rm-appload

use libc::{c_void, sockaddr_un, AF_UNIX, SOCK_SEQPACKET};
use std::env;
use std::io::{self, Write};
use std::mem;
use std::os::unix::ffi::OsStrExt;
use std::process::{Command, Stdio};

const MSG_REFRESH: u32 = 1;
const MSG_APPLY: u32 = 2;
const MSG_STATE: u32 = 100;
const MSG_APPLYING: u32 = 101;
const MSG_APPLIED: u32 = 102;
const MSG_ERROR: u32 = 199;
const MSG_SYSTEM_TERMINATE: u32 = 0xFFFF_FFFF;
const MSG_SYSTEM_NEW_COORDINATOR: u32 = 0xFFFF_FFFE;
const MAX_MESSAGE: usize = 16 * 1024;
const NODE: &str = "/home/root/node/bin/node";
const CONTROLLER: &str = "/home/root/paper-agent/native/settings-controller.mjs";

#[repr(C)]
#[derive(Default)]
struct MessageHeader {
    msg_type: u32,
    length: u32,
}

fn connect_socket(socket_path: &std::ffi::OsStr) -> io::Result<i32> {
    let bytes = socket_path.as_bytes();
    let fd = unsafe { libc::socket(AF_UNIX, SOCK_SEQPACKET, 0) };
    if fd < 0 {
        return Err(io::Error::last_os_error());
    }
    let mut address: sockaddr_un = unsafe { mem::zeroed() };
    address.sun_family = AF_UNIX as u16;
    if bytes.len() >= address.sun_path.len() {
        unsafe { libc::close(fd) };
        return Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            "AppLoad socket path is too long",
        ));
    }
    for (target, source) in address.sun_path.iter_mut().zip(bytes.iter()) {
        *target = *source as libc::c_char;
    }
    let result = unsafe {
        libc::connect(
            fd,
            &address as *const _ as *const libc::sockaddr,
            mem::size_of::<sockaddr_un>() as libc::socklen_t,
        )
    };
    if result != 0 {
        let error = io::Error::last_os_error();
        unsafe { libc::close(fd) };
        return Err(error);
    }
    Ok(fd)
}

fn send_packet(fd: i32, data: &[u8]) -> io::Result<()> {
    let sent = unsafe { libc::send(fd, data.as_ptr() as *const c_void, data.len(), 0) };
    if sent < 0 {
        return Err(io::Error::last_os_error());
    }
    if sent as usize != data.len() {
        return Err(io::Error::new(
            io::ErrorKind::WriteZero,
            "short AppLoad send",
        ));
    }
    Ok(())
}

fn send_message(fd: i32, msg_type: u32, contents: &str) -> io::Result<()> {
    let data = contents.as_bytes();
    let header = MessageHeader {
        msg_type,
        length: data.len() as u32,
    };
    let header_bytes = unsafe {
        std::slice::from_raw_parts(
            &header as *const _ as *const u8,
            mem::size_of::<MessageHeader>(),
        )
    };
    send_packet(fd, header_bytes)?;
    if !data.is_empty() {
        send_packet(fd, data)?;
    }
    Ok(())
}

fn receive_packet(fd: i32, buffer: &mut [u8]) -> io::Result<usize> {
    let received = unsafe { libc::recv(fd, buffer.as_mut_ptr() as *mut c_void, buffer.len(), 0) };
    if received < 0 {
        return Err(io::Error::last_os_error());
    }
    Ok(received as usize)
}

fn receive_message(fd: i32) -> io::Result<Option<(u32, String)>> {
    let mut header = MessageHeader::default();
    let header_bytes = unsafe {
        std::slice::from_raw_parts_mut(
            &mut header as *mut _ as *mut u8,
            mem::size_of::<MessageHeader>(),
        )
    };
    let received = receive_packet(fd, header_bytes)?;
    if received == 0 {
        return Ok(None);
    }
    if received != header_bytes.len() || header.length as usize > MAX_MESSAGE {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            "invalid AppLoad message",
        ));
    }
    let mut body = vec![0; header.length as usize];
    if !body.is_empty() && receive_packet(fd, &mut body)? != body.len() {
        return Err(io::Error::new(
            io::ErrorKind::UnexpectedEof,
            "incomplete AppLoad message",
        ));
    }
    Ok(Some((
        header.msg_type,
        String::from_utf8_lossy(&body).into_owned(),
    )))
}

fn controller(command: &str, input: Option<&str>) -> Result<String, String> {
    let mut child = Command::new(NODE)
        .arg(CONTROLLER)
        .arg(command)
        .stdin(if input.is_some() {
            Stdio::piped()
        } else {
            Stdio::null()
        })
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|error| format!("Could not start settings controller: {error}"))?;
    if let Some(value) = input {
        child
            .stdin
            .take()
            .ok_or_else(|| "Could not open settings input".to_string())?
            .write_all(value.as_bytes())
            .map_err(|error| format!("Could not send settings: {error}"))?;
    }
    let output = child
        .wait_with_output()
        .map_err(|error| format!("Settings controller failed: {error}"))?;
    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
    let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
    if output.status.success() {
        Ok(stdout)
    } else if stderr.is_empty() {
        Err("Paper Agent could not apply settings.".to_string())
    } else {
        Err(stderr.chars().take(600).collect())
    }
}

fn send_state(fd: i32) -> io::Result<()> {
    match controller("get", None) {
        Ok(state) => send_message(fd, MSG_STATE, &state),
        Err(error) => send_message(fd, MSG_ERROR, &error),
    }
}

fn run() -> Result<(), String> {
    let socket = env::args_os()
        .nth(1)
        .ok_or_else(|| "AppLoad did not provide a backend socket".to_string())?;
    let fd = connect_socket(&socket).map_err(|error| format!("AppLoad connection: {error}"))?;
    loop {
        let Some((msg_type, contents)) =
            receive_message(fd).map_err(|error| format!("AppLoad receive: {error}"))?
        else {
            break;
        };
        match msg_type {
            MSG_SYSTEM_TERMINATE => break,
            MSG_SYSTEM_NEW_COORDINATOR | MSG_REFRESH => {
                send_state(fd).map_err(|error| format!("AppLoad send: {error}"))?;
            }
            MSG_APPLY => {
                send_message(fd, MSG_APPLYING, "")
                    .map_err(|error| format!("AppLoad send: {error}"))?;
                match controller("apply", Some(&contents)) {
                    Ok(state) => send_message(fd, MSG_APPLIED, &state),
                    Err(error) => send_message(fd, MSG_ERROR, &error),
                }
                .map_err(|error| format!("AppLoad send: {error}"))?;
            }
            _ => {
                send_message(fd, MSG_ERROR, "Unsupported settings request.")
                    .map_err(|error| format!("AppLoad send: {error}"))?;
            }
        }
    }
    unsafe { libc::close(fd) };
    Ok(())
}

fn main() {
    if let Err(error) = run() {
        eprintln!("{error}");
        std::process::exit(1);
    }
}

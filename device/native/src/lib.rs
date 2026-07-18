// SPDX-License-Identifier: MIT
//
// The native-writeback architecture and pen protocol are adapted from
// yangg1224/smart_remarkable at cb787065281b7211b012bd5e5d9be751fe5adaef.
// This crate replaces its fixed Ferrari/RM2 paths and coordinate constants
// with Chiappa discovery and runtime axis queries.

pub mod geometry;
pub mod handwriting;
pub mod image;
pub mod job;
pub mod protocol;
pub mod renderer;
pub mod skeleton;
pub mod traditional;

#[cfg(target_os = "linux")]
pub mod device;
#[cfg(target_os = "linux")]
pub mod writer;

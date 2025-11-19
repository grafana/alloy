// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Defines FFI error codes and their conversion from Rust error types.

use std::io;
use symblib::{dwarf, gosym, objfile, retpads, symbconv};

pub type FfiResult<T> = Result<T, StatusCode>;

/// Error codes exposed to the C API.
///
/// The errors that we are exposing are currently rather coarsely mapped.
/// In the future, it probably makes sense to expose sub-errors more granularly.
#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, thiserror::Error)]
#[non_exhaustive]
pub enum StatusCode {
    #[error("OK: not actually an error")]
    Ok = 0,

    #[error("IO error")]
    IoMisc = 1,

    #[error("IO error: file not found")]
    IoFileNotFound = 2,

    #[error("Object file reading error")]
    Objfile = 3,

    #[error("DWARF reading error")]
    Dwarf = 4,

    #[error("Symbol conversion error")]
    Symbconv = 5,

    #[error("Return pad extraction error")]
    Retpad = 6,

    #[error("Invalid UTF-8")]
    BadUtf8 = 7,

    #[error("The channel was already closed in a previous call")]
    AlreadyClosed = 8,

    #[error("Point resolver error")]
    PointResolver = 9,
}

impl From<StatusCode> for FfiResult<()> {
    fn from(code: StatusCode) -> Self {
        if code == StatusCode::Ok {
            Ok(())
        } else {
            Err(code)
        }
    }
}

impl From<FfiResult<()>> for StatusCode {
    fn from(result: FfiResult<()>) -> Self {
        match result {
            Ok(()) => StatusCode::Ok,
            Err(e) => e,
        }
    }
}

impl From<io::Error> for StatusCode {
    fn from(e: io::Error) -> Self {
        if e.kind() == io::ErrorKind::NotFound {
            StatusCode::IoFileNotFound
        } else {
            StatusCode::IoMisc
        }
    }
}

impl From<objfile::Error> for StatusCode {
    fn from(e: objfile::Error) -> Self {
        match e {
            objfile::Error::IO(io) => io.into(),
            _ => Self::Objfile,
        }
    }
}

impl From<dwarf::Error> for StatusCode {
    fn from(e: dwarf::Error) -> Self {
        match e {
            dwarf::Error::Objfile(x) => x.into(),
            _ => Self::Dwarf,
        }
    }
}

impl From<symbconv::Error> for StatusCode {
    fn from(e: symbconv::Error) -> Self {
        match e {
            symbconv::Error::Objfile(x) => x.into(),
            _ => Self::Symbconv,
        }
    }
}

impl From<symbconv::multi::Error> for StatusCode {
    fn from(e: symbconv::multi::Error) -> Self {
        symbconv::Error::Multi(e).into()
    }
}

impl From<std::str::Utf8Error> for StatusCode {
    fn from(_: std::str::Utf8Error) -> Self {
        Self::BadUtf8
    }
}

impl From<retpads::Error> for StatusCode {
    fn from(_: retpads::Error) -> Self {
        Self::Retpad
    }
}

impl From<gosym::Error> for StatusCode {
    fn from(_: gosym::Error) -> Self {
        StatusCode::Symbconv
    }
}

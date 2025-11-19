// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

use std::ffi::{c_char, CString};
use std::{mem, ptr};

/// Read-only, nullable, owned FFI-safe string type.
#[derive(Debug)]
#[repr(transparent)]
pub struct SymblibString(*mut c_char);

impl From<Option<String>> for SymblibString {
    fn from(maybe_str: Option<String>) -> Self {
        match maybe_str {
            Some(s) => s.into(),
            None => SymblibString(ptr::null_mut()),
        }
    }
}

impl From<String> for SymblibString {
    fn from(x: String) -> Self {
        Self(unsafe { CString::from_vec_unchecked(x.into_bytes()).into_raw() })
    }
}

impl From<SymblibString> for Option<String> {
    fn from(maybe_str: SymblibString) -> Self {
        if maybe_str.0.is_null() {
            None
        } else {
            let cstr = unsafe { CString::from_raw(maybe_str.0) };
            mem::forget(maybe_str);
            Some(cstr.into_string().unwrap())
        }
    }
}

impl Drop for SymblibString {
    fn drop(&mut self) {
        if !self.0.is_null() {
            drop(unsafe { CString::from_raw(self.0 as _) });
            self.0 = ptr::null_mut();
        }
    }
}

unsafe impl Send for SymblibString {}

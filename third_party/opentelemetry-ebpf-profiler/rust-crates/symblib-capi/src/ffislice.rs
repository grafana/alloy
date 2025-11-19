// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

use std::{marker::PhantomData, mem, ptr, slice};

/// Read-only, owned FFI-safe owned slice type.
///
/// The caller must ensure that `T` is FFI-safe (`#[repr(C)]`).
#[repr(C)]
#[derive(Debug)]
pub struct SymblibSlice<T> {
    /// Data pointer.
    ///
    /// May or may not be null for empty slices: don't rely on it.
    data: *mut T,

    /// Number of entries in the slice.
    len: usize,

    /// Make compiler print warnings if `T` isn't FFI-safe.
    _marker: PhantomData<T>,
}

impl<T> From<Vec<T>> for SymblibSlice<T> {
    fn from(vec: Vec<T>) -> Self {
        let mut s = vec.into_boxed_slice();
        let data = s.as_mut_ptr();
        let len = s.len();
        mem::forget(s);

        Self {
            data,
            len,
            _marker: PhantomData,
        }
    }
}

impl<T> From<SymblibSlice<T>> for Box<[T]> {
    fn from(s: SymblibSlice<T>) -> Self {
        unsafe {
            let std_slice = slice::from_raw_parts_mut(s.data, s.len);
            mem::forget(s);
            Box::<[T]>::from_raw(std_slice)
        }
    }
}

impl<T> From<SymblibSlice<T>> for Vec<T> {
    fn from(s: SymblibSlice<T>) -> Self {
        Vec::from(Box::<[T]>::from(s))
    }
}

impl<T> Drop for SymblibSlice<T> {
    fn drop(&mut self) {
        // Drop by converting to boxed slice and then dropping the slice.
        drop(Box::<[T]>::from(unsafe { ptr::read(self) }));
    }
}

unsafe impl<T: Send> Send for SymblibSlice<T> {}

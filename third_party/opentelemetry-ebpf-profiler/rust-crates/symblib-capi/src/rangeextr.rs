// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

use super::{FfiResult, StatusCode, SymblibSlice, SymblibString};
use std::ffi::{c_int, c_void, OsString};
use std::fs;
use std::os::unix::ffi::OsStringExt as _;
use std::path::{Path, PathBuf};
use symblib::objfile::{self, SymbolSource};
use symblib::symbconv::RangeExtractor as _;
use symblib::{dwarf, symbconv as sc, symbconv, symbfile};
use std::os::fd::FromRawFd;

/// Extract ranges from an executable.
///
/// This creates a [`symblib::symbconv::multi`] extractor with all supported
/// debug symbol formats registered with the following priority:
///
/// 1) DWARF
/// 2) Go symbols
/// 3) ELF debug symbols
/// 4) ELF dynamic symbols
///
/// This extractor is then run to completion and the visitor is invoked for
/// every range found in the executable. The user_data pointer is passed to
/// the visitor untouched and may be NULL.
#[no_mangle]
pub unsafe extern "C" fn symblib_rangeextr(
    executable_fd: c_int,
    _dwarf_sup_fd: c_int,
    visitor: SymblibRangeVisitor,
    user_data: *mut c_void,
) -> StatusCode {

    let visitor: symbconv::RangeVisitor = &mut |rng|  {
        let ffi_rng = SymblibRange::from(rng);
        match visitor(user_data, &ffi_rng) {
            StatusCode::Ok => Ok(()),
            code => Err(Box::new(code)),
        }
    };
    let executable =  fs::File::from_raw_fd(executable_fd);

    let sup = &None;
    let res = match rangeextr_impl(&executable, sup, visitor) {
        Ok(()) => StatusCode::Ok,
        Err(e) => e,
    };
    std::mem::forget(executable);
    res
}

pub fn rangeextr_impl(
    executable: &fs::File,
    dwarf_sup: &Option<fs::File>,
    visitor: symbconv::RangeVisitor,
) -> FfiResult<()> {

    // Open and mmap main object file.
    let obj = objfile::File::load_file(executable)?;
    let obj_reader = obj.parse()?;

    // Load DWARF sections.
    let mut dw = dwarf::Sections::load(&obj_reader)?;

    // If a supplementary path was found, load its data.
    let sup_obj;
    let sup_reader;

    if let Some(dwarf_sup) = dwarf_sup {
        sup_obj = objfile::File::load_file(dwarf_sup)?;
        sup_reader = sup_obj.parse()?;
        dw.load_sup(&sup_reader)?;
    }

    let mut extr = sc::multi::Extractor::new(&obj_reader)?;

    extr.add("dwarf", sc::dwarf::Extractor::new(&dw));
    extr.add("go", sc::go::Extractor::new(&obj_reader));
    extr.add(
        "dbg-obj-sym",
        sc::obj::Extractor::new(&obj_reader, SymbolSource::Debug),
    );
    extr.add(
        "dyn-obj-sym",
        sc::obj::Extractor::new(&obj_reader, SymbolSource::Dynamic),
    );

    // Run the extractor with the user's callback.

    let result = extr.extract(visitor);
    // Extract the error code from the visitor error branches.
    match result {
        Ok(_) => Ok(()),
        Err(
            sc::Error::Dwarf(sc::dwarf::Error::Visitor(v))
            | sc::Error::Go(sc::go::Error::Visitor(v))
            | sc::Error::Obj(v),
        ) => Err(v
            .downcast::<StatusCode>()
            .map(|x| *x)
            .unwrap_or(StatusCode::Symbconv)),
        Err(_) => Err(StatusCode::Symbconv),
    }
}

fn _resolve_alt_link(exec_path: &Path, obj: &objfile::Reader) -> FfiResult<Option<PathBuf>> {
    let alt_link = obj.gnu_debug_alt_link()?;

    let Some(alt_link) = alt_link else {
        return Ok(None);
    };

    // Turn array of bytes into a proper path.
    let alt_path = OsString::from_vec(alt_link.path);
    let alt_path = PathBuf::from(alt_path);

    if alt_path.is_absolute() {
        return Ok(Some(alt_path));
    }

    Ok(Some(
        exec_path
            .canonicalize()?
            .parent()
            .expect("absolute file path should always have a parent")
            .join(&alt_path),
    ))
}

/// Visitor callback for extracted ranges.
///
/// The range is **borrowed** to the callee and the pointer is only valid for
/// the duration of the visitor call. Returning an error will abort further
/// execution and return early.
pub type SymblibRangeVisitor =
    unsafe extern "C" fn(user_data: *mut c_void, range: *const SymblibRange) -> StatusCode;

/// FFI-safe variant of [`symbfile::Range`].
#[repr(C)]
#[derive(Debug)]
pub struct SymblibRange {
    pub elf_va: u64,
    pub length: u32,
    pub func: SymblibString,      // never null
    pub file: SymblibString,      // may be null
    pub call_file: SymblibString, // may be null
    pub call_line: u32,           // 0 = unknown
    pub depth: u32,
    pub line_table: SymblibSlice<SymblibLineTableEntry>,

    // pub(crate) rust_range: Box<symbfile::Range>,
}

impl From<symbfile::Range> for SymblibRange {
    fn from(rng: symbfile::Range) -> Self {
        // let rust_range = Box::new(rng.clone());
        let table: Vec<SymblibLineTableEntry> =
            rng.line_table.into_iter().map(Into::into).collect();

        Self {
            elf_va: rng.elf_va,
            length: rng.length,
            func: rng.func.into(),
            file: rng.file.into(),
            call_file: rng.call_file.into(),
            call_line: rng.call_line.unwrap_or(0),
            depth: rng.depth,
            line_table: SymblibSlice::from(table),
            // rust_range,
        }
    }
}

/// FFI-safe variant of [`symbfile::LineTableEntry`].
#[repr(C)]
#[derive(Debug)]
pub struct SymblibLineTableEntry {
    pub offset: u32,
    pub line_number: u32,
}

impl From<symbfile::LineTableEntry> for SymblibLineTableEntry {
    fn from(entry: symbfile::LineTableEntry) -> Self {
        Self {
            offset: entry.offset,
            line_number: entry.line_number,
        }
    }
}

#[cfg(test)]
mod tests {
    use std::fs::OpenOptions;
    use std::os::fd::{AsFd, AsRawFd};
    use super::*;
    use std::ptr;

    #[test]
    fn rangeextr() {
        let file = "../symblib/testdata/inline";

        extern "C" fn visitor(_: *mut c_void, rng: *const SymblibRange) -> StatusCode {
            assert_ne!(rng, ptr::null());
            dbg!(unsafe { &*rng });
            StatusCode::Ok
        }
        let file = OpenOptions::new().read(true).open(file).unwrap();

        assert_eq!(
            unsafe {
                let fd = file.as_raw_fd();
                symblib_rangeextr(fd as c_int, -1, visitor, ptr::null_mut()) },
            StatusCode::Ok
        );
    }
}

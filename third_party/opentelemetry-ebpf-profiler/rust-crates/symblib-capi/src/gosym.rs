// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

use crate::{FfiResult, StatusCode, SymblibPointResolver};
use std::ffi::{c_char, CStr};
use std::mem;
use std::path::Path;
use symblib::symbconv::{PointResolver, ResolvedSymbol, Result as SymconvResult};
use symblib::{gosym::GoRuntimeInfo, objfile};

pub struct SymblibGoRuntime {
    #[allow(unused)]
    obj: Box<objfile::File>,
    runtime: GoRuntimeInfo<'static>,
}

impl PointResolver for SymblibGoRuntime {
    fn symbols_for_pc(&self, pc: symblib::VirtAddr) -> SymconvResult<Vec<ResolvedSymbol>> {
        self.runtime.symbols_for_pc(pc)
    }
}

#[no_mangle]
pub unsafe extern "C" fn symblib_goruntime_new(
    executable: *const c_char,
    runtime: *mut *mut SymblibPointResolver,
) -> StatusCode {
    match goruntime_new_impl(executable, runtime) {
        Ok(()) => StatusCode::Ok,
        Err(e) => e,
    }
}

unsafe fn goruntime_new_impl(
    executable: *const c_char,
    runtime: *mut *mut SymblibPointResolver,
) -> FfiResult<()> {
    let executable = CStr::from_ptr(executable)
        .to_str()
        .map(Path::new)
        .map_err(|_| StatusCode::BadUtf8)?;

    let obj = Box::new(objfile::File::load(executable)?);
    let obj_reader = obj.parse()?;
    let go_runtime = GoRuntimeInfo::open(&obj_reader)?;

    // Transmute away lifetime to allow for self-referential struct.
    let go_runtime: GoRuntimeInfo<'static> = mem::transmute(go_runtime);

    let resolver = SymblibGoRuntime {
        obj,
        runtime: go_runtime,
    };

    let point_resolver = Box::new(SymblibPointResolver::new(
        Box::new(resolver) as Box<dyn PointResolver + Send>
    ));
    *runtime = Box::into_raw(point_resolver);
    Ok(())
}

#[no_mangle]
pub unsafe extern "C" fn symblib_goruntime_free(runtime: *mut SymblibPointResolver) {
    if !runtime.is_null() {
        drop(Box::from_raw(runtime));
    }
}

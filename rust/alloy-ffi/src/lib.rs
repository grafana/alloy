use std::ffi::{CStr, c_char, c_void};
use std::sync::{Mutex, OnceLock};

use alloy_core::{Context, DynSource, Error, Output, OutputReceiver, Registry, output};

pub const ALLOY_OK: i32 = 0;
pub const ALLOY_CLOSED: i32 = 1;

/// Every Rust component, as linked into Alloy.
pub fn registry() -> &'static Registry {
    static REGISTRY: OnceLock<Registry> = OnceLock::new();
    REGISTRY.get_or_init(|| {
        let mut r = Registry::new();
        alloy_loki_source::register(&mut r);
        r
    })
}

#[repr(C)]
pub struct Buf {
    ptr: *mut u8,
    len: usize,
    cap: usize,
}

impl Buf {
    fn from_vec(v: Vec<u8>) -> Self {
        let mut v = std::mem::ManuallyDrop::new(v);
        Self {
            ptr: v.as_mut_ptr(),
            len: v.len(),
            cap: v.capacity(),
        }
    }
}

pub struct AlloyComponent {
    ctx: Context,
    source: Mutex<Box<dyn DynSource>>,
    out: Mutex<Option<Output>>,
    rx: Mutex<OutputReceiver>,
}

fn build(name: &str, args: &[u8]) -> Result<AlloyComponent, Error> {
    let source = registry().build_source(name, args)?;
    let (out, rx) = output();
    Ok(AlloyComponent {
        ctx: Context::new(),
        source: Mutex::new(source),
        out: Mutex::new(Some(out)),
        rx: Mutex::new(rx),
    })
}

/// Returns null and fills `err` on failure; free it with `alloy_buf_free`.
///
/// # Safety
/// `name` must be a valid C string, `args` must point to `args_size`
/// readable bytes and `err` must be valid for writes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn alloy_component_new(
    name: *const c_char,
    args: *const c_void,
    args_size: usize,
    err: *mut Buf,
) -> *mut AlloyComponent {
    let args = unsafe { std::slice::from_raw_parts(args as *const u8, args_size) };
    let res = match unsafe { CStr::from_ptr(name) }.to_str() {
        Ok(name) => build(name, args),
        Err(e) => Err(e.into()),
    };

    match res {
        Ok(c) => Box::into_raw(Box::new(c)),
        Err(e) => {
            unsafe { *err = Buf::from_vec(e.to_string().into_bytes()) };
            std::ptr::null_mut()
        }
    }
}

/// Runs the source until `alloy_component_close` is called. Only the first
/// call runs it.
///
/// # Safety
/// `c` must come from `alloy_component_new` and not be freed.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn alloy_source_run(c: *const AlloyComponent) {
    let c = unsafe { &*c };
    let Some(out) = c.out.lock().unwrap().take() else {
        return;
    };
    c.source.lock().unwrap().run(&c.ctx, &out);
}

/// Blocks until the next batch and fills `out`. Returns `ALLOY_CLOSED`
/// without touching `out` once the source has stopped and every batch has
/// been received.
///
/// # Safety
/// `c` must come from `alloy_component_new` and not be freed. `out` must be
/// valid for writes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn alloy_source_recv(c: *const AlloyComponent, out: *mut Buf) -> i32 {
    let c = unsafe { &*c };
    match c.rx.lock().unwrap().recv() {
        Ok(batch) => {
            unsafe { *out = Buf::from_vec(batch.into_bytes()) };
            ALLOY_OK
        }
        Err(_) => ALLOY_CLOSED,
    }
}

/// Tells the component to stop. Safe to call concurrently with the other
/// functions.
///
/// # Safety
/// `c` must come from `alloy_component_new` and not be freed.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn alloy_component_close(c: *const AlloyComponent) {
    unsafe { &*c }.ctx.close();
}

/// # Safety
/// `c` must come from `alloy_component_new`, and no other call on `c` may
/// be running.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn alloy_component_free(c: *mut AlloyComponent) {
    drop(unsafe { Box::from_raw(c) });
}

/// # Safety
/// `buf` must have been filled by this library and not freed before.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn alloy_buf_free(buf: *mut Buf) {
    let buf = unsafe { &mut *buf };
    drop(unsafe { Vec::from_raw_parts(buf.ptr, buf.len, buf.cap) });
    buf.ptr = std::ptr::null_mut();
    buf.len = 0;
    buf.cap = 0;
}

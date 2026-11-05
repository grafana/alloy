use std::sync::{Condvar, Mutex};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

pub struct Gen {
    lines_per_batch: u32,
    interval: Duration,
    state: Mutex<State>,
    cv: Condvar,
}

struct State {
    closed: bool,
    counter: u64,
}

#[repr(C)]
pub struct Buf {
    ptr: *mut u8,
    len: usize,
    cap: usize,
}

pub const RG_OK: i32 = 0;
pub const RG_CLOSED: i32 = 1;

#[unsafe(no_mangle)]
pub extern "C" fn rg_new(lines_per_batch: u32, interval_ms: u64) -> *mut Gen {
    let g = Gen {
        lines_per_batch,
        interval: Duration::from_millis(interval_ms),
        state: Mutex::new(State {
            closed: false,
            counter: 0,
        }),
        cv: Condvar::new(),
    };
    Box::into_raw(Box::new(g))
}

/// Blocks for the interval, then fills `out` with one batch. Returns
/// `RG_CLOSED` without touching `out` once `rg_close` has been called.
///
/// # Safety
/// `g` must come from `rg_new` and not be freed. `out` must be valid for writes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn rg_next_batch(g: *const Gen, out: *mut Buf) -> i32 {
    let g = unsafe { &*g };

    let state = g.state.lock().unwrap();
    let (mut state, _) = g
        .cv
        .wait_timeout_while(state, g.interval, |s| !s.closed)
        .unwrap();
    if state.closed {
        return RG_CLOSED;
    }

    let ts = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos() as i64;

    // Each entry: i64 LE unix nanos, u32 LE line length, line bytes.
    let mut buf = Vec::new();
    for _ in 0..g.lines_per_batch {
        let line = format!("rustgen line {}", state.counter);
        state.counter += 1;
        buf.extend_from_slice(&ts.to_le_bytes());
        buf.extend_from_slice(&(line.len() as u32).to_le_bytes());
        buf.extend_from_slice(line.as_bytes());
    }

    let mut buf = std::mem::ManuallyDrop::new(buf);
    unsafe {
        *out = Buf {
            ptr: buf.as_mut_ptr(),
            len: buf.len(),
            cap: buf.capacity(),
        };
    }
    RG_OK
}

/// # Safety
/// `buf` must have been filled by `rg_next_batch` and not freed before.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn rg_buf_free(buf: *mut Buf) {
    let buf = unsafe { &mut *buf };
    drop(unsafe { Vec::from_raw_parts(buf.ptr, buf.len, buf.cap) });
    buf.ptr = std::ptr::null_mut();
    buf.len = 0;
    buf.cap = 0;
}

/// Unblocks a pending `rg_next_batch`; later calls return `RG_CLOSED`.
/// Safe to call concurrently with `rg_next_batch`.
///
/// # Safety
/// `g` must come from `rg_new` and not be freed.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn rg_close(g: *const Gen) {
    let g = unsafe { &*g };
    g.state.lock().unwrap().closed = true;
    g.cv.notify_all();
}

/// # Safety
/// `g` must come from `rg_new`, and no other call on `g` may be running.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn rg_free(g: *mut Gen) {
    drop(unsafe { Box::from_raw(g) });
}

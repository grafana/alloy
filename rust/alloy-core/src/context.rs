use std::sync::{Arc, Condvar, Mutex};
use std::time::Duration;

/// Tells a running component when it has to stop. Cheap to clone and share
/// between threads.
#[derive(Clone, Default)]
pub struct Context {
    inner: Arc<Inner>,
}

#[derive(Default)]
struct Inner {
    closed: Mutex<bool>,
    cv: Condvar,
}

impl Context {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn is_closed(&self) -> bool {
        *self.inner.closed.lock().unwrap()
    }

    /// Sleeps for `d` or until the context is closed. Returns false if it
    /// was closed.
    pub fn sleep(&self, d: Duration) -> bool {
        let closed = self.inner.closed.lock().unwrap();
        let (closed, _) = self
            .inner
            .cv
            .wait_timeout_while(closed, d, |closed| !*closed)
            .unwrap();
        !*closed
    }

    pub fn close(&self) {
        *self.inner.closed.lock().unwrap() = true;
        self.inner.cv.notify_all();
    }
}

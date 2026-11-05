use std::sync::mpsc::{Receiver, RecvError, SendError, SyncSender, sync_channel};

use crate::{Batch, Component, Context};

/// A component that produces batches.
pub trait Source: Component {
    /// Runs until `ctx` is closed, sending batches to `out`. `out` can be
    /// cloned to send from several threads; all clones must be dropped
    /// before `run` returns.
    fn run(&mut self, ctx: &Context, out: &Output);
}

/// Where a source sends its batches. `send` blocks while the receiving end
/// is busy.
#[derive(Clone)]
pub struct Output {
    tx: SyncSender<Batch>,
}

#[derive(Debug)]
pub struct Closed;

impl Output {
    /// Fails once the receiving end is gone.
    pub fn send(&self, batch: Batch) -> Result<(), Closed> {
        self.tx.send(batch).map_err(|SendError(_)| Closed)
    }
}

/// The receiving end of an [`Output`].
pub struct OutputReceiver {
    rx: Receiver<Batch>,
}

impl OutputReceiver {
    /// Blocks until the next batch. Fails once every `Output` is dropped
    /// and all batches are received.
    pub fn recv(&self) -> Result<Batch, Closed> {
        self.rx.recv().map_err(|RecvError| Closed)
    }
}

pub fn output() -> (Output, OutputReceiver) {
    let (tx, rx) = sync_channel(1);
    (Output { tx }, OutputReceiver { rx })
}

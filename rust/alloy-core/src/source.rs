use std::sync::mpsc::{Receiver, RecvError, SendError, SyncSender, sync_channel};

use crate::{Component, Context, Signal};

/// A component that produces batches of signal `S`.
pub trait Source<S: Signal>: Component {
    /// Runs until `ctx` is closed, sending batches to `out`. `out` can be
    /// cloned to send from several threads; all clones must be dropped
    /// before `run` returns.
    fn run(&mut self, ctx: &Context, out: &Output<S>);
}

/// Where a source sends its batches. `send` blocks while the receiving end
/// is busy.
pub struct Output<S: Signal> {
    tx: SyncSender<S::Batch<'static>>,
}

impl<S: Signal> Clone for Output<S> {
    fn clone(&self) -> Self {
        Self {
            tx: self.tx.clone(),
        }
    }
}

#[derive(Debug)]
pub struct Closed;

impl<S: Signal> Output<S> {
    /// Fails once the receiving end is gone.
    pub fn send(&self, batch: S::Batch<'static>) -> Result<(), Closed> {
        self.tx.send(batch).map_err(|SendError(_)| Closed)
    }
}

/// The receiving end of an [`Output`].
pub struct OutputReceiver<S: Signal> {
    rx: Receiver<S::Batch<'static>>,
}

impl<S: Signal> OutputReceiver<S> {
    /// Blocks until the next batch. Fails once every `Output` is dropped
    /// and all batches are received.
    pub fn recv(&self) -> Result<S::Batch<'static>, Closed> {
        self.rx.recv().map_err(|RecvError| Closed)
    }
}

pub fn output<S: Signal>() -> (Output<S>, OutputReceiver<S>) {
    let (tx, rx) = sync_channel(1);
    (Output { tx }, OutputReceiver { rx })
}

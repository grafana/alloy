use crate::{Component, Error, Signal};

/// A component that consumes batches of signal `S` and forwards nothing.
/// `consume` is called concurrently, so the component must be `Sync`.
pub trait Sink<S: Signal>: Component + Sync {
    fn consume(&self, batch: &S::Batch<'_>) -> Result<(), Error>;
}

use crate::logs::LogBatch;
use crate::{Component, Error};

/// A component that consumes batches and forwards nothing. `consume` is called
/// concurrently, so the component must be `Sync`.
pub trait Sink: Component + Sync {
    fn consume(&self, batch: &LogBatch<'_>) -> Result<(), Error>;
}

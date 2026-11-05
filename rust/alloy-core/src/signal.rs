use crate::Error;
use crate::schema::SignalKind;

/// A kind of telemetry data and the batch type it moves in. Components pick
/// their signal with the type parameter of `Source` or `Sink`, so one type can
/// handle several signals.
pub trait Signal: 'static {
    /// Borrows from the buffer when read from Go; built in Rust as
    /// `Batch<'static>`.
    type Batch<'a>: Send;

    const KIND: SignalKind;

    fn encode(batch: &Self::Batch<'_>) -> Vec<u8>;

    fn read(buf: &[u8]) -> Result<Self::Batch<'_>, Error>;
}

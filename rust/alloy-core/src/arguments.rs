use serde::de::DeserializeOwned;

use crate::schema::Field;

/// Arguments of a component, decoded from CBOR that Go encodes from the
/// generated Go arguments. Go has already applied defaults and validated
/// them. Implement with `#[alloy_core::arguments]`.
pub trait Arguments: DeserializeOwned + Send + 'static {
    fn schema() -> &'static [Field];
}

#[doc(hidden)]
pub mod __private {
    use std::time::Duration;

    use serde::{Deserialize, Deserializer};

    /// Go encodes `time.Duration` as integer nanoseconds.
    pub fn duration_nanos<'de, D: Deserializer<'de>>(d: D) -> Result<Duration, D::Error> {
        Ok(Duration::from_nanos(u64::deserialize(d)?))
    }
}

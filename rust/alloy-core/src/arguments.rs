use crate::Error;

/// Arguments of a component. Go has already applied defaults and validated
/// them, and passes them as `Raw`.
pub trait Arguments: Sized + Send + 'static {
    /// The struct Go fills in. Must be `#[repr(C)]`, only contain FFI-safe
    /// fields, and match its C declaration in the header.
    type Raw: Copy;

    fn from_raw(raw: &Self::Raw) -> Result<Self, Error>;
}

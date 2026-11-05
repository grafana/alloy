use crate::schema::Stability;
use crate::{Arguments, Error};

/// Name and stability of a component. Implement with
/// `#[alloy_core::component(name = "...", stability = ...)]`.
pub trait ComponentMeta {
    /// The component's name in the Alloy config, e.g. `loki.source.file`.
    const NAME: &'static str;
    const STABILITY: Stability;
}

/// Common to every component kind.
pub trait Component: ComponentMeta + Send + 'static {
    type Args: Arguments;

    fn new(args: Self::Args) -> Result<Self, Error>
    where
        Self: Sized;
}

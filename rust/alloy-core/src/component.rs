use crate::{Arguments, Error};

/// Common to every component kind.
pub trait Component: Send + 'static {
    type Args: Arguments;

    fn new(args: Self::Args) -> Result<Self, Error>
    where
        Self: Sized;
}

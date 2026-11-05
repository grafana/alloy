mod arguments;
mod batch;
mod component;
mod context;
mod registry;
pub mod schema;
mod source;

pub use alloy_core_macros::{arguments, component};
pub use arguments::Arguments;
pub use batch::Batch;
pub use component::{Component, ComponentMeta};
pub use context::Context;
pub use registry::{DynSource, Registry};
pub use source::{Closed, Output, OutputReceiver, Source, output};

pub type Error = Box<dyn std::error::Error + Send + Sync>;

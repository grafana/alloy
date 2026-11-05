mod arguments;
mod component;
mod context;
pub mod logs;
mod registry;
pub mod schema;
mod sink;
mod source;

pub use alloy_core_macros::{arguments, component};
pub use arguments::Arguments;
pub use component::{Component, ComponentMeta};
pub use context::Context;
pub use registry::{DynSink, DynSource, Instance, Registry};
pub use sink::Sink;
pub use source::{Closed, Output, OutputReceiver, Source, output};

pub type Error = Box<dyn std::error::Error + Send + Sync>;

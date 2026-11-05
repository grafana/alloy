use std::collections::HashMap;

use crate::logs::LogBatch;
use crate::schema::{ComponentInfo, Kind};
use crate::{Arguments, Component, Context, Error, Output, Sink, Source};

/// Object-safe view of a [`Source`], which can't be a trait object itself
/// because of [`Component`]'s associated type and constructor.
pub trait DynSource: Send {
    fn run(&mut self, ctx: &Context, out: &Output);
}

impl<S: Source> DynSource for S {
    fn run(&mut self, ctx: &Context, out: &Output) {
        Source::run(self, ctx, out)
    }
}

/// Object-safe view of a [`Sink`].
pub trait DynSink: Send + Sync {
    fn consume(&self, batch: &LogBatch<'_>) -> Result<(), Error>;
}

impl<S: Sink> DynSink for S {
    fn consume(&self, batch: &LogBatch<'_>) -> Result<(), Error> {
        Sink::consume(self, batch)
    }
}

/// A component built by the registry.
pub enum Instance {
    Source(Box<dyn DynSource>),
    Sink(Box<dyn DynSink>),
}

type Builder = fn(&[u8]) -> Result<Instance, Error>;

struct Entry {
    info: ComponentInfo,
    build: Builder,
}

/// Maps component names to their constructors and descriptions.
#[derive(Default)]
pub struct Registry {
    components: HashMap<&'static str, Entry>,
}

impl Registry {
    pub fn new() -> Self {
        Self::default()
    }

    /// Panics if a component with the same name is already registered.
    pub fn register_source<S: Source>(&mut self) {
        self.register::<S>(Kind::Source, |args| {
            Ok(Instance::Source(Box::new(build::<S>(args)?)))
        });
    }

    /// Panics if a component with the same name is already registered.
    pub fn register_sink<S: Sink>(&mut self) {
        self.register::<S>(Kind::Sink, |args| Ok(Instance::Sink(Box::new(build::<S>(args)?))));
    }

    fn register<C: Component>(&mut self, kind: Kind, build: Builder) {
        let entry = Entry {
            info: ComponentInfo {
                name: C::NAME,
                kind,
                stability: C::STABILITY,
                args: C::Args::schema(),
            },
            build,
        };
        if self.components.insert(C::NAME, entry).is_some() {
            panic!("component {} registered twice", C::NAME);
        }
    }

    pub fn components(&self) -> impl Iterator<Item = &ComponentInfo> {
        self.components.values().map(|e| &e.info)
    }

    /// Builds the component registered as `name` from its `Raw` arguments.
    pub fn build(&self, name: &str, args: &[u8]) -> Result<Instance, Error> {
        match self.components.get(name) {
            Some(e) => (e.build)(args),
            None => Err(format!("unknown component {name}").into()),
        }
    }
}

fn build<C: Component>(args: &[u8]) -> Result<C, Error> {
    type Raw<C> = <<C as Component>::Args as Arguments>::Raw;

    let want = size_of::<Raw<C>>();
    if args.len() != want {
        return Err(format!("arguments are {} bytes, expected {want}", args.len()).into());
    }

    // SAFETY: the length matches, and Arguments requires Raw to be the C
    // struct Go encodes.
    let raw = unsafe { (args.as_ptr() as *const Raw<C>).read_unaligned() };
    C::new(C::Args::from_raw(&raw)?)
}

use std::collections::HashMap;

use crate::schema::{ComponentInfo, Kind};
use crate::{Arguments, Component, Context, Error, Output, Source};

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

type SourceBuilder = fn(&[u8]) -> Result<Box<dyn DynSource>, Error>;

struct Entry {
    info: ComponentInfo,
    build: SourceBuilder,
}

/// Maps component names to their constructors and descriptions.
#[derive(Default)]
pub struct Registry {
    sources: HashMap<&'static str, Entry>,
}

impl Registry {
    pub fn new() -> Self {
        Self::default()
    }

    /// Panics if a component with the same name is already registered.
    pub fn register_source<S: Source>(&mut self) {
        let name = S::NAME;
        let entry = Entry {
            info: ComponentInfo {
                name,
                kind: Kind::Source,
                stability: S::STABILITY,
                args: S::Args::schema(),
            },
            build: build_source::<S>,
        };
        if self.sources.insert(name, entry).is_some() {
            panic!("component {name} registered twice");
        }
    }

    pub fn components(&self) -> impl Iterator<Item = &ComponentInfo> {
        self.sources.values().map(|e| &e.info)
    }

    /// Builds the source registered as `name` from its `Raw` arguments.
    pub fn build_source(&self, name: &str, args: &[u8]) -> Result<Box<dyn DynSource>, Error> {
        match self.sources.get(name) {
            Some(e) => (e.build)(args),
            None => Err(format!("unknown component {name}").into()),
        }
    }
}

fn build_source<S: Source>(args: &[u8]) -> Result<Box<dyn DynSource>, Error> {
    type Raw<S> = <<S as Component>::Args as Arguments>::Raw;

    let want = size_of::<Raw<S>>();
    if args.len() != want {
        return Err(format!("arguments are {} bytes, expected {want}", args.len()).into());
    }

    // SAFETY: the length matches, and Arguments requires Raw to be the C
    // struct Go encodes.
    let raw = unsafe { (args.as_ptr() as *const Raw<S>).read_unaligned() };
    Ok(Box::new(S::new(S::Args::from_raw(&raw)?)?))
}

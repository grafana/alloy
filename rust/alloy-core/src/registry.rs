use std::collections::HashMap;
use std::marker::PhantomData;
use std::sync::Mutex;

use crate::schema::{ComponentInfo, Kind};
use crate::{
    Arguments, Component, Context, Error, Output, OutputReceiver, Signal, Sink, Source, output,
};

/// A source built by the registry, with batches encoded for Go.
pub trait DynSource: Send + Sync {
    /// Runs the source until `ctx` is closed. Only the first call runs it.
    fn run(&self, ctx: &Context);

    /// Blocks until the next encoded batch. `None` once the source has
    /// stopped and every batch has been received.
    fn recv(&self) -> Option<Vec<u8>>;
}

/// A sink built by the registry, taking batches encoded by Go.
pub trait DynSink: Send + Sync {
    fn consume(&self, buf: &[u8]) -> Result<(), Error>;
}

/// A component built by the registry.
pub enum Instance {
    Source(Box<dyn DynSource>),
    Sink(Box<dyn DynSink>),
}

struct SourceInstance<S: Signal, C> {
    source: Mutex<C>,
    out: Mutex<Option<Output<S>>>,
    rx: Mutex<OutputReceiver<S>>,
}

impl<S: Signal, C: Source<S>> DynSource for SourceInstance<S, C> {
    fn run(&self, ctx: &Context) {
        let Some(out) = self.out.lock().unwrap().take() else {
            return;
        };
        self.source.lock().unwrap().run(ctx, &out);
    }

    fn recv(&self) -> Option<Vec<u8>> {
        let batch = self.rx.lock().unwrap().recv().ok()?;
        Some(S::encode(&batch))
    }
}

// fn() -> S keeps the instance Send + Sync whatever S is.
struct SinkInstance<S: Signal, C> {
    sink: C,
    signal: PhantomData<fn() -> S>,
}

impl<S: Signal, C: Sink<S>> DynSink for SinkInstance<S, C> {
    fn consume(&self, buf: &[u8]) -> Result<(), Error> {
        self.sink.consume(&S::read(buf)?)
    }
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

    /// Registers `C` as a source of signal `S`. Panics if a component with the
    /// same name is already registered.
    pub fn register_source<S: Signal, C: Source<S>>(&mut self) {
        self.register::<S, C>(Kind::Source, |args| {
            let (out, rx) = output::<S>();
            Ok(Instance::Source(Box::new(SourceInstance {
                source: Mutex::new(build::<C>(args)?),
                out: Mutex::new(Some(out)),
                rx: Mutex::new(rx),
            })))
        });
    }

    /// Registers `C` as a sink of signal `S`. Panics if a component with the
    /// same name is already registered.
    pub fn register_sink<S: Signal, C: Sink<S>>(&mut self) {
        self.register::<S, C>(Kind::Sink, |args| {
            Ok(Instance::Sink(Box::new(SinkInstance {
                sink: build::<C>(args)?,
                signal: PhantomData::<fn() -> S>,
            })))
        });
    }

    fn register<S: Signal, C: Component>(&mut self, kind: Kind, build: Builder) {
        let entry = Entry {
            info: ComponentInfo {
                name: C::NAME,
                kind,
                signal: S::KIND,
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

    /// Builds the component registered as `name` from its CBOR arguments.
    pub fn build(&self, name: &str, args: &[u8]) -> Result<Instance, Error> {
        match self.components.get(name) {
            Some(e) => (e.build)(args),
            None => Err(format!("unknown component {name}").into()),
        }
    }
}

fn build<C: Component>(mut args: &[u8]) -> Result<C, Error> {
    let decoded: C::Args = ciborium::from_reader(&mut args).map_err(|e| match e {
        ciborium::de::Error::Semantic(_, msg) => format!("decoding arguments: {msg}"),
        e => format!("decoding arguments: {e:?}"),
    })?;
    if !args.is_empty() {
        return Err(format!("{} trailing bytes after arguments", args.len()).into());
    }
    C::new(decoded)
}

# Rewrite it in Rust: experiment notes

Exploration only. Any real migration would have to be incremental: replace
parts of Alloy with Rust step by step, not a big-bang rewrite.

## Goal

Open: performance, memory footprint, safety, or learning? Drives which
approach to try first.

## Constraints

- Alloy builds on large Go ecosystems: OTel Collector (core + contrib),
  Prometheus (scrape, remote_write, WAL), Loki clients. Rust has no
  equivalent of the collector ecosystem, so a realistic end state is a
  hybrid, not a full port.
- Every shipped platform needs a Rust target in the build (see the zig
  cross-compile experiment).
- `CGO_ENABLED=1` is already the default.
- All Rust code must link into Go as a single staticlib. Multiple
  staticlibs would each bundle std and an allocator, clash on symbols and
  couldn't share types. So component crates are plain `lib` crates and one
  aggregate crate exposes the C API.

## Approaches

### 1. Rust libraries over cgo (FFI)

Rust `staticlib` with a C ABI, linked into the Go binary.

- Targets: CPU-heavy leaf code with narrow interfaces. `loki.process`
  parsing stages, WAL encode/decode, relabeling, compression.
- cgo calls cost roughly tens of ns plus marshalling, so interfaces must be
  batch-oriented.

### 2. Go control plane, Rust data plane

Go keeps config language, component graph, UI, reloads. Some components get
a Go shell that hands config to a Rust engine; data flows inside Rust.

- Example: a full logs path (`loki.source.file` -> `loki.process` ->
  `loki.write`) in Rust when every component in the chain has a Rust
  implementation.
- Data crosses the boundary only at the edges.
- Hard part: mixed Go/Rust chains need a shared batch format (Arrow, flat
  buffer). The loki pipeline v2 interface is a candidate seam.

### 3. Out-of-process Rust components

Rust sidecar/subprocess over OTLP/gRPC, Unix socket or shared memory; a Go
proxy component manages lifecycle.

- No cgo, crash isolation, independent builds.
- Serialization cost, more moving parts, harder debugging and health
  reporting.

### 4. WASM plugins

Rust compiled to WASM, run in Go via wazero (pure Go, no cgo).

- Fits processing stages and user-defined transforms; doubles as a plugin
  system.
- Slower than native, copies in/out, poor fit for I/O-heavy components.

### 5. Rewrite self-contained modules with differential testing

Start with `syntax/` (parser, formatter, later evaluator). Compare Rust and
Go output on the same inputs (fuzzing, existing test corpus).

- Could ship first as tooling (formatter, LSP) before touching the runtime.
- Low risk, good for learning, no runtime payoff on its own.

### 6. Invert the host

Rust owns `main` and the runtime, embeds remaining Go as a `c-archive`.

- Most disruptive; only one Go runtime per process. Endgame, not a start.

## Current leaning

Option 2 on a narrow logs pipeline (cgo, batch interface): answers whether
it is faster/leaner end to end and surfaces FFI/build pain early. Option 5
as a low-stakes warm-up if learning comes first.

## Decisions

- Approach 2 (Go control plane, Rust data plane).
- First step: a Rust-based loki source feeding the existing Go loki
  pipeline (v2), i.e. a mixed Rust -> Go chain. The source is a toy
  generator (`loki.source.rustgen`), then a toy sink (`loki.rustecho`).
- Rust -> Go edges: Go pulls. A Go goroutine drains a blocking Rust
  `recv`; no Rust -> Go callbacks.
- Rust -> Rust edges link directly inside Rust, no FFI per batch (not
  built yet). Go keeps the graph: a Rust component's exported receiver
  carries an opaque handle to its Rust consumer. On `forward_to` updates
  the Go shell splits children into Rust handles (Rust-side fanout,
  called directly) and Go consumers (pull path). Needs a Rust mirror of
  `FanoutV2` (incl. `ErrConsumerStopped` on reload) and ref-counted
  component lifetimes.
- Sources push: `run(&mut self, ctx, out)` sends into a cloneable
  `Output`, so multi-producer sources (e.g. prometheus.scrape) fit.
- Only add component kinds when needed. `Component` (args, `new`,
  name/stability via `ComponentMeta`) is the supertrait; kinds so far:
  `Source`, `Sink` (processor parked). Discovery/exporters will need
  `Exports`.
- Kinds are generic over the signal: `Source<S: Signal>`, `Sink<S:
  Signal>`. A generic parameter (not an associated type) so one type can
  implement several signals, as `otelcol.*` components do. `Signal {
  type Batch<'a>; const KIND; encode; read }`; only `Logs` exists.
  Prometheus later: `MetricBatch` (series with samples, native
  histograms, exemplars, metadata); Go adapts `storage.Appender`
  (buffer appends, `Commit` = consume OK, `Rollback` = discard);
  `SeriesRef` caching stays on each side.
- Layout: cargo workspace in repo-root `rust/`, one staticlib
  (`alloy-ffi`) linked via cgo. Build tag skipped for now.
- Arguments: Go owns defaults and validation (generated). Encoded as
  CBOR, keyed by field name (`fxamacker/cbor/v2` in Go, serde +
  `ciborium` in Rust). No JSON, no C structs.
- Generate the Go side of every Rust component from the Rust registry
  with a Rust binary (`alloy-gen`). Rust is the source of truth for
  args. Generated code is checked in; validation is generated into Go;
  custom Go goes in a handwritten file next to `zz_generated.go`.
- Registration: explicit `register()` list per crate. Auto-registration
  (inventory/linkme) rejected for now: the linker can drop entries in a
  staticlib linked into Go, and it hides the list.
- One batch type and wire format in both directions (`LogBatch`).
  Handwritten for now; FlatBuffers/protobuf deferred.

## Seam: loki pipeline v2

- Sources push `loki.Batch` into `loki.FanoutV2.Consume(ctx, batch)`
  (`internal/component/common/loki/fanout_v2.go`); downstream components
  export `loki.LogsReceiverV2` wrapping a `loki.Consumer`.
- `Consume` returning nil means the batch can be committed; context errors
  and `ErrConsumerStopped` mean don't commit (retry on the latter).
- v2 is enabled via the `lokipipelinev2` service; `loki.source.file` and
  `loki.echo` are current v2 users.

## Progress

### Rust workspace (`rust/`)

- `alloy-core`: component traits and shared plumbing.
  - `Component: ComponentMeta + Send` (`type Args`, `new`),
    `ComponentMeta { NAME, STABILITY }`.
  - `Signal` (`type Batch<'a>: Send`, `KIND`, `encode`, `read`).
  - `Source<S> { run(&mut self, ctx, out: &Output<S>) }`,
    `Sink<S>: Sync { consume(&self, &S::Batch) }`.
  - `Context` (closed flag, `sleep`), `Output<S>`/`OutputReceiver<S>` (bounded
    channel, cap 1, `send` blocks = backpressure). Shutdown: close ctx ->
    `run` returns and drops all `Output`s -> `recv` fails -> Go's drain
    loop ends.
  - `logs`: `Logs` signal, `LogBatch`/`LogStream`/`LogEntry`/`Label`,
    strings as `Cow<str>`. `read` borrows from the buffer (invalid UTF-8
    replaced), `push(labels, LogEntry::new(ts, line))` builds owned
    batches, `encode()`. Wire format: LE,
    length-prefixed; streams with labels, entries with ts nanos, line,
    structured metadata.
  - `Arguments: DeserializeOwned + Send { fn schema() }`.
  - `schema`: `ComponentInfo { name, kind, signal, stability, args }`,
    `Field { name, ty, default, min, max, doc }`, `FieldType` (ints,
    floats, bool, `Duration`, `String`, `StringMap`), `Value` (incl.
    `Str`).
  - `Registry`: `register_source::<Logs, C>()` /
    `register_sink::<Logs, C>()`, `build(name, args) ->
    Instance::{Source, Sink}` (CBOR decode with `ciborium`, rejects
    trailing bytes, then `new`), `components()`. Signal-erased
    `DynSource { run(ctx), recv() -> Option<Vec<u8>> }` (owns the typed
    channel) and `DynSink { consume(&[u8]) }`, so the C API stays byte
    based and signal-agnostic.
- `alloy-core-macros` (re-exported from `alloy-core`):
  - `#[arguments]` on one struct derives serde `Deserialize` with
    `deny_unknown_fields` (via `alloy_core::serde`, so component crates
    don't depend on serde) and implements `Arguments::schema()`. Field
    types: ints, floats, bool, `Duration` (decoded from integer nanos,
    Go's encoding), `String`, `BTreeMap`/`HashMap<String, String>`. Doc
    comments -> `doc`, `#[arg(default, min, max)]` with literals
    (durations as `"1s"`; min/max only on numbers and durations; maps
    can't have defaults), no default = required. Values checked at
    compile time. Errors name the field, e.g. "unknown field `nope`,
    expected `lines_per_batch` or `interval`" / "missing field ..".
  - `#[component(name = "..", stability = Experimental)]` implements
    `ComponentMeta`.
  - Inspect output with `cargo expand -p <crate> <module>` (needs nightly)
    or `RUSTC_BOOTSTRAP=1 cargo rustc -p <crate> --profile=check --
    -Zunpretty=expanded`.
- `alloy-loki-source`: `loki.source.rustgen` (`lines_per_batch: u32`,
  `interval: Duration`; emits streams without labels).
- `alloy-loki-sink`: `loki.rustecho` (prints labels, ts, line,
  structured metadata to stderr until Rust logging exists).
- `alloy-ffi`: the single staticlib (`liballoy_ffi.a`, also `rlib`).
  `registry()` built once from each crate's `register()`. C API in
  `include/alloy.h`: `alloy_component_new(name, args, size, &err)`,
  `alloy_source_run` (blocking), `alloy_source_recv` (blocking,
  `ALLOY_CLOSED` at end), `alloy_sink_consume(c, buf, len, &err)`
  (`ALLOY_OK`/`ALLOY_CLOSED`/`ALLOY_ERROR`), `alloy_component_close`,
  `alloy_component_free`, `alloy_buf_free`. No per-component types in the
  header. Zero-length args/buffers passed as null.
- `alloy-gen <repo root>`: writes
  `internal/component/<name dots->slashes>/zz_generated.go` per component
  and gofmts it: register call by kind, `Arguments` (one field per
  schema field with `alloy` and `cbor:"<name>"` tags; sources also get
  squashed `LokiSourceArguments` with `cbor:"-"`), `DefaultArguments`,
  `SetToDefault`, `Validate` (schema min/max, else the Rust type's range;
  durations >= 0), `LokiSource()` for sources. Go types: ints -> `int`,
  floats -> `float64`, `time.Duration`, `string`, `map[string]string`.

### Go

- `internal/component/common/rustcomponent`: the only package with cgo /
  link flags.
  - `EncodeArgs(v)`: `cbor.Marshal` of the generated arguments.
  - `Source` (`NewSource`, `Run`, `Recv(labels) (loki.Batch, bool,
    error)`, `Close`, `Free`), `Sink` (`NewSink`, `Consume`, `Close`,
    `Free`). `encodeBatch`/`decodeBatch` for the wire format
    (`decodeBatch` merges the component's static labels; Rust stream
    labels win).
  - `RegisterLokiSource[A]`: v2 check, fanout, two goroutines (`Run` +
    drain loop on `Recv`), own retry on `ErrConsumerStopped` (fresh
    `Clone` per attempt, dskit backoff 1ms..1s), restart when encoded
    args or labels change. Drain keeps receiving after cancel until
    `ALLOY_CLOSED`. Stop order: cancel ctx, close, wait both, free.
    Shared args `LokiSourceArguments { forward_to, labels }` via a named
    `alloy:",squash"` field (Alloy syntax panics on embedded fields).
  - `RegisterLokiSink[A]`: exports `receiver`, Rust sink created in `New`,
    `Consume` under RLock (`ErrConsumerStopped` once stopped), `Update`
    swaps the sink when args change, `Run` closes + frees under Lock.
- Generated packages: `internal/component/loki/source/rustgen`,
  `internal/component/loki/rustecho` (only `zz_generated.go`). `all.go`
  imports added by hand.
- `make rust` (cargo release build, `make alloy` depends on it),
  `make generate-rust-components` (not in `make generate`, no CI check).
- Configs: `rustgen.alloy` (rustgen -> `loki.echo`), `rustecho.alloy`
  (rustgen -> rustecho, Rust -> Go -> Rust today).

### Known gaps

- `TestRegisteredComponents` needs `-update`; no docs pages for the new
  components (docs generator test).
- `make test`/`make lint` don't build Rust; Go may not notice when
  `liballoy_ffi.a` changes (unverified, stale test cache/binaries).
- Rust panics abort Alloy (no `catch_unwind`); Rust can't log through
  Alloy yet.
- `NewSource` on a sink name (or vice versa) isn't rejected.
- Label order in `encodeBatch` follows Go map iteration.
- Processor: parked (trait removed from the tree). Planned path: Go
  `Consume` encodes -> `alloy_processor_process(c, in, len, &out, &err)`
  -> Go decodes -> `fanout.Consume`, returning its error (same retry
  semantics as Go processors). `RegisterLokiProcessor[A]` with squashed
  `forward_to`, generator `Kind::Processor`.

## Open questions

- What is the goal of the experiment?
- Next component: `loki.rustaddlabels` (map args now supported).
  - Batches (hot path): maybe Loki's own `push.proto` via `prost` +
    `protox` later; keep handwritten for now.

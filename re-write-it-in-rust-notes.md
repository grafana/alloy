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
  pipeline (v2), i.e. a mixed Rust -> Go chain.
- The Rust source is a toy generator (synthetic lines), not a file tailer.
- Go pulls (Rust -> Go edges only): a Go goroutine calls a blocking Rust
  `next_batch`, converts to `loki.Batch` and calls `Consume`. No Rust -> Go
  callbacks.
- Rust -> Rust edges link directly inside Rust, no FFI per batch. Go keeps
  the graph: a Rust component's exported receiver carries an opaque handle
  to its Rust consumer. On `forward_to` updates the Go shell of a Rust
  component splits children into Rust handles (passed to a Rust-side
  fanout, called directly) and Go consumers (served via the pull path).
  FFI is used only for rewiring and at real Rust -> Go edges.
  Needs: a Rust mirror of `FanoutV2` (incl. `ErrConsumerStopped`
  semantics on reload) and ref-counted component lifetimes so handles held
  by Go stay valid.
- FFI format: Rust fills a hand-rolled length-prefixed byte buffer (not
  the FlatBuffers library), Go decodes and copies into `loki.Batch`.
  Revisit FlatBuffers once labels/structured metadata cross the boundary.
- Layout: cargo `staticlib` in repo-root `rust/`, linked via cgo, Go side
  behind a build tag.

## Seam: loki pipeline v2

- Sources push `loki.Batch` into `loki.FanoutV2.Consume(ctx, batch)`
  (`internal/component/common/loki/fanout_v2.go`); downstream components
  export `loki.LogsReceiverV2` wrapping a `loki.Consumer`.
- `Consume` returning nil means the batch can be committed; context errors
  and `ErrConsumerStopped` mean don't commit (retry on the latter).
- v2 is enabled via the `lokipipelinev2` service; `loki.source.file` and
  `loki.echo` are current v2 users.

## Progress

- `rust/`: cargo workspace.
- `rust/rustgen`: single staticlib crate for now (no lib-crate split yet).
  C API in `include/rustgen.h`: `rg_new(lines_per_batch, interval_ms)`,
  blocking `rg_next_batch`, `rg_buf_free`, `rg_close` (unblocks, safe to
  call concurrently), `rg_free` (only after no call is running). Buffer is
  repeated `i64 LE ts nanos | u32 LE len | line bytes`; labels are not in
  the buffer. Not wired into Go yet.
- `make rust`: `cargo build --release` in `rust/`, output
  `rust/target/release/librustgen.a`. `make alloy` depends on it.
- Build tag: skipped for now.
- `loki.source.rustgen` (`internal/component/loki/source/rustgen`):
  experimental, v2 only. Args `forward_to`, `lines_per_batch` (100),
  `interval` (1s), `labels`. `generator.go` is the cgo wrapper; `Run`
  loops on the blocking `rg_next_batch` in a goroutine and flushes through
  `loki.StreamBatcherV2` (retry on `ErrConsumerStopped`). Changing
  anything but `forward_to` recreates the generator (close, wait, free).
  Linked statically via `${SRCDIR}` path to `librustgen.a`.

## Open questions

- What is the goal of the experiment?

# beyla.ebpf

Runs [Grafana Beyla](https://github.com/grafana/beyla) (eBPF auto-instrumentation)
as a **child process**: Alloy translates the component config to Beyla YAML, spawns
the embedded Beyla binary, then ingests Beyla's telemetry and forwards it
downstream. Supported on `linux/amd64` and `linux/arm64`; elsewhere it is a no-op.

## Flow

```
Alloy config ─► Arguments ─► config.Build ─► Beyla YAML (in-memory fd) ─► exec_linux.go spawns Beyla
```

While the subprocess runs, the component (`beyla_linux.go`):

- ingests Beyla's OTLP and forwards it to the component's `output` — `otlp_linux.go`
- proxies Prometheus `/metrics` + pprof scrapes to the subprocess — `serveHTTP`
- probes liveness and restarts on failure — `watchdog_linux.go`

## Files

### Component & lifecycle

| file | role |
|---|---|
| `beyla_linux.go` | The component. Registration, `New`/`Run`/`Update`, health, the HTTP scrape proxy (`serveHTTP`), config-reload handling, target exports, and subprocess orchestration (spawn / monitor / restart). |
| `beyla_placeholder.go` | No-op `Component` for non-`linux/{amd64,arm64}` builds; logs a warning and does nothing. |

### Config

| file | role |
|---|---|
| `args.go` | `Arguments` (an alias to `config.Arguments`) + `Exports`. The config types and translation live in [`internal/config`](./internal/config/gen/README.md). |
| `config.go` | `writeConfigFile`: `config.Build(args, runtime)` → YAML → an in-memory fd (memfd) handed to Beyla. |

### Subprocess

| file | role |
|---|---|
| `exec_linux.go` | Spawns Beyla: raises eBPF capabilities, extracts the embedded binary to a memfd, `exec`s it, and pipes its logs. |
| `watchdog_linux.go` | Periodic liveness probe; a dead/unhealthy subprocess triggers a restart. |
| `beyla_embed_amd64.go`, `beyla_embed_arm64.go` | `//go:embed binaries/<arch>` the downloaded Beyla binary into the Alloy binary (an `embed.FS` per arch; read at runtime by `embeddedBeylaBinary`). |
| `fd_linux.go` | Low-level fd helpers shared by config + subprocess: `createExecMemfd` and `writeData` (write-all with EINTR retry). |

### Telemetry

| file | role |
|---|---|
| `otlp_linux.go` | Local OTLP receiver: ingests the subprocess's OTLP over a unix socket and forwards to the component's `output` (otelcol consumers). |

### Subpackages

| dir | role |
|---|---|
| `internal/config` | Beyla config types and the hand-written Args→YAML translation. See its [README](./internal/config/gen/README.md). |
| `internal/subprocess` | `Handle`: thread-safe subprocess state (pid, ports, restart bookkeeping). |
| `internal/health` | `Reporter`: thread-safe component health. |

### Tests

| file | role |
|---|---|
| `sections_test.go` | End-to-end: every config section reaches the emitted Beyla YAML. |
| `beyla_linux_test.go` | Deprecation-warning behavior. |

Also present: `binaries/{amd64,arm64}/beyla` — the downloaded Beyla binaries (gitignored,
embedded at build) — each with a committed `placeholder` so `//go:embed` matches before the
download, plus `.beyla-binary-version` (stamp). Managed by `make beyla`. Without the
download only the placeholder is embedded, and `beyla.ebpf` errors at runtime if used.

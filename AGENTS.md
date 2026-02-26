# AGENTS.md

## Cursor Cloud specific instructions

### Overview

Grafana Alloy is an OpenTelemetry Collector distribution written in Go with an embedded React UI. It is a single binary that runs programmable observability pipelines.

### System dependencies

- **Go 1.25.7** (check `go.mod` for current version)
- **libsystemd-dev** (required on Linux for Loki components)
- **Node.js + npm** (for building the embedded UI at `internal/web/ui`)
- **golangci-lint v2** (install via `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`)

Ensure `$HOME/go/bin` is on `PATH` (already configured in `~/.bashrc`).

### Key commands

All commands are documented in the `Makefile`. Quick reference:

| Task | Command |
|---|---|
| Build binary | `make alloy` (builds UI + Go binary to `build/alloy`) |
| Build binary (skip UI) | `SKIP_UI_BUILD=1 make alloy` |
| Run | `./build/alloy run <CONFIG_FILE>` |
| Lint (Go) | `make lint` |
| Lint (UI) | `cd internal/web/ui && npx eslint .` |
| Test | `make test` |
| Test (single package) | `go test -race ./internal/runtime/...` |
| UI dev server | `cd internal/web/ui && npm run dev` |

### Gotchas

- The project has **three Go modules**: root (`go.mod`), `syntax/go.mod`, and `collector/go.mod`. Run `go mod download` in each when refreshing dependencies.
- `make alloy` automatically runs `npm install && npm run build` in `internal/web/ui` before compiling Go. Use `SKIP_UI_BUILD=1` to skip if UI assets already exist.
- The `make lint` target requires both `golangci-lint` and the custom `alloylint` binary (built automatically by the target).
- Alloy serves its web UI on `http://localhost:12345` by default.
- The `example-config.alloy` file expects docker-compose backends (Mimir, Loki, Tempo). For standalone testing, create a minimal config with just `prometheus.exporter.unix` and `prometheus.scrape` components.
- `golangci-lint` v2 may exit with code 7 (warning) even with 0 issues. This is not a failure.

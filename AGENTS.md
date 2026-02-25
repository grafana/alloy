# Agents

## Cursor Cloud specific instructions

### Overview

Grafana Alloy is a Go-based OpenTelemetry Collector distribution. It is a monorepo with multiple Go modules and an embedded React web UI.

### Key services

| Service | Description | How to run |
|---|---|---|
| **Alloy binary** | Main telemetry agent (Go) | `make alloy` then `./build/alloy run <config.alloy>` |
| **Web UI** | React debugging UI embedded in the binary | Served at `http://localhost:12345` when Alloy runs |

### Build and test commands

Refer to the `Makefile` at the repo root for all targets. Key commands:

- **Build**: `make alloy` (builds UI first, then Go binary). Use `SKIP_UI_BUILD=1 make alloy` if UI assets already exist.
- **Test**: `make test` runs Go tests across all modules with `-race`.
- **Lint (Go)**: `make lint` (requires `golangci-lint` and builds `alloylint` first).
- **Lint (UI)**: `cd internal/web/ui && npm run lint` and `npm run format`.
- **UI dev server**: `cd internal/web/ui && npm run dev` (Vite dev server on port 5173).

### Go modules

There are 5 independent Go modules with separate `go.mod` files:

| Module | Path |
|---|---|
| Main | `/workspace` |
| Syntax | `/workspace/syntax` |
| Collector | `/workspace/collector` |
| Alloy Engine Extension | `/workspace/extension/alloyengine` |
| Tools | `/workspace/tools` |

Run `go mod download` in each if needed. `golangci-lint` must be run from each module's directory.

### Non-obvious caveats

- **Node.js version**: The UI requires Node.js 24.x (see `internal/web/ui/.nvmrc`). Use `nvm use 24` before working in the UI directory.
- **libsystemd-dev**: Required system package for CGO builds (Loki journal components). Install via `apt-get install -y libsystemd-dev`.
- **golangci-lint**: Must be in `$PATH`. Installed to `$HOME/go/bin/golangci-lint`. Ensure `$HOME/go/bin` is in `$PATH`.
- **scrape_interval config**: When writing test Alloy configs, `scrape_interval` must be >= `scrape_timeout` (default 10s). Setting `scrape_interval` below 10s without also lowering `scrape_timeout` causes a startup error.
- **Alloy default port**: The HTTP server defaults to `localhost:12345`. Override with `--server.http.listen-addr`.
- **UI build**: The `make alloy` target runs `generate-ui` first (npm install + build). If the UI is already built, set `SKIP_UI_BUILD=1` to skip.
- **Integration tests**: Docker-based integration tests (`make integration-test-docker`) require Docker and external service containers. They are not needed for regular development.

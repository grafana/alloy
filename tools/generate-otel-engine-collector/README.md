# generate-otel-engine-collector

Generates the Alloy OTel Collector distribution: runs the OpenTelemetry Collector Builder (OCB) against `collector/builder-config.yaml`, runs `go mod tidy` in the collector directory, then post-processes the generated `main.go` and writes `main_alloy.go` from an embedded template.

## Usage

```bash
make generate-otel-collector-distro
```

`BUILDER_VERSION` is defined in the root Makefile and should be kept in sync with OTel component versioning.

## Flags

- `--collector-dir` - Path to the collector directory (contains builder-config.yaml). Required.
- `--builder-version` - OTel builder version (e.g. v0.139.0). Defaults to `BUILDER_VERSION` env var.
- `--from-scratch` - Remove main*.go, components.go, go.mod, and go.sum before generating. Use this to generate from a clean state.

## Build Process

The tool performs the following steps:

### 0. Optional clean (when `--from-scratch` is set)

Removes `main*.go`, `components.go`, `go.mod`, and `go.sum` before generating to produce a clean state.

### 1. OCB Generation (`go run go.opentelemetry.io/collector/cmd/builder@${BUILDER_VERSION}`)

The BUILDER_VERSION is defined in the root Makefile, and should be kept in sync with our general OTel component versioning.

The OCB tool reads `builder-config.yaml` and generates:
- `main.go` - The main entry point for the collector
- `components.go` - Component factories and registrations
- `go.mod` and `go.sum` - Module dependencies
- Other platform-specific main files (`main_others.go`, `main_windows.go`)

The `builder-config.yaml` file defines:
- General collector distribution metadata
- The Open Telemetry components to include in the distribution (receivers/processors/exporters etc)
- Replace directives, which are automated/generated to keep in sync with `dependency-replacements.yaml` in the root of the project

### 2. Go Module Tidying (`go mod tidy`)

Ensures the generated `go.mod` is properly formatted and all dependencies are resolved.

### 3. Post-processing

1. **Generates `main_alloy.go`** - Creates a wrapper that integrates the OTel collector command into Alloy's CLI as a subcommand (`alloy otel`)
2. **Modifies `main.go`** - Patches the OCB output in two places: replaces `otelcol.NewCommand(params)` with `newAlloyCommand(params)` to use the Alloy-integrated command, and replaces the hardcoded `Version` field with `CollectorVersion()` so the collector reports the Alloy release version

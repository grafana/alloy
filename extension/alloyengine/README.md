# Alloy Engine Extension

The `alloy engine` extension embeds the **Default Engine** (the underlying Alloy runtime used by `alloy run`) within the **OTel Engine** (the OpenTelemetry Collector runtime exposed via the `otel` subcommand).

This extension allows you to run a Default Engine pipeline set up with Alloy configuration alongside the OTel Engine set up with YAML configuration. These two pipelines run in parallel, and can't natively interact with one another.

If the Alloy configuration fails to load for whatever reason, the extension continues retrying at most every 15 seconds.

## Configuration

The extension accepts the following configuration fields:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `config` | object | Yes | - | The Alloy configuration source. See [Config Object](#config-object) for details. |
| `flags` | map[string]string | No | `{}` | Additional flags to pass to the `alloy run` command. Flags should be specified without the leading `--` prefix. |

### Config Object

The `config` object specifies the Alloy configuration source. Exactly one of `path` (or the deprecated `file`) or `inline.content` must be set.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `path` | string | Required unless `inline.content` is set | - | Path to an Alloy config file or a directory containing `.alloy` files. Mutually exclusive with `inline.content`. |
| `inline` | object | Required unless `path` is set | - | Inline Alloy configuration. See [Inline Object](#inline-object) for details. |

### Inline Object

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `content` | string | Yes | - | The inline Alloy configuration to run. |
| `module_path` | string | No | current working directory | Value resolved for the `module_path` Alloy config keyword. |

### Example Configuration

```yaml
extensions:
  alloyengine:
    config:
      path: ./config.alloy
    flags:
      server.http.listen-addr: 0.0.0.0:12345
      stability.level: experimental

service:
  extensions: [alloyengine]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
```

In this example, the extension:
1. Starts the default engine with the configuration file at the relative path `./config.alloy`.
2. Passes the `--server.http.listen-addr=0.0.0.0:12345` and `--stability.level=experimental` flags to the `alloy run` command.
3. Runs the Alloy configuration concurrently with the OpenTelemetry Collector pipeline.

## Lifecycle

The extension manages the lifecycle of the embedded default engine:

- **Start**: When the extension starts, it launches the default engine in a separate goroutine and runs the Alloy configuration.
- **Ready**: The extension reports ready once the default engine has successfully started.
- **Shutdown**: When the extension shuts down, it gracefully terminates the default engine and waits for it to exit.

## Limitations

Only one alloyengine instance can be active per process. The embedded Default Engine uses process-global state (Prometheus registry, controller ID, storage path and so forth), so running multiple instances will cause conflicts. If you configure multiple alloyengine extensions, only the first to start will succeed; subsequent instances will fail at startup with a clear error.

Please note that if extensions fail to start, the collector will also fail to start. This means that the errors described above will ultimately mean you cannot start the collector without ensuring that you specify which of the alloyengine extensions you wish to run.

If `config.inline.module_path` isn't defined, `config.inline` resolves the `module_path` Alloy config keyword to the current working directory of the OpenTelemetry Collector process.

The `remotecfg` Alloy configuration block can't be used with the alloyengine extension. Use OpenTelemetry OpAMP for Collector configuration management instead.

The UI assets aren't included in the Alloy Go module, so an OCB build needs an extra step to serve the web UI. See [Build with the UI embedded](#build-with-the-ui-embedded) for details.

## Stability

This extension is currently marked as **experimental** stability level. The API and behavior may change in future releases.

## Include `alloyengine` extension in an OCB distribution

You can include the `alloyengine` extension in a custom OpenTelemetry Collector distribution built with [OpenTelemetry Collector Builder (OCB)](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder) by following the steps below.

### Choose the Alloy version

The `alloyengine` extension is released and versioned together with the rest of Alloy. Choose the version you want to embed from the [releases page](https://github.com/grafana/alloy/releases). We recommend the latest version.

The rest of these steps refer to this version as `<ALLOY_VERSION>`.

### Use a compatible OCB version

OCB uses a fixed version of the OpenTelemetry Collector core dependencies in the generated distribution's `go.mod`. The build config has no field to change this. The deprecated `dist.otelcol_version` was removed from OCB in [opentelemetry-collector#11405](https://github.com/open-telemetry/opentelemetry-collector/issues/11405) and is now silently ignored.

To avoid compatibility issues between components and extensions, use an OCB version that matches the OpenTelemetry Collector version Alloy depends on. Read the version of the `go.opentelemetry.io/collector/otelcol` dependency from Alloy's root `go.mod` at `<ALLOY_VERSION>`:

```shell
curl -s https://raw.githubusercontent.com/grafana/alloy/<ALLOY_VERSION>/go.mod | grep 'go.opentelemetry.io/collector/otelcol '
# go.opentelemetry.io/collector/otelcol v0.158.0
```

Use this version as `<OCB_VERSION>` to fetch OCB and verify that it runs:

```shell
go run go.opentelemetry.io/collector/cmd/builder@<OCB_VERSION> version
# ocb version <OCB_VERSION>
```

### Add the extension to the OCB builder config

Add the `alloyengine` extension to your OCB builder YAML config file. It's important to set the `import` field as the extension doesn't have its own Go module. For example:

```yaml
extensions:
  - gomod: github.com/grafana/alloy <ALLOY_VERSION>
    import: github.com/grafana/alloy/extension/alloyengine
    name: alloyengine
```

Replace `<ALLOY_VERSION>` with the version you chose.

### Include replace directives from Alloy

The `alloyengine` extension needs a set of replace directives to work correctly. Go ignores the replace directives of the modules it depends on, so you must carry them in your own builder config. The build fails without them.

Copy the whole `replaces` block from Alloy's `collector/builder-config.yaml` at `<ALLOY_VERSION>` into your own distribution's `builder-config.yaml`. This includes the replaces between the `<BEGIN_SHARED_REPLACE_DIRECTIVES>` and `<END_SHARED_REPLACE_DIRECTIVES>` markers.

These replaces change between Alloy versions, so use the file at the version you chose:

```shell
curl -s https://raw.githubusercontent.com/grafana/alloy/<ALLOY_VERSION>/collector/builder-config.yaml
```

### Enable CGO

Set `dist.cgo_enabled: true` in the OCB builder config. OCB disables CGO by default, while Alloy builds assume CGO is enabled unless you intentionally opt out. On Linux, make sure the build environment has the required system development libraries, such as `libsystemd-dev`.

### Choose how to build

The remaining steps depend on whether you want the Default Engine web UI, by default served on port `12345`.

The UI assets aren't included in the Alloy Go module, so you can only build them from a local Alloy checkout. Without them the UI returns `404 Not Found`, while other endpoints, such as the HTTP API and GraphQL, still work.

Follow one of these sections:

- [Build without the UI](#build-without-the-ui) uses a released Alloy version and needs no checkout.
- [Build with the UI embedded](#build-with-the-ui-embedded) needs a local Alloy checkout and Node.js.

### Build without the UI

1. Remove these two local module replaces from the `replaces` block you copied earlier:

   ```yaml
   replaces:
     - github.com/grafana/alloy => ../
     - github.com/grafana/alloy/syntax => ../syntax
   ```

   Their paths are relative and only work from Alloy's `collector/` directory. Your build then uses the released Alloy version you set in `gomod`.

1. Run OCB with the version you chose earlier:

   ```shell
   go run go.opentelemetry.io/collector/cmd/builder@<OCB_VERSION> --config=builder-config.yaml
   ```

OCB writes the binary into the `dist.output_path` directory, named after `dist.name`.

### Build with the UI embedded

1. Clone the Alloy repository and check out `<ALLOY_VERSION>`:

   ```shell
   git clone https://github.com/grafana/alloy.git
   cd alloy
   git checkout <ALLOY_VERSION>
   ```

1. Build the UI assets. We use [mise](https://mise.jdx.dev), which installs the local toolchain, including the Node.js version that Alloy builds with. Versions are pinned in [`mise.toml`](../../mise.toml). Other versions may fail to build.

   ```shell
   mise install
   make generate-ui
   ```

1. Point these two local module replaces at your local Alloy checkout:

   ```yaml
   replaces:
     - github.com/grafana/alloy => ../              # <- Change this to your checkout path
     - github.com/grafana/alloy/syntax => ../syntax # <- Change this to your checkout path + /syntax
   ```

   Change both to absolute paths to your checkout. For example, if you cloned Alloy into `/path/to/alloy`, use:

   ```yaml
   replaces:
     - github.com/grafana/alloy => /path/to/alloy
     - github.com/grafana/alloy/syntax => /path/to/alloy/syntax
   ```

1. Add the `embedalloyui` build tag, which embeds the assets you built into the binary:

   ```yaml
   dist:
     build_tags: embedalloyui
   ```

   If you already set other build tags, add `embedalloyui` to the comma-separated list: `build_tags: "<other_tags>,embedalloyui"`.

1. Run OCB with the version you chose earlier:

   ```shell
   go run go.opentelemetry.io/collector/cmd/builder@<OCB_VERSION> --config=builder-config.yaml
   ```

OCB writes the binary into the `dist.output_path` directory, named after `dist.name`.

### Verify the extension works

Start your distribution with a collector configuration that enables the `alloyengine` extension. The following configuration runs a `alloyengine` with minimal config and serves the Default Engine HTTP server on port `12345`:

```yaml
extensions:
  alloyengine:
    config:
      inline:
        content: |
          logging {
            level = "info"
          }
    flags:
      server.http.listen-addr: 127.0.0.1:12345

receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 127.0.0.1:4317

exporters:
  debug:

service:
  extensions: [alloyengine]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
```

This example uses `otlp` receiver and a `debug` exporter, which may not be included in your distribution. Replace them with your own pipeline if needed.

Check that it's running:

```shell
curl localhost:12345/-/ready
# Alloy is ready.
```

If you built with the UI, open `http://localhost:12345` in a browser to check that it loads.

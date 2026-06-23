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

The `config` object specifies the Alloy configuration source. Either `path` or `inline` must be set, but not both.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `path` | string | No | - | Path to an Alloy config file or a directory containing `.alloy` files. |
| `inline` | object | No | - | Inline Alloy configuration. See [Inline Object](#inline-object) for details. |

### Inline Object

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `content` | string | Yes | - | The inline Alloy configuration to run. |
| `module_path` | string | No | current working directory | Value resolved for the `module_path` Alloy config keyword. Has no effect when `config.path` is set. |

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

## Build with OCB

To include the extension in an external OpenTelemetry Collector Builder (OCB) distribution, add it as an extension using the Alloy module plus the extension import path. The extension doesn't have its own Go module, so the one-line `gomod: github.com/grafana/alloy/extension/alloyengine ...` form doesn't work.

```yaml
dist:
  name: custom-alloy-otel
  output_path: ./dist
  cgo_enabled: true

extensions:
  - gomod: github.com/grafana/alloy v<ALLOY_VERSION>
    import: github.com/grafana/alloy/extension/alloyengine
    name: alloyengine
```

Replace `v<ALLOY_VERSION>` with the Alloy version you want to embed.

### Replace directives

If you copy from Alloy's in-tree `collector/builder-config.yaml`, don't copy the local module replaces unchanged. These paths only work from Alloy's `collector/` directory:

```yaml
replaces:
  - github.com/grafana/alloy => ../
  - github.com/grafana/alloy/syntax => ../syntax
```

For a build that uses a released Alloy version, remove both local module replaces.

For a build that uses a local Alloy checkout, keep both replaces but point them at your checkout:

```yaml
replaces:
  - github.com/grafana/alloy => /path/to/alloy
  - github.com/grafana/alloy/syntax => /path/to/alloy/syntax
```

The `github.com/grafana/alloy/syntax` replace must point at the `syntax` submodule in the same checkout as the Alloy module.

If you copy additional replace directives from Alloy's `collector/builder-config.yaml`, keep the shared remote replaces between the `<BEGIN_SHARED_REPLACE_DIRECTIVES>` and `<END_SHARED_REPLACE_DIRECTIVES>` markers. Those replaces track dependency forks and pins that Alloy needs during OCB builds.

### CGO

Set `dist.cgo_enabled: true` in the OCB builder config. OCB disables CGO by default, while Alloy builds assume CGO is enabled unless you intentionally opt out. On Linux, make sure the build environment has the required system development libraries, such as `libsystemd-dev`.

## Lifecycle

The extension manages the lifecycle of the embedded default engine:

- **Start**: When the extension starts, it launches the default engine in a separate goroutine and runs the Alloy configuration.
- **Ready**: The extension reports ready once the default engine has successfully started.
- **Shutdown**: When the extension shuts down, it gracefully terminates the default engine and waits for it to exit.

## Limitations

Only one alloyengine instance can be active per process. The embedded Default Engine uses process-global state (Prometheus registry, controller ID, storage path and so forth), so running multiple instances will cause conflicts. If you configure multiple alloyengine extensions, only the first to start will succeed; subsequent instances will fail at startup with a clear error.

Please note that if extensions fail to start, the collector will also fail to start. This means that the errors described above will ultimately mean you cannot start the collector without ensuring that you specify which of the alloyengine extensions you wish to run.

## Stability

This extension is currently marked as **experimental** stability level. The API and behavior may change in future releases.

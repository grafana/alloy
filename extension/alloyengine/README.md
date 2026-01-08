# Alloy Engine Extension

The `alloy engine` extension embeds the **Default Engine** (the underlying Alloy runtime used by `alloy run`) within the **OTel Engine** (the OpenTelemetry Collector runtime exposed via the `otel` subcommand).

This extension allows you to run a Default Engine pipeline set up with Alloy configuration file alongside the OTel Engine set up with YAML configuration. These two pipelines will be ran in parallel, and cannot natively interact with one another.

If the alloy configuration file fails to load for whatever reason, the extension will continue retrying at most every 15 seconds.

## Configuration

The extension accepts the following configuration fields:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `config` | object | Yes | - | The Alloy configuration source. See [Config Object](#config-object) for details. |
| `flags` | map[string]string | No | `{}` | Additional flags to pass to the `alloy run` command. Flags should be specified without the leading `--` prefix. |

### Config Object

The `config` object specifies the Alloy configuration source.
Currently we only support `file` as an input type, but this is planned to be extended to other formats

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `file` | string | Yes | - | The path to the Alloy configuration file to run. |

### Example Configuration

```yaml
extensions:
  alloyengine:
    config:
      file: ./config.alloy
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

In this example, the extension will:
1. Start the default engine with the configuration file at the relative path `./config.alloy`
2. Pass the `--server.http.listen-addr=0.0.0.0:12345` and `--stability.level=experimental` flags to the `alloy run` command
4. Run the Alloy configuration concurrently with the OpenTelemetry Collector pipeline

## Lifecycle

The extension manages the lifecycle of the embedded default engine:

- **Start**: When the extension starts, it launches the default engine in a separate goroutine, executing the specified Alloy configuration file
- **Ready**: The extension reports ready once the default engine has successfully started
- **Shutdown**: When the extension is shut down, it gracefully terminates the default engine and waits for it to exit

## Stability

This extension is currently marked as **Development** stability level. The API and behavior may change in future releases.


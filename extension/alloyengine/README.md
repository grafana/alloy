# Alloy Engine Extension

The `alloy engine` extension embeds the **Default Engine** (the underlying Alloy runtime used by `alloy run`) within the OpenTelemetry Collector runtime exposed through the `otel` subcommand.

This extension allows you to run a Default Engine pipeline set up with inline Alloy configuration alongside the OpenTelemetry Collector runtime set up with YAML configuration. These two pipelines run in parallel, and can't natively interact with one another.

If the Alloy configuration fails to load for whatever reason, the extension continues retrying at most every 15 seconds.

## Configuration

The extension accepts the following configuration fields:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `config` | object | Yes | - | The Alloy configuration source. See [Config Object](#config-object) for details. |
| `flags` | map[string]string | No | `{}` | Additional flags to pass to the `alloy run` command. Flags should be specified without the leading `--` prefix. |

### Config Object

The `config` object specifies the Alloy configuration source.
Currently, the extension supports inline Alloy configuration.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `content` | string | Yes | - | The inline Alloy configuration to run. |
| `module_path` | string | No | - | Value resolved for the `module_path` Alloy config keyword. |

### Example Configuration

```yaml
extensions:
  alloyengine:
    config:
      content: |
        logging {
          level = "debug"
        }
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
1. Starts the default engine with the inline Alloy configuration.
2. Passes the `--server.http.listen-addr=0.0.0.0:12345` and `--stability.level=experimental` flags to the `alloy run` command.
3. Runs the Alloy configuration concurrently with the OpenTelemetry Collector pipeline.

## Lifecycle

The extension manages the lifecycle of the embedded default engine:

- **Start**: When the extension starts, it launches the default engine in a separate goroutine and runs the inline Alloy configuration.
- **Ready**: The extension reports ready once the default engine has successfully started.
- **Shutdown**: When the extension shuts down, it gracefully terminates the default engine and waits for it to exit.

## Limitations

Only one alloyengine instance can be active per process. The embedded Default Engine uses process-global state (Prometheus registry, controller ID, storage path and so forth), so running multiple instances will cause conflicts. If you configure multiple alloyengine extensions, only the first to start will succeed; subsequent instances will fail at startup with a clear error.

Please note that if extensions fail to start, the collector will also fail to start. This means that the errors described above will ultimately mean you cannot start the collector without ensuring that you specify which of the alloyengine extensions you wish to run.

## Stability

This extension is currently marked as **experimental** stability level. The API and behavior may change in future releases.

# Support Bundle Extension

The `supportbundle` extension is an OpenTelemetry Collector extension. It serves an HTTP endpoint that returns a zip archive of diagnostics from a running collector. The archive holds runtime metadata, pprof profiles, the running configuration, and, when configured, collector logs captured during the request. Support engineers use the bundle to debug a running collector.

## Stability

This extension is currently marked as **development** stability level. The API and behavior may change in future releases.

## Security

The bundle can hold sensitive information. Protect the endpoint.

- **The endpoint has no authentication by default.** Anyone who can reach it can download the bundle. The default endpoint is `localhost:8089` (loopback only). If you bind to a non-loopback address, add an authenticator through `confighttp` (see [Log capture](#log-capture) example for the `auth` field) and use TLS.
- **`logs.txt` is not redacted.** Captured logs can contain secrets that the collector logs at runtime.
- **`environment.txt` can contain secrets.** The default allowlist includes proxy variables (`HTTP_PROXY`, `HTTPS_PROXY`, ...), which can hold credentials in their URLs. Variables you add with `environment_variables` are captured verbatim.
- **`config.yaml` is redacted, but not perfectly.** The extension writes the unexpanded configuration. The collector redacts fields typed as secrets (`configopaque.String`), but a secret placed in a plain string field is not redacted. Prefer environment references (`${env:...}`) for secrets so they stay unexpanded in the bundle.

Treat a downloaded bundle as sensitive: store and share it carefully.

## Configuration

The `Config` struct embeds OpenTelemetry's [`confighttp.ServerConfig`][confighttp]. So the extension accepts every server field that `confighttp` provides, such as `endpoint`, `tls`, `auth`, `cors`, and the read and write timeouts.

The extension holds the response open for the whole collection window, so it defaults `write_timeout` to `0` (no limit) instead of the `confighttp` default of `30s`. If you set a non-zero `write_timeout`, it must be greater than `max_collection_duration`, or the bundle download is cut off; the extension rejects a shorter value at startup.

The extension adds the following fields:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | `/support` | The HTTP path that serves the bundle. It must start with `/`. |
| `default_collection_duration` | duration | `30s` | The default collection window for windowed collectors, such as the CPU profile, used when a request does not set `duration`. It must be positive and must not exceed `max_collection_duration`. |
| `max_collection_duration` | duration | `60s` | The upper bound for a requested collection window. It must be positive. |
| `environment_variables` | list of strings | `[]` | Extra environment variable names to capture, beyond the built-in allowlist. See [Environment variables](#environment-variables). |
| `log_buffer_limit` | int | `1048576` | The maximum size of the captured log buffer, in bytes. A value of `0` disables the limit. |

The default endpoint is `localhost:8089`.

### Environment variables

The extension captures an allowlist of environment variables in `environment.txt`. The built-in allowlist matches the Alloy engine support bundle: `AUTOMEMLIMIT`, `GODEBUG`, `GOGC`, `GOMAXPROCS`, `GOMEMLIMIT`, `HOSTNAME`, the proxy variables (`HTTP_PROXY`, `http_proxy`, `HTTPS_PROXY`, `https_proxy`, `NO_PROXY`, `no_proxy`), `PPROF_BLOCK_PROFILING_RATE`, and `PPROF_MUTEX_PROFILING_PERCENT`.

Add extra names with `environment_variables`. The operator owns this list. A request cannot extend it, so a caller cannot read arbitrary environment variables from the process.

```yaml
extensions:
  supportbundle:
    environment_variables: [MY_CUSTOM_FLAG, DEPLOYMENT_REGION]
```

### Example Configuration

Minimal configuration:

```yaml
extensions:
  supportbundle:

service:
  extensions: [supportbundle]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
```

The extension exposes `localhost:8089/support` with the defaults above.

Because the extension embeds `confighttp.ServerConfig`, you can set TLS and authentication on the endpoint. The following example uses TLS and a server authenticator:

```yaml
extensions:
  basicauth/server:
    htpasswd:
      inline: |
        user:password

  supportbundle:
    endpoint: 0.0.0.0:8089
    path: /support
    default_collection_duration: 5s
    max_collection_duration: 60s
    tls:
      cert_file: /path/to/cert.pem
      key_file: /path/to/key.pem
    auth:
      authenticator: basicauth/server

service:
  extensions: [basicauth/server, supportbundle]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
```

See the [`confighttp`][confighttp] documentation for every available server field.

### Log capture

The extension registers a zap log sink named `supportbundle`. To capture collector logs in the bundle, add `supportbundle://` to `service::telemetry::logs::output_paths`. Keep the existing output paths so logs still reach their normal destination.

```yaml
service:
  telemetry:
    logs:
      output_paths: [stderr, supportbundle://]
  extensions: [supportbundle]
```

The extension keeps logs only while it builds a bundle. It captures the lines that the collector writes during the collection window. It discards logs at all other times, so the cost is low when no bundle runs. Without this configuration, the bundle has no `logs.txt`.

## Usage

Send an HTTP request to the configured path to download the bundle:

```sh
curl -o bundle.zip 'localhost:8089/support?duration=2'
```

### Duration query parameter

The `duration` query parameter sets the collection window for the request. It overrides the `default_collection_duration` default. The window applies to the profiles that sample over time: the CPU, mutex, and block profiles.

- A bare number is a value in seconds. For example, `duration=2` means 2 seconds.
- A value with a Go duration unit suffix uses Go duration syntax. For example, `duration=500ms`.
- The extension clamps the value to the range `[0, max_collection_duration]`.
- A value of `0` skips the CPU profile. The mutex and block profiles are still emitted, but they hold little data without a window.
- An invalid value falls back to the `default_collection_duration` default.

The extension serves one bundle at a time. The pprof profiles use process-wide state, so requests do not run in parallel.

## Bundle contents

The response is a zip archive. All entries live under a single root directory, `otelcol-support-bundle/`.

```
otelcol-support-bundle/
├── metadata.yaml
├── config.yaml            # present once the collector sends a config snapshot
├── component-status.yaml  # health of each pipeline component
├── environment.txt        # allowlisted environment variables that are set
├── feature-gates.txt      # every feature gate and its state
├── metrics-start.txt      # metrics at window start (present with a pull endpoint and duration > 0)
├── metrics-end.txt        # metrics at window end (present with a pull endpoint and duration > 0)
├── metrics.txt            # single metrics sample (present with a pull endpoint and duration = 0)
├── pprof/
│   ├── heap.pprof         # always present (point-in-time snapshot)
│   ├── goroutine.pprof    # always present (point-in-time snapshot)
│   ├── mutex.pprof        # always present (sampled over the window)
│   ├── block.pprof        # always present (reflects the operator's block profiling config)
│   └── cpu.pprof          # only present when duration > 0
├── logs.txt               # only present when log capture is configured and logs are written during the window
└── errors.txt             # only present when a gatherer fails
```

- `metadata.yaml` holds build and runtime information. This includes the command, description, version, `GOOS`, `GOARCH`, CPU count, `GOMAXPROCS`, Go version, uptime, start time, hostname, and the collector's telemetry resource attributes (such as `service.name` and `service.instance.id`).
- `config.yaml` holds the collector's running configuration. The collector sends this to the extension through the `ConfigSnapshotWatcher` interface, so no extra setup is needed. The extension keeps the unexpanded form: environment references such as `${env:FOO}` stay intact, and sensitive fields are redacted. The extension never writes the expanded configuration, so it does not leak secrets.
- `component-status.yaml` holds the latest status of each pipeline component. The collector reports status through the `componentstatus.Watcher` interface. Each entry has the component ID, kind, pipelines, status (such as `StatusOK` or `StatusPermanentError`), the last error, and a timestamp.
- `environment.txt` holds the allowlisted environment variables that are set, one `NAME=value` per line. See [Environment variables](#environment-variables).
- `feature-gates.txt` lists every feature gate and whether it is enabled, one `id=bool` per line.
- `metrics-start.txt` / `metrics-end.txt` / `metrics.txt` hold the collector's own telemetry metrics, scraped from the metrics endpoint in the configuration. When the duration is greater than `0`, the extension takes a sample at the start of the window (`metrics-start.txt`) and another at the end (`metrics-end.txt`) so you can compute counter deltas; when the duration is `0`, it takes a single sample (`metrics.txt`). This is best effort: it works only when the collector exposes a pull (Prometheus) metrics reader and the endpoint is reachable. When no such endpoint is configured, the files are absent.
- `pprof/` holds the runtime profiles. The extension always collects the heap and goroutine profiles (point-in-time snapshots), the mutex profile, and the block profile. It collects the CPU profile only when the resolved duration is greater than `0`. The extension enables mutex sampling for the collection window and restores the previous rate afterward, so the mutex profile holds the most data when the duration is greater than `0`. The extension does **not** change the block profile rate (the Go runtime cannot report the current rate, so it cannot be restored). The block profile therefore reflects whatever block profiling the operator has configured, for example through the `pprof` extension; it is empty when block profiling is off.
- `logs.txt` holds the collector logs written during the collection window. It is present only when log capture is configured (see [Log capture](#log-capture)) and the collector writes at least one log line during the window. The buffer is capped at `log_buffer_limit` bytes. When the logs reach the cap, the file ends with a truncation notice.
- `errors.txt` is present only when one or more gatherers fail. It lists each failure. The extension still returns the files that it did collect.

[confighttp]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/confighttp/README.md

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.otlphttp/
aliases:
  - ../otelcol.exporter.otlphttp/ # /docs/alloy/latest/reference/components/otelcol.exporter.otlphttp/
description: Learn about otelcol.exporter.otlphttp
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.exporter.otlphttp
---

# `otelcol.exporter.otlphttp`

`otelcol.exporter.otlphttp` accepts telemetry data from other `otelcol` components and writes them over the network using the OTLP HTTP protocol.

{{< admonition type="note" >}}
`otelcol.exporter.otlphttp` is a wrapper over the upstream OpenTelemetry Collector [`otlphttp`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`otlphttp`]: https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/exporter/otlphttpexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.otlphttp` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.otlphttp "<LABEL>" {
  client {
    endpoint = "<HOST>:<PORT>"
  }
}
```

## Arguments

You can use the following arguments with `otelcol.exporter.otlphttp`:

| Name               | Type     | Description                                                               | Default                           | Required |
| ------------------ | -------- | ------------------------------------------------------------------------- | --------------------------------- | -------- |
| `encoding`         | `string` | The encoding to use for messages. Should be either `"proto"` or `"json"`. | `"proto"`                         | no       |
| `logs_endpoint`    | `string` | The endpoint to send logs to.                                             | `client.endpoint + "/v1/logs"`    | no       |
| `metrics_endpoint` | `string` | The endpoint to send metrics to.                                          | `client.endpoint + "/v1/metrics"` | no       |
| `traces_endpoint`  | `string` | The endpoint to send traces to.                                           | `client.endpoint + "/v1/traces"`  | no       |

The default value depends on the `endpoint` field set in the required `client` block.
If set, these arguments override the `client.endpoint` field for the corresponding signal.

## Blocks

You can use the following blocks with `otelcol.exporter.otlphttp`:

| Block                                                 | Description                                                                | Required |
| ----------------------------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`client`][client]                                    | Configures the HTTP client to send telemetry data to.                      | yes      |
| `client` > [`compression_params`][compression_params] | Configure advanced compression options.                                    | no       |
| `client` > [`cookies`][cookies]                       | Store cookies from server responses and reuse them in subsequent requests. | no       |
| `client` > [`tls`][tls]                               | Configures TLS for the HTTP client.                                        | no       |
| [`debug_metrics`][debug_metrics]                      | Configures the metrics that this component generates to monitor its state. | no       |
| [`retry_on_failure`][retry_on_failure]                | Configures retry mechanism for failed requests.                            | no       |
| [`sending_queue`][sending_queue]                      | Configures batching of data before sending.                                | no       |

The > symbol indicates deeper levels of nesting.
For example, `client` > `tls` refers to a `tls` block defined inside a `client` block.

[client]: #client
[tls]: #tls
[cookies]: #cookies
[compression_params]: #compression_params
[sending_queue]: #sending_queue
[retry_on_failure]: #retry_on_failure
[debug_metrics]: #debug_metrics

### `client`

<span class="badge docs-labels__stage docs-labels__item">Required</span>

The `client` block configures the HTTP client used by the component.

The following arguments are supported:

| Name                      | Type                       | Description                                                                                                        | Default    | Required |
| ------------------------- | -------------------------- | ------------------------------------------------------------------------------------------------------------------ | ---------- | -------- |
| `endpoint`                | `string`                   | The target URL to send telemetry data to.                                                                          |            | yes      |
| `auth`                    | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests.                                       |            | no       |
| `compression`             | `string`                   | Compression mechanism to use for requests.                                                                         | `"gzip"`   | no       |
| `disable_keep_alives`     | `bool`                     | Disable HTTP keep-alive.                                                                                           | `false`    | no       |
| `headers`                 | `map(string)`              | Additional headers to send with the request.                                                                       | `{}`       | no       |
| `http2_ping_timeout`      | `duration`                 | Timeout after which the connection will be closed if a response to Ping isn't received.                            | `"15s"`    | no       |
| `http2_read_idle_timeout` | `duration`                 | Timeout after which a health check using ping frame will be carried out if no frame is received on the connection. | `"0s"`     | no       |
| `idle_conn_timeout`       | `duration`                 | Time to wait before an idle connection closes itself.                                                              | `"90s"`    | no       |
| `max_conns_per_host`      | `int`                      | Limits the total (dialing,active, and idle) number of connections per host.                                        | `0`        | no       |
| `max_idle_conns_per_host` | `int`                      | Limits the number of idle HTTP connections the host can keep open.                                                 | `0`        | no       |
| `max_idle_conns`          | `int`                      | Limits the number of idle HTTP connections the client can keep open.                                               | `100`      | no       |
| `proxy_url`               | `string`                   | HTTP proxy to send requests through.                                                                               |            | no       |
| `read_buffer_size`        | `string`                   | Size of the read buffer the HTTP client uses for reading server responses.                                         | `0`        | no       |
| `timeout`                 | `duration`                 | Time to wait before marking a request as failed.                                                                   | `"30s"`    | no       |
| `write_buffer_size`       | `string`                   | Size of the write buffer the HTTP client uses for writing requests.                                                | `"512KiB"` | no       |

When setting `headers`, note that:

* Certain headers such as `Content-Length` and `Connection` are automatically written when needed and values in `headers` may be ignored.
* The `Host` header is automatically derived from the `endpoint` value. However, this automatic assignment can be overridden by explicitly setting a `Host` header in `headers`.

Setting `disable_keep_alives` to `true` will result in significant overhead establishing a new HTTP or HTTPS connection for every request.
Before enabling this option, consider whether changes to idle connection settings can achieve your goal.

If `http2_ping_timeout` is unset or set to `0s`, it will default to `15s`.

If `http2_read_idle_timeout` is unset or set to `0s`, then no health check will be performed.

{{< docs/shared lookup="reference/components/otelcol-compression-field.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `compression_params`

The `compression_params` block allows for configuration of advanced compression options.

The following arguments are supported:

| Name    | Type  | Description                  | Default | Required |
| ------- | ----- | ---------------------------- | ------- | -------- |
| `level` | `int` | Configure compression level. |         | yes      |

For valid combinations of `client.compression` and `client.compression_params.level`, refer to the [upstream documentation][confighttp].

[confighttp]: https://github.com/open-telemetry/opentelemetry-collector/blob/<OTEL_VERSION>/config/confighttp/README.md

### `cookies`

The `cookies` block allows the HTTP client to store cookies from server responses and reuse them in subsequent requests.

This could be useful in situations such as load balancers relying on cookies for sticky sessions and enforcing a maximum session age.

The following arguments are supported:

| Name      | Type   | Description                               | Default | Required |
| --------- | ------ | ----------------------------------------- | ------- | -------- |
| `enabled` | `bool` | The target URL to send telemetry data to. | `false` | no       |

### `tls`

The `tls` block configures TLS settings used for the connection to the HTTP server.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to the HTTP server are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures an in-memory buffer of batches before data is sent to the HTTP server.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.otlphttp` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.otlphttp` doesn't expose any component-specific debug information.

## Example

This example creates an exporter to send data to a locally running Grafana Tempo without TLS:

```alloy
otelcol.exporter.otlphttp "tempo" {
    client {
        endpoint = "http://tempo:4318"
        tls {
            insecure             = true
            insecure_skip_verify = true
        }
    }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.otlphttp` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

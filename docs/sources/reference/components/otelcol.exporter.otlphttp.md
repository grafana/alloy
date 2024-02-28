---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/otelcol.exporter.otlphttp/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/otelcol.exporter.otlphttp/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/otelcol.exporter.otlphttp/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/otelcol.exporter.otlphttp/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/otelcol.exporter.otlphttp/
description: Learn about otelcol.exporter.otlphttp
title: otelcol.exporter.otlphttp
---

# otelcol.exporter.otlphttp

`otelcol.exporter.otlphttp` accepts telemetry data from other `otelcol`
components and writes them over the network using the OTLP HTTP protocol.

> **NOTE**: `otelcol.exporter.otlphttp` is a wrapper over the upstream
> OpenTelemetry Collector `otlphttp` exporter. Bug reports or feature requests
> will be redirected to the upstream repository, if necessary.

Multiple `otelcol.exporter.otlphttp` components can be specified by giving them
different labels.

## Usage

```river
otelcol.exporter.otlphttp "LABEL" {
  client {
    endpoint = "HOST:PORT"
  }
}
```

## Arguments

`otelcol.exporter.otlphttp` supports the following arguments:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`metrics_endpoint` | `string` | The endpoint to send metrics to. | `client.endpoint + "/v1/metrics"` | no
`logs_endpoint`    | `string` | The endpoint to send logs to.    | `client.endpoint + "/v1/logs"`    | no
`traces_endpoint`  | `string` | The endpoint to send traces to.  | `client.endpoint + "/v1/traces"`  | no

The default value depends on the `endpoint` field set in the required `client`
block. If set, these arguments override the `client.endpoint` field for the
corresponding signal.

## Blocks

The following blocks are supported inside the definition of
`otelcol.exporter.otlphttp`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
client           | [client][] | Configures the HTTP server to send telemetry data to. | yes
client > tls     | [tls][] | Configures TLS for the HTTP client. | no
sending_queue    | [sending_queue][] | Configures batching of data before sending. | no
retry_on_failure | [retry_on_failure][] | Configures retry mechanism for failed requests. | no
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

The `>` symbol indicates deeper levels of nesting. For example, `client > tls`
refers to a `tls` block defined inside a `client` block.

[client]: #client-block
[tls]: #tls-block
[sending_queue]: #sending_queue-block
[retry_on_failure]: #retry_on_failure-block
[debug_metrics]: #debug_metrics-block

### client block

The `client` block configures the HTTP client used by the component.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`endpoint`           | `string`      | The target URL to send telemetry data to. | | yes
`read_buffer_size`   | `string`      | Size of the read buffer the HTTP client uses for reading server responses. | `0` | no
`write_buffer_size`  | `string`      | Size of the write buffer the HTTP client uses for writing requests. | `"512KiB"` | no
`timeout`            | `duration`    | Time to wait before marking a request as failed. | `"30s"` | no
`headers`            | `map(string)` | Additional headers to send with the request. | `{}` | no
`compression`        | `string`      | Compression mechanism to use for requests. | `"gzip"` | no
`max_idle_conns`     | `int`         | Limits the number of idle HTTP connections the client can keep open. | `100` | no
`max_idle_conns_per_host` | `int`    | Limits the number of idle HTTP connections the host can keep open. | `0` | no
`max_conns_per_host` | `int`         | Limits the total (dialing,active, and idle) number of connections per host. | `0` | no
`idle_conn_timeout`  | `duration`    | Time to wait before an idle connection closes itself. | `"90s"` | no
`disable_keep_alives`| `bool`        | Disable HTTP keep-alive. | `false` | no
`auth`               | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests. | | no

Setting `disable_keep_alives` to `true` will result in significant overhead establishing a new HTTP(s) connection for every request.
Before enabling this option, consider whether changes to idle connection settings can achieve your goal.

{{< docs/shared lookup="flow/reference/components/otelcol-compression-field.md" source="agent" version="<AGENT_VERSION>" >}}

### tls block

The `tls` block configures TLS settings used for the connection to the HTTP
server.

{{< docs/shared lookup="flow/reference/components/otelcol-tls-config-block.md" source="agent" version="<AGENT_VERSION>" >}}

### sending_queue block

The `sending_queue` block configures an in-memory buffer of batches before data is sent
to the HTTP server.

{{< docs/shared lookup="flow/reference/components/otelcol-queue-block.md" source="agent" version="<AGENT_VERSION>" >}}

### retry_on_failure block

The `retry_on_failure` block configures how failed requests to the HTTP server are
retried.

{{< docs/shared lookup="flow/reference/components/otelcol-retry-block.md" source="agent" version="<AGENT_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="flow/reference/components/otelcol-debug-metrics-block.md" source="agent" version="<AGENT_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics,
logs, or traces).

## Component health

`otelcol.exporter.otlphttp` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.exporter.otlphttp` does not expose any component-specific debug
information.

## Example

This example creates an exporter to send data to a locally running Grafana
Tempo without TLS:

```river
otelcol.exporter.otlphttp "tempo" {
    client {
        endpoint = "http://tempo:4317"
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

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
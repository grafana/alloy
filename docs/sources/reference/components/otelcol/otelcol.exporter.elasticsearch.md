---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.elasticsearch/
description: Learn about otelcol.exporter.elasticsearch
labels:
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: otelcol.exporter.elasticsearch
---

# `otelcol.exporter.elasticsearch`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.elasticsearch` accepts logs, metrics, and traces telemetry data from other `otelcol` components and sends it to Elasticsearch using the Elasticsearch bulk API.

{{< admonition type="note" >}}
`otelcol.exporter.elasticsearch` is a wrapper over the upstream OpenTelemetry Collector [`elasticsearch`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`elasticsearch`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/elasticsearchexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.elasticsearch` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.elasticsearch "<LABEL>" {
    endpoints = ["http://localhost:9200"]

    authentication {
        user     = "elastic"
        password = "changeme"
    }
}
```

Exactly one of `endpoints`, `cloudid`, or `client.endpoint` must be specified.

## Arguments

The following arguments are supported:

| Name                      | Type           | Description                                                                                                | Default | Required |
|---------------------------|----------------|------------------------------------------------------------------------------------------------------------|---------|----------|
| `endpoints`               | `list(string)` | List of Elasticsearch URLs to send events to.                                                              | `[]`    | no       |
| `cloudid`                 | `string`       | Elastic Cloud cluster ID. Mutually exclusive with `endpoints` and `client.endpoint`.                       | `""`    | no       |
| `num_workers`             | `int`          | (Deprecated upstream) Number of workers publishing bulk requests. Prefer `sending_queue.num_consumers`.    | `0`     | no       |
| `logs_index`              | `string`       | Static index used for routing logs. Leave empty to use dynamic routing.                                    | `""`    | no       |
| `metrics_index`           | `string`       | Static index used for routing metrics. Leave empty to use dynamic routing.                                 | `""`    | no       |
| `traces_index`            | `string`       | Static index used for routing traces. Leave empty to use dynamic routing.                                  | `""`    | no       |
| `pipeline`                | `string`       | Default ingest node pipeline used for processing events.                                                   | `""`    | no       |
| `include_source_on_error` | `bool`         | Include part of the source document in bulk error responses. Requires Elasticsearch 8.18+.                 | `null`  | no       |
| `metadata_keys`           | `list(string)` | Client metadata keys used as partition keys when batching is enabled. Keys are normalized to lower case.   | `[]`    | no       |

## Blocks

You can use the following blocks with `otelcol.exporter.elasticsearch`:

{{< docs/alloy-config >}}

| Block                                              | Description                                                                              | Required |
|----------------------------------------------------|------------------------------------------------------------------------------------------|----------|
| [`client`][client]                                 | Configures the HTTP client used to send data to Elasticsearch.                           | no       |
| `client` > [`compression_params`][compression]     | Configures the HTTP client compression parameters.                                       | no       |
| `client` > [`cookies`][cookies]                    | Configures cookie handling for the HTTP client.                                          | no       |
| `client` > [`tls`][tls]                            | Configures TLS for the HTTP client.                                                      | no       |
| [`authentication`][authentication]                 | Configures Elasticsearch basic-auth or API-key authentication.                           | no       |
| [`debug_metrics`][debug_metrics]                   | Configures the metrics that this component generates to monitor its state.               | no       |
| [`discover`][discover]                             | Configures Elasticsearch node discovery (sniffing).                                      | no       |
| [`flush`][flush]                                   | (Deprecated upstream) Configures the bulk-request flush settings.                        | no       |
| [`logs_dynamic_id`][logs_dynamic_id]               | Use the `elasticsearch.document_id` log record attribute as the document ID.             | no       |
| [`logs_dynamic_index`][logs_dynamic_index]         | (Deprecated upstream) Enables dynamic index routing for logs.                            | no       |
| [`logs_dynamic_pipeline`][logs_dynamic_pipeline]   | Use the `elasticsearch.document_pipeline` log record attribute as the ingest pipeline.   | no       |
| [`logstash_format`][logstash_format]               | Configures Logstash-compatible index naming.                                             | no       |
| [`mapping`][mapping]                               | Configures Elasticsearch document mapping modes.                                         | no       |
| [`metrics_dynamic_index`][metrics_dynamic_index]   | (Deprecated upstream) Enables dynamic index routing for metrics.                         | no       |
| [`retry`][retry]                                   | Configures retry behavior for failed bulk requests.                                      | no       |
| [`sending_queue`][sending_queue]                   | Configures batching of data before sending.                                              | no       |
| `sending_queue` > [`batch`][batch]                 | Configures batching requests based on a timeout and a minimum number of items.           | no       |
| [`telemetry`][telemetry]                           | Configures experimental request/response logging for debugging.                          | no       |
| [`traces_dynamic_id`][traces_dynamic_id]           | Use the `elasticsearch.document_id` span attribute as the document ID.                   | no       |
| [`traces_dynamic_index`][traces_dynamic_index]     | (Deprecated upstream) Enables dynamic index routing for traces.                          | no       |

[client]: #client
[compression]: #compression_params
[cookies]: #cookies
[tls]: #tls
[authentication]: #authentication
[debug_metrics]: #debug_metrics
[discover]: #discover
[flush]: #flush
[logs_dynamic_id]: #logs_dynamic_id
[logs_dynamic_index]: #logs_dynamic_index
[logs_dynamic_pipeline]: #logs_dynamic_pipeline
[logstash_format]: #logstash_format
[mapping]: #mapping
[metrics_dynamic_index]: #metrics_dynamic_index
[retry]: #retry
[sending_queue]: #sending_queue
[batch]: #batch
[telemetry]: #telemetry
[traces_dynamic_id]: #traces_dynamic_id
[traces_dynamic_index]: #traces_dynamic_index

{{< /docs/alloy-config >}}

### `client`

The `client` block configures the HTTP client used by the component.

The following arguments are supported:

| Name                      | Type                | Description                                                                                       | Default  | Required |
|---------------------------|---------------------|---------------------------------------------------------------------------------------------------|----------|----------|
| `auth`                    | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests.               | `null`   | no       |
| `compression`             | `string`            | Compression algorithm to use when sending bulk requests. Either `gzip` or `none`.                 | `"gzip"` | no       |
| `disable_keep_alives`     | `bool`              | Disable HTTP keep-alive.                                                                          | `false`  | no       |
| `endpoint`                | `string`            | Single Elasticsearch URL. Mutually exclusive with `endpoints` and `cloudid`.                      | `""`     | no       |
| `force_attempt_http2`     | `bool`              | Force the HTTP client to try the HTTP/2 protocol.                                                 | `true`   | no       |
| `headers`                 | `map(string)`       | Additional HTTP headers sent on every bulk request.                                               | `{}`     | no       |
| `http2_ping_timeout`      | `duration`          | Time to wait for an HTTP/2 ping ack before closing the connection.                                | `"0s"`   | no       |
| `http2_read_idle_timeout` | `duration`          | Time after which an HTTP/2 health check is performed if no frame is received.                     | `"0s"`   | no       |
| `idle_conn_timeout`       | `duration`          | Time to wait before an idle connection closes itself.                                             | `"0s"`   | no       |
| `max_conns_per_host`      | `int`               | Limits the total number of connections per host. Zero means no limit.                             | `0`      | no       |
| `max_idle_conns_per_host` | `int`               | Limits the number of idle HTTP connections the host can keep open.                                | `0`      | no       |
| `max_idle_conns`          | `int`               | Limits the number of idle HTTP connections the client can keep open.                              | `0`      | no       |
| `proxy_url`               | `string`            | Proxy URL the HTTP client uses for outbound requests.                                             | `""`     | no       |
| `read_buffer_size`        | `string`            | Size of the read buffer the HTTP client uses for reading server responses.                        | `"0"`    | no       |
| `timeout`                 | `duration`          | Time to wait before marking a request as failed.                                                  | `"90s"`  | no       |
| `write_buffer_size`       | `string`            | Size of the write buffer the HTTP client uses for writing requests.                               | `"0"`    | no       |

#### `compression_params`

The `compression_params` block configures the HTTP compression parameters.

| Name    | Type  | Description                                                                                                                   | Default | Required |
|---------|-------|-------------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `level` | `int` | The compression level. For example, with gzip valid values are `-1` (default), `0` (none), `1` (best speed), `9` (best size). |         | yes      |

#### `cookies`

The `cookies` block configures cookie handling for the HTTP client.

| Name      | Type   | Description                                            | Default | Required |
|-----------|--------|--------------------------------------------------------|---------|----------|
| `enabled` | `bool` | Whether the HTTP client should persist server cookies. | `false` | no       |

#### `tls`

The `tls` block configures TLS for the HTTP client.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `authentication`

The `authentication` block configures Elasticsearch credentials.

| Name       | Type     | Description                                                  | Default | Required |
|------------|----------|--------------------------------------------------------------|---------|----------|
| `user`     | `string` | Username for HTTP Basic Authentication.                      | `""`    | no       |
| `password` | `secret` | Password for HTTP Basic Authentication.                      | `""`    | no       |
| `api_key`  | `secret` | Base64-encoded Elasticsearch API key, used instead of basic. | `""`    | no       |

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `discover`

The `discover` block configures Elasticsearch node discovery.
Don't enable discovery when operating Elasticsearch behind a proxy or load balancer.

| Name       | Type       | Description                                                                                       | Default | Required |
|------------|------------|---------------------------------------------------------------------------------------------------|---------|----------|
| `on_start` | `bool`     | Look for available Elasticsearch nodes the first time the exporter connects.                      | `false` | no       |
| `interval` | `duration` | Refresh the list of Elasticsearch URLs at the given interval. URLs aren't refreshed when `<= 0s`. | `"0s"`  | no       |

### `flush`

{{< admonition type="warning" >}}
The `flush` block is deprecated upstream. Use the `sending_queue` > `batch` block instead.
{{< /admonition >}}

| Name       | Type       | Description                                              | Default | Required |
|------------|------------|----------------------------------------------------------|---------|----------|
| `bytes`    | `int`      | Send buffer flushing limit in bytes.                     | `0`     | no       |
| `interval` | `duration` | Maximum age of a document in the send buffer.            | `"0s"`  | no       |

### `logs_dynamic_id`

| Name      | Type   | Description                                                                | Default | Required |
|-----------|--------|----------------------------------------------------------------------------|---------|----------|
| `enabled` | `bool` | When `true`, use the `elasticsearch.document_id` log attribute as the ID.  | `false` | no       |

### `logs_dynamic_index`

{{< admonition type="warning" >}}
The `logs_dynamic_index` block is deprecated upstream. Dynamic document routing is now always enabled.
{{< /admonition >}}

| Name      | Type   | Description              | Default | Required |
|-----------|--------|--------------------------|---------|----------|
| `enabled` | `bool` | Enable dynamic routing.  | `false` | no       |

### `logs_dynamic_pipeline`

| Name      | Type   | Description                                                                              | Default | Required |
|-----------|--------|------------------------------------------------------------------------------------------|---------|----------|
| `enabled` | `bool` | When `true`, use the `elasticsearch.document_pipeline` log attribute as the pipeline.    | `false` | no       |

### `logstash_format`

The `logstash_format` block enables Logstash-compatible index naming. The resolved index name is
`<index>-<date>`, for example `logs-2024.01.31`.

| Name               | Type     | Description                                          | Default       | Required |
|--------------------|----------|------------------------------------------------------|---------------|----------|
| `enabled`          | `bool`   | Enable Logstash-style index naming.                  | `false`       | no       |
| `prefix_separator` | `string` | Separator between the index prefix and the date.     | `"-"`         | no       |
| `date_format`      | `string` | strftime-style format string for the date suffix.    | `"%Y.%m.%d"`  | no       |

### `mapping`

The `mapping` block configures Elasticsearch document mapping.

| Name            | Type           | Description                                                                                                                                                | Default                                       | Required |
|-----------------|----------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------|----------|
| `mode`          | `string`       | (Deprecated upstream — ignored) The default mapping mode.                                                                                                  | `"otel"`                                      | no       |
| `allowed_modes` | `list(string)` | Allowed document mapping modes that can be selected through the `X-Elastic-Mapping-Mode` client metadata key. Empty list allows all modes.                  | `["bodymap", "ecs", "none", "otel", "raw"]`   | no       |

### `metrics_dynamic_index`

{{< admonition type="warning" >}}
The `metrics_dynamic_index` block is deprecated upstream. Dynamic document routing is now always enabled.
{{< /admonition >}}

| Name      | Type   | Description              | Default | Required |
|-----------|--------|--------------------------|---------|----------|
| `enabled` | `bool` | Enable dynamic routing.  | `false` | no       |

### `retry`

The `retry` block configures the Elasticsearch exporter's retry behavior. Failed sends are retried with exponential backoff.

| Name               | Type        | Description                                                                                            | Default      | Required |
|--------------------|-------------|--------------------------------------------------------------------------------------------------------|--------------|----------|
| `enabled`          | `bool`      | Whether to retry failed requests.                                                                      | `true`       | no       |
| `initial_interval` | `duration`  | Initial waiting time after a failed request.                                                           | `"100ms"`    | no       |
| `max_interval`     | `duration`  | Maximum waiting time between retries.                                                                  | `"1m"`       | no       |
| `max_retries`      | `int`       | Maximum number of retries per request. Zero means use the exporter's default.                          | `0`          | no       |
| `retry_on_status`  | `list(int)` | HTTP status codes that trigger a retry.                                                                | `[429]`      | no       |

### `sending_queue`

The `sending_queue` block configures an in-memory buffer of batches before data is sent to Elasticsearch.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

#### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `telemetry`

The `telemetry` block configures experimental telemetry/debug logging.

{{< admonition type="warning" >}}
Enabling these settings may log request or response bodies, which could expose sensitive data.
Only use them for testing or debugging.
{{< /admonition >}}

| Name                              | Type       | Description                                                          | Default | Required |
|-----------------------------------|------------|----------------------------------------------------------------------|---------|----------|
| `log_request_body`                | `bool`     | Log the body of bulk requests sent to Elasticsearch.                 | `false` | no       |
| `log_response_body`               | `bool`     | Log the body of bulk responses received from Elasticsearch.          | `false` | no       |
| `log_failed_docs_input`           | `bool`     | Log the input of documents that fail to be indexed.                  | `false` | no       |
| `log_failed_docs_input_rate_limit`| `duration` | Minimum interval between consecutive failed-doc log entries.         | `"1s"`  | no       |

### `traces_dynamic_id`

| Name      | Type   | Description                                                                       | Default | Required |
|-----------|--------|-----------------------------------------------------------------------------------|---------|----------|
| `enabled` | `bool` | When `true`, use the `elasticsearch.document_id` span attribute as the ID.        | `false` | no       |

### `traces_dynamic_index`

{{< admonition type="warning" >}}
The `traces_dynamic_index` block is deprecated upstream. Dynamic document routing is now always enabled.
{{< /admonition >}}

| Name      | Type   | Description              | Default | Required |
|-----------|--------|--------------------------|---------|----------|
| `enabled` | `bool` | Enable dynamic routing.  | `false` | no       |

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                 |
|---------|--------------------|-------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.elasticsearch` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.elasticsearch` doesn't expose any component-specific debug information.

## Example

Send logs and traces from an OTLP receiver to a local Elasticsearch cluster, authenticating with basic credentials:

```alloy
otelcol.receiver.otlp "default" {
    http {}

    output {
        logs   = [otelcol.exporter.elasticsearch.default.input]
        traces = [otelcol.exporter.elasticsearch.default.input]
    }
}

otelcol.exporter.elasticsearch "default" {
    endpoints = ["http://localhost:9200"]

    authentication {
        user     = sys.env("ES_USER")
        password = sys.env("ES_PASSWORD")
    }

    logs_index   = "alloy-logs"
    traces_index = "alloy-traces"
}
```

## Compatible components

`otelcol.exporter.elasticsearch` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

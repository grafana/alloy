---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.opensearch/
description: Learn about otelcol.exporter.opensearch
labels:
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: otelcol.exporter.opensearch
---

# `otelcol.exporter.opensearch`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.opensearch` accepts logs and traces telemetry data from other `otelcol` components and sends it to OpenSearch using the bulk API.

{{< admonition type="note" >}}
`otelcol.exporter.opensearch` is a wrapper over the upstream OpenTelemetry Collector [`opensearch`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`opensearch`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/opensearchexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.opensearch` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.opensearch "<LABEL>" {
    client {
        endpoint = "http://localhost:9200"
    }
}
```

## Arguments

The following arguments are supported:

| Name                       | Type       | Description                                                                                                       | Default        | Required |
|----------------------------|------------|-------------------------------------------------------------------------------------------------------------------|----------------|----------|
| `dataset`                  | `string`   | Observability dataset name used in the default index pattern `ss4o_{type}-{dataset}-{namespace}`.                 | `"default"`    | no       |
| `namespace`                | `string`   | Observability namespace used in the default index pattern.                                                        | `"namespace"`  | no       |
| `logs_index`               | `string`   | Index, alias, or data stream name to send logs to. Overrides the default `ss4o_logs-{dataset}-{namespace}`.       | `""`           | no       |
| `logs_index_fallback`      | `string`   | Fallback logs index used when routing fails.                                                                      | `""`           | no       |
| `logs_index_time_format`   | `string`   | Time-format suffix appended to `logs_index` (tokens: `yyyy`, `yy`, `MM`, `dd`, `HH`, `mm`, `ss`).                 | `""`           | no       |
| `traces_index`             | `string`   | Index, alias, or data stream name to send traces to. Overrides the default `ss4o_traces-{dataset}-{namespace}`.   | `""`           | no       |
| `traces_index_fallback`    | `string`   | Fallback traces index used when routing fails.                                                                    | `""`           | no       |
| `traces_index_time_format` | `string`   | Time-format suffix appended to `traces_index`.                                                                    | `""`           | no       |
| `bulk_action`              | `string`   | Bulk action used for ingestion. Must be `"create"` or `"index"`.                                                  | `"create"`     | no       |
| `timeout`                  | `duration` | Time to wait before a bulk request is marked failed. `0s` uses the underlying client default.                     | `"0s"`         | no       |

## Blocks

You can use the following blocks with `otelcol.exporter.opensearch`:

{{< docs/alloy-config >}}

| Block                                              | Description                                                                              | Required |
|----------------------------------------------------|------------------------------------------------------------------------------------------|----------|
| [`client`][client]                                 | Configures the HTTP client used to send data to OpenSearch.                              | yes      |
| `client` > [`tls`][tls]                            | Configures TLS for the HTTP client.                                                      | no       |
| [`debug_metrics`][debug_metrics]                   | Configures the metrics that this component generates to monitor its state.               | no       |
| [`mapping`][mapping]                               | Configures the document mapping mode and field transforms.                               | no       |
| [`retry_on_failure`][retry_on_failure]             | Configures retry mechanism for failed requests.                                          | no       |
| [`sending_queue`][sending_queue]                   | Configures batching of data before sending.                                              | no       |
| `sending_queue` > [`batch`][batch]                 | Configures batching requests based on a timeout and a minimum number of items.           | no       |

[client]: #client
[tls]: #tls
[debug_metrics]: #debug_metrics
[mapping]: #mapping
[retry_on_failure]: #retry_on_failure
[sending_queue]: #sending_queue
[batch]: #batch

{{< /docs/alloy-config >}}

### `client`

{{< badge text="Required" >}}

The `client` block configures the HTTP client used by the component.

{{< docs/shared lookup="reference/components/otelcol-http-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

#### `tls`

The `tls` block configures TLS for the HTTP client.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `mapping`

The `mapping` block configures the OpenSearch document mapping.

| Name              | Type          | Description                                                                                                                                                                       | Default  | Required |
|-------------------|---------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|----------|
| `mode`            | `string`      | Field mapping standard. One of `"ss4o"` (Simple Schema for Observability), `"ecs"`, `"flatten_attributes"`, or `"bodymap"` (logs only — body is stored verbatim).                  | `"ss4o"` | no       |
| `fields`          | `map(string)` | Additional field mappings applied on top of the selected mode.                                                                                                                    | `{}`     | no       |
| `file`            | `string`      | Path to an external file with additional field mappings.                                                                                                                          | `""`     | no       |
| `timestamp_field` | `string`      | Document field where the record timestamp is stored.                                                                                                                              | `""`     | no       |
| `unix_timestamp`  | `bool`        | Store the timestamp as Unix epoch milliseconds instead of an ISO-8601 string.                                                                                                     | `false`  | no       |
| `dedup`           | `bool`        | Remove duplicate fields before sending.                                                                                                                                           | `false`  | no       |
| `dedot`           | `bool`        | Replace dots in field names with underscores.                                                                                                                                     | `false`  | no       |

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to OpenSearch are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures an in-memory buffer of batches before data is sent to OpenSearch.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

#### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                 |
|---------|--------------------|-------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for logs and traces.

## Component health

`otelcol.exporter.opensearch` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.opensearch` doesn't expose any component-specific debug information.

## Example

Forward OTLP logs and traces from an OTLP receiver to a local OpenSearch cluster:

```alloy
otelcol.receiver.otlp "default" {
    http {}
    grpc {}

    output {
        logs   = [otelcol.exporter.opensearch.default.input]
        traces = [otelcol.exporter.opensearch.default.input]
    }
}

otelcol.exporter.opensearch "default" {
    dataset   = "kubernetes"
    namespace = "production"

    client {
        endpoint = "https://opensearch.internal:9200"
        tls {
            insecure_skip_verify = true
        }
    }

    mapping {
        mode = "ss4o"
    }
}
```

With this configuration the exporter writes to data streams matching the SS4O naming convention: `ss4o_logs-kubernetes-production` and `ss4o_traces-kubernetes-production`.

To target a static index instead, set `logs_index` and `traces_index` explicitly.

## Compatible components

`otelcol.exporter.opensearch` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

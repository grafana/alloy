---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.clickhouse/
description: Learn about otelcol.exporter.clickhouse
labels:
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: otelcol.exporter.clickhouse
---

# `otelcol.exporter.clickhouse`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.clickhouse` accepts metrics, logs, and traces telemetry data from other `otelcol` components and sends it to a [ClickHouse](https://clickhouse.com) database.

{{< admonition type="note" >}}
`otelcol.exporter.clickhouse` is a wrapper over the upstream OpenTelemetry Collector [`clickhouse`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`clickhouse`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/clickhouseexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.clickhouse` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.clickhouse "<LABEL>" {
  endpoint = "tcp://127.0.0.1:9000"
}
```

## Arguments

You can use the following arguments with `otelcol.exporter.clickhouse`:

| Name                | Type          | Description                                                 | Default         | Required |
|---------------------|---------------|-------------------------------------------------------------|-----------------|----------|
| `endpoint`          | `string`      | ClickHouse server address.                                  |                 | yes      |
| `async_insert`      | `bool`        | Enable asynchronous inserts.                                | `true`          | no       |
| `cluster_name`      | `string`      | ClickHouse cluster name.                                    | `""`            | no       |
| `compress`          | `string`      | Compression algorithm.                                      | `""`            | no       |
| `connection_params` | `map(string)` | Additional driver connection parameters.                    | `{}`            | no       |
| `create_schema`     | `bool`        | Create the database and tables if they don't exist.         | `true`          | no       |
| `database`          | `string`      | Database name.                                              | `"default"`     | no       |
| `json`              | `bool`        | Store attributes as JSON columns instead of `Map` columns.  | `false`         | no       |
| `logs_table_name`   | `string`      | Name of the table used to store logs.                       | `"otel_logs"`   | no       |
| `password`          | `secret`      | Authentication password.                                    | `""`            | no       |
| `timeout`           | `duration`    | Timeout for every attempt to send data to the backend.      | `"5s"`          | no       |
| `traces_table_name` | `string`      | Name of the table used to store traces.                     | `"otel_traces"` | no       |
| `ttl`               | `duration`    | Data retention period for the tables the component creates. | `"0s"`          | no       |
| `username`          | `string`      | Authentication username.                                    | `""`            | no       |

The `endpoint` argument supports the `tcp://`, `http://`, `https://`, and `clickhouse://` schemes.
To connect to more than one host, separate the addresses with commas, for example, `tcp://addr1:9000,addr2:9000`.

The `compress` argument accepts `none`, `zstd`, `lz4`, `gzip`, `deflate`, `br`, and `true`, which is an alias for `lz4`.
When `compress` is empty, the component uses `lz4`.

The `json` argument stores attributes in the logs and traces tables as JSON columns.
Use ClickHouse v25+ for reliable JSON support.
You might also need to set `enable_json_type=1` in `endpoint` or `connection_params`.

The `create_schema` argument controls whether the component runs data definition language (DDL) statements at startup.
The `cluster_name` and `ttl` arguments only affect those statements, so they do nothing when `create_schema` is `false`.
When you set `cluster_name`, the component appends `ON CLUSTER <name>` to each DDL statement.
Set `ttl` to `0s` to keep data indefinitely.

## Blocks

You can use the following blocks with `otelcol.exporter.clickhouse`:

{{< docs/alloy-config >}}

| Block                                                               | Description                                                                    | Required |
|---------------------------------------------------------------------|--------------------------------------------------------------------------------|----------|
| [`debug_metrics`][debug_metrics]                                    | Configures the metrics that this component generates to monitor its state.     | no       |
| [`metrics_tables`][metrics_tables]                                  | Configures table names for each metric type.                                   | no       |
| `metrics_tables` > [`exponential_histogram`][exponential_histogram] | Configures the table for exponential histogram metrics.                        | no       |
| `metrics_tables` > [`gauge`][gauge]                                 | Configures the table for gauge metrics.                                        | no       |
| `metrics_tables` > [`histogram`][histogram]                         | Configures the table for histogram metrics.                                    | no       |
| `metrics_tables` > [`sum`][sum]                                     | Configures the table for sum metrics.                                          | no       |
| `metrics_tables` > [`summary`][summary]                             | Configures the table for summary metrics.                                      | no       |
| [`retry_on_failure`][retry_on_failure]                              | Configures retry mechanism for failed requests.                                | no       |
| [`sending_queue`][sending_queue]                                    | Configures batching of data before sending.                                    | no       |
| `sending_queue` > [`batch`][batch]                                  | Configures batching requests based on a timeout and a minimum number of items. | no       |
| [`table_engine`][table_engine]                                      | Configures the ClickHouse table engine.                                        | no       |
| [`tls`][tls]                                                        | Configures TLS for the connection.                                             | no       |

[debug_metrics]: #debug_metrics
[metrics_tables]: #metrics_tables
[exponential_histogram]: #exponential_histogram
[gauge]: #gauge
[histogram]: #histogram
[sum]: #sum
[summary]: #summary
[retry_on_failure]: #retry_on_failure
[sending_queue]: #sending_queue
[batch]: #batch
[table_engine]: #table_engine
[tls]: #tls

{{< /docs/alloy-config >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `metrics_tables`

The `metrics_tables` block configures the table names for each OpenTelemetry metric type.
Each metric type has its own sub-block.

### `exponential_histogram`

The `exponential_histogram` block configures the table used to store exponential histogram metrics.

The following arguments are supported:

| Name   | Type     | Description        | Default                                | Required |
|--------|----------|--------------------|----------------------------------------|----------|
| `name` | `string` | Name of the table. | `"otel_metrics_exponential_histogram"` | no       |

### `gauge`

The `gauge` block configures the table used to store gauge metrics.

The following arguments are supported:

| Name   | Type     | Description        | Default                | Required |
|--------|----------|--------------------|------------------------|----------|
| `name` | `string` | Name of the table. | `"otel_metrics_gauge"` | no       |

### `histogram`

The `histogram` block configures the table used to store histogram metrics.

The following arguments are supported:

| Name   | Type     | Description        | Default                    | Required |
|--------|----------|--------------------|----------------------------|----------|
| `name` | `string` | Name of the table. | `"otel_metrics_histogram"` | no       |

### `sum`

The `sum` block configures the table used to store sum metrics.

The following arguments are supported:

| Name   | Type     | Description        | Default              | Required |
|--------|----------|--------------------|----------------------|----------|
| `name` | `string` | Name of the table. | `"otel_metrics_sum"` | no       |

### `summary`

The `summary` block configures the table used to store summary metrics.

The following arguments are supported:

| Name   | Type     | Description        | Default                  | Required |
|--------|----------|--------------------|--------------------------|----------|
| `name` | `string` | Name of the table. | `"otel_metrics_summary"` | no       |

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to ClickHouse are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures an in-memory buffer of batches before data is sent to ClickHouse.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `table_engine`

The `table_engine` block configures the ClickHouse table engine used when `create_schema` is `true`.

The following arguments are supported:

| Name     | Type     | Description                            | Default | Required |
|----------|----------|----------------------------------------|---------|----------|
| `name`   | `string` | ClickHouse table engine name.          | `""`    | no       |
| `params` | `string` | Parameters passed to the table engine. | `""`    | no       |

When `name` is empty, the component uses `MergeTree`.

### `tls`

The `tls` block configures TLS settings used for the connection to ClickHouse.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                 |
|---------|--------------------|-------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.clickhouse` is reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.clickhouse` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.exporter.clickhouse` doesn't expose any component-specific debug metrics.

## Example

```alloy
otelcol.receiver.otlp "default" {
  grpc {}
  output {
    logs    = [otelcol.processor.batch.default.input]
    traces  = [otelcol.processor.batch.default.input]
    metrics = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    logs    = [otelcol.exporter.clickhouse.default.input]
    traces  = [otelcol.exporter.clickhouse.default.input]
    metrics = [otelcol.exporter.clickhouse.default.input]
  }
}

otelcol.exporter.clickhouse "default" {
  endpoint     = "tcp://127.0.0.1:9000"
  database     = "otel"
  async_insert = true
  ttl          = "72h"

  sending_queue {
    num_consumers = 4
    queue_size    = 100
  }

  retry_on_failure {
    enabled          = true
    max_elapsed_time = "5m"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.clickhouse` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.clickhouse/
description: Learn about otelcol.exporter.clickhouse
labels:
  stage: experimental
  products:
    - oss
title: otelcol.exporter.clickhouse
---

# `otelcol.exporter.clickhouse`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.clickhouse` accepts logs, metrics, and traces telemetry data from other `otelcol` components and sends it to a [ClickHouse](https://clickhouse.com) database.

{{< admonition type="note" >}}
`otelcol.exporter.clickhouse` is a wrapper over the upstream OpenTelemetry Collector [`clickhouse`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`clickhouse`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/clickhouseexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.clickhouse` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.clickhouse "LABEL" {
  endpoint = "tcp://127.0.0.1:9000"
}
```

## Arguments

You can use the following arguments with `otelcol.exporter.clickhouse`:

| Name                | Type            | Description                                                                      | Default         | Required |
|---------------------|-----------------|----------------------------------------------------------------------------------|-----------------|----------|
| `endpoint`          | `string`        | ClickHouse server address. Supports `tcp://`, `http://`, `https://`, and `clickhouse://` schemes. Multiple hosts can be comma-separated (for example, `tcp://addr1:9000,addr2:9000`). | | yes |
| `username`          | `string`        | Authentication username.                                                         | `""`            | no       |
| `password`          | `secret`        | Authentication password.                                                         | `""`            | no       |
| `database`          | `string`        | Database name.                                                                   | `"default"`     | no       |
| `connection_params` | `map(string)`   | Additional driver connection parameters.                                         | `{}`            | no       |
| `logs_table_name`   | `string`   | Name of the table used to store logs.    | `"otel_logs"`   | no |
| `traces_table_name` | `string`   | Name of the table used to store traces.  | `"otel_traces"` | no |
| `ttl`               | `duration` | Data retention period. `0` means no expiration. | `"0s"` | no |
| `cluster_name`        | `string`   | ClickHouse cluster name. When set, `ON CLUSTER <name>` is appended to DDL statements.                                             | `""`              | no |
| `create_schema`       | `bool`     | Automatically create the database and tables if they don't exist.                                                                 | `true`            | no |
| `compress`            | `string`   | Compression algorithm. Supported values: `none`, `zstd`, `lz4`, `gzip`, `deflate`, `br`, `true` (alias for `lz4`).              | `"lz4"`           | no |
| `async_insert`        | `bool`     | Enable asynchronous inserts.                                                                                                      | `true`            | no |
| `timeout`             | `duration` | Timeout for every attempt to send data to the backend.                                                                            | `"5s"`            | no |
| `json`                | `bool`     | Use JSON column type for attributes in logs and traces tables. When `false`, `Map` columns are used. Requires ClickHouse v25+.    | `false`           | no |

## Blocks

You can use the following blocks with `otelcol.exporter.clickhouse`:

{{< docs/alloy-config >}}

| Block                                               | Description                                                                | Required |
|-----------------------------------------------------|----------------------------------------------------------------------------|----------|
| [`table_engine`][te]                                | Configures the ClickHouse table engine.                                    | no       |
| [`metrics_tables`][mt]                              | Configures table names for each metric type.                               | no       |
| `metrics_tables` > [`gauge`][mtc]                   | Table name for gauge metrics.                                              | no       |
| `metrics_tables` > [`sum`][mtc]                     | Table name for sum metrics.                                                | no       |
| `metrics_tables` > [`summary`][mtc]                 | Table name for summary metrics.                                            | no       |
| `metrics_tables` > [`histogram`][mtc]               | Table name for histogram metrics.                                          | no       |
| `metrics_tables` > [`exponential_histogram`][mtc]   | Table name for exponential histogram metrics.                              | no       |
| [`tls`][tls]                                        | Configures TLS for the connection.                                         | no       |
| [`sending_queue`][sq]                               | Configures batching of data before sending.                                | no       |
| [`retry_on_failure`][rof]                           | Configures retry mechanism for failed requests.                            | no       |
| [`debug_metrics`][dm]                               | Configures the metrics that this component generates to monitor its state. | no       |

[te]: #table_engine
[mt]: #metrics_tables
[mtc]: #metrics_tables-sub-blocks
[tls]: #tls
[sq]: #sending_queue
[rof]: #retry_on_failure
[dm]: #debug_metrics

{{< /docs/alloy-config >}}

### `table_engine`

The `table_engine` block configures the ClickHouse table engine used when `create_schema` is `true`.

| Name     | Type     | Description                            | Default       | Required |
|----------|----------|----------------------------------------|---------------|----------|
| `name`   | `string` | ClickHouse table engine name. Defaults to `MergeTree` when empty. | `""` | no       |
| `params` | `string` | Parameters passed to the table engine. | `""`          | no       |

### `metrics_tables`

The `metrics_tables` block configures individual table names for each OTel metric type. Each sub-block (`gauge`, `sum`, `summary`, `histogram`, `exponential_histogram`) accepts the following argument:

| Name   | Type     | Description        | Default                       | Required |
|--------|----------|--------------------|-------------------------------|----------|
| `name` | `string` | Name of the table. | See sub-block defaults below. | no       |

Sub-block defaults:

| Sub-block               | Default table name                   |
|-------------------------|--------------------------------------|
| `gauge`                 | `otel_metrics_gauge`                 |
| `sum`                   | `otel_metrics_sum`                   |
| `summary`               | `otel_metrics_summary`               |
| `histogram`             | `otel_metrics_histogram`             |
| `exponential_histogram` | `otel_metrics_exponential_histogram` |

### `tls`

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `retry_on_failure`

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                    |
|---------|--------------------|----------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data.  |

## Component health

`otelcol.exporter.clickhouse` is reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.clickhouse` does not expose any component-specific debug information.

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

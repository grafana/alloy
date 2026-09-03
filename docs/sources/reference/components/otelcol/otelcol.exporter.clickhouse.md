---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.clickhouse/
description: Learn about otelcol.exporter.clickhouse
aliases:
  - ../otelcol.exporter.clickhouse/ # /docs/alloy/latest/reference/components/otelcol.exporter.clickhouse/
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

`otelcol.exporter.clickhouse` accepts metrics, logs, and traces telemetry data from other `otelcol` components and writes them to a [ClickHouse](https://clickhouse.com/) database over its native TCP protocol.

{{< admonition type="note" >}}
`otelcol.exporter.clickhouse` is a wrapper over the upstream OpenTelemetry Collector [`clickhouse`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`clickhouse`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/clickhouseexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.clickhouse` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.clickhouse "<LABEL>" {
    endpoint = "tcp://<CLICKHOUSE_HOST>:9000"
}
```

## Arguments

You can use the following arguments with `otelcol.exporter.clickhouse`:

| Name                  | Type          | Description                                                                                     | Default            | Required |
|-----------------------|---------------|---------------------------------------------------------------------------------------------------|---------------------|----------|
| `endpoint`            | `string`      | The ClickHouse server DSN, for example `tcp://localhost:9000` or `clickhouse://localhost:9000`. |                     | yes      |
| `async_insert`        | `bool`        | Enable asynchronous inserts. Ignored if async inserts are already configured in `endpoint` or `connection_params`. | `true`              | no       |
| `cluster_name`        | `string`      | If set, appends `ON CLUSTER` with the given name when creating tables.                          | `""`                | no       |
| `compress`            | `string`      | Compression algorithm to use. Valid values are `none`, `zstd`, `lz4`, `gzip`, `deflate`, and `br`. | `"lz4"`             | no       |
| `connection_params`   | `map(string)` | Extra connection parameters, for example `compression` or `dial_timeout`.                       | `{}`                | no       |
| `create_schema`       | `bool`        | Run the DDL to create the database and tables if they don't already exist.                      | `true`              | no       |
| `database`            | `string`      | The database to export to.                                                                       | `"default"`         | no       |
| `logs_table_name`     | `string`      | The table name used for logs.                                                                    | `"otel_logs"`       | no       |
| `metrics_table_name`  | `string`      | The base table name used for metrics, before the per-metric-type suffix.                        | `"otel_metrics"`    | no       |
| `password`            | `secret`      | The password used to authenticate to ClickHouse.                                                | `""`                | no       |
| `timeout`             | `duration`    | The timeout for every attempt to send data to ClickHouse. `0` means no timeout.                 | `"0s"`              | no       |
| `traces_table_name`   | `string`      | The table name used for traces.                                                                 | `"otel_traces"`     | no       |
| `ttl`                 | `duration`    | The time-to-live for inserted data, for example `48h`. `0` means no TTL.                        | `"0s"`              | no       |
| `username`            | `string`      | The username used to authenticate to ClickHouse.                                                | `""`                | no       |

If `username` is set, it overrides any username in `endpoint`.
If `password` is set, it overrides any password in `endpoint`.

## Blocks

You can use the following blocks with `otelcol.exporter.clickhouse`:

{{< docs/alloy-config >}}

| Block                                   | Description                                                                    | Required |
|------------------------------------------|--------------------------------------------------------------------------------|----------|
| [`debug_metrics`][debug_metrics]         | Configures the metrics that this component generates to monitor its state.     | no       |
| [`metrics_tables`][metrics_tables]       | Configures per-metric-type table name overrides.                              | no       |
| metrics_tables > [`gauge`][metric_type]  | Table name override for gauge metrics.                                        | no       |
| metrics_tables > [`sum`][metric_type]    | Table name override for sum metrics.                                          | no       |
| metrics_tables > [`summary`][metric_type]| Table name override for summary metrics.                                      | no       |
| metrics_tables > [`histogram`][metric_type] | Table name override for histogram metrics.                                 | no       |
| metrics_tables > [`exponential_histogram`][metric_type] | Table name override for exponential histogram metrics.         | no       |
| [`retry_on_failure`][retry_on_failure]   | Configures retry mechanism for failed requests.                               | no       |
| [`sending_queue`][queue]                 | Configures batching of data before sending.                                   | no       |
| `sending_queue` > [`batch`][batch]       | Configures batching requests based on a timeout and a minimum number of items.| no       |
| [`table_engine`][table_engine]           | Configures the ClickHouse table `ENGINE` clause used when creating tables.     | no       |
| [`tls`][tls]                             | Configures TLS for the connection to ClickHouse.                              | no       |

[batch]: #batch
[debug_metrics]: #debug_metrics
[metric_type]: #gauge-sum-summary-histogram-and-exponential_histogram
[metrics_tables]: #metrics_tables
[queue]: #sending_queue
[retry_on_failure]: #retry_on_failure
[table_engine]: #table_engine
[tls]: #tls

{{< /docs/alloy-config >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `metrics_tables`

The `metrics_tables` block configures table name overrides for each metric type.
If a sub-block is omitted, its table name defaults to `metrics_table_name` plus a type-specific suffix, for example `otel_metrics_gauge`.

### `gauge`, `sum`, `summary`, `histogram`, and `exponential_histogram`

Each of these blocks accepts a single argument:

| Name   | Type     | Description                          | Default | Required |
|--------|----------|---------------------------------------|---------|----------|
| `name` | `string` | The table name for this metric type. |         | no       |

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to ClickHouse are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures queueing and batching for the exporter.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `table_engine`

The `table_engine` block configures the `ENGINE` clause used when `create_schema` creates ClickHouse tables.

| Name     | Type     | Description                                     | Default       | Required |
|----------|----------|--------------------------------------------------|---------------|----------|
| `name`   | `string` | The table engine name, for example `MergeTree` or `ReplicatedMergeTree`. | `"MergeTree"` | no       |
| `params` | `string` | Raw parameters passed to the engine, for example replication path and replica name arguments. | `""`          | no       |

### `tls`

The `tls` block configures TLS for the connection to ClickHouse.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                 |
|---------|--------------------|-------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.clickhouse` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.clickhouse` doesn't expose any component-specific debug information.

## Example

This example scrapes Prometheus metrics and exports them to ClickHouse, using a non-default database and a one-week TTL:

```alloy
prometheus.exporter.self "default" {
}

prometheus.scrape "metamonitoring" {
  targets    = prometheus.exporter.self.default.targets
  forward_to = [otelcol.receiver.prometheus.default.receiver]
}

otelcol.receiver.prometheus "default" {
  output {
    metrics = [otelcol.exporter.clickhouse.default.input]
  }
}

otelcol.exporter.clickhouse "default" {
    endpoint = "tcp://clickhouse.clickhouse.svc.cluster.local:9000"
    username = "default"
    password = "<CLICKHOUSE_PASSWORD>"
    database = "metrics"
    ttl      = "168h"
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

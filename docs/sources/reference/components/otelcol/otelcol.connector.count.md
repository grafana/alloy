---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.connector.count/
aliases:
  - ../otelcol.connector.count/ # /docs/alloy/latest/reference/components/otelcol.connector.count/
description: Learn about otelcol.connector.count
labels:
  stage: experimental
  products:
    - oss
title: otelcol.connector.count
---

# `otelcol.connector.count`

`otelcol.connector.count` accepts spans, span events, metrics, data points, and log records from other `otelcol` components and generates metrics that count the received telemetry data.

{{< admonition type="note" >}}
`otelcol.connector.count` is a wrapper over the upstream OpenTelemetry Collector Contrib `count` connector.
Bug reports or feature requests are redirected to the upstream repository if necessary.
{{< /admonition >}}

You can specify multiple `otelcol.connector.count` components by giving them different labels.

## Usage

```alloy
otelcol.connector.count "<LABEL>" {
  output {
    metrics = [...]
  }
}
```

## Arguments

`otelcol.connector.count` doesn't support any arguments and can be configured using blocks only.

## Blocks

You can use the following blocks with `otelcol.connector.count`:

| Block                            | Description                                                                              | Required |
| -------------------------------- | ---------------------------------------------------------------------------------------- | -------- |
| [`spans`][spans]                 | Configures custom metrics for counting spans.                                            | no       |
| [`spans` > `attributes`][attributes] | Groups span counts by attribute values.                                              | no       |
| [`spanevents`][spanevents]       | Configures custom metrics for counting span events.                                      | no       |
| [`spanevents` > `attributes`][attributes] | Groups span event counts by attribute values.                                   | no       |
| [`metrics`][metrics]             | Configures custom metrics for counting metrics.                                          | no       |
| [`metrics` > `attributes`][attributes] | Groups metric counts by attribute values.                                          | no       |
| [`datapoints`][datapoints]       | Configures custom metrics for counting data points.                                      | no       |
| [`datapoints` > `attributes`][attributes] | Groups data point counts by attribute values.                                     | no       |
| [`logs`][logs]                   | Configures custom metrics for counting log records.                                      | no       |
| [`logs` > `attributes`][attributes] | Groups log counts by attribute values.                                                | no       |
| [`output`][output]               | Configures where to send received telemetry data.                                        | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state.               | no       |

The `>` symbol indicates deeper levels of nesting.
For example, `spans` > `attributes` refers to an `attributes` block defined inside a `spans` block.

[spans]: #spans
[spanevents]: #spanevents
[metrics]: #metrics
[datapoints]: #datapoints
[logs]: #logs
[attributes]: #attributes
[output]: #output
[debug_metrics]: #debug_metrics

### `spans`

The `spans` block configures a custom metric for counting spans.

You can specify multiple `spans` blocks to define different metrics.

If no `spans` blocks are defined, the connector emits a default metric named `trace.span.count`.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`name` | `string` | Metric name. | Uses default name based on telemetry type. | no
`description` | `string` | Metric description. | `""` | no
`conditions` | `list(string)` | OTTL expressions for filtering spans. | `[]` | no

The `conditions` argument accepts a list of [OTTL][] expressions that filter which spans to count.
If any condition matches, the span is counted.
If no conditions are specified, all spans are counted.

[OTTL]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl

### `spanevents`

The `spanevents` block configures a custom metric for counting span events.

You can specify multiple `spanevents` blocks to define different metrics.

If no `spanevents` blocks are defined, the connector emits a default metric named `trace.span.event.count`.

This block shares the same configuration structure as [`spans`][spans].
Refer to the [`spans`][spans] block documentation for the complete list of supported arguments and blocks.

### `metrics`

The `metrics` block configures a custom metric for counting metrics.

You can specify multiple `metrics` blocks to define different metrics.

If no `metrics` blocks are defined, the connector emits a default metric named `metric.count`.

This block shares the same configuration structure as [`spans`][spans].
Refer to the [`spans`][spans] block documentation for the complete list of supported arguments and blocks.

### `datapoints`

The `datapoints` block configures a custom metric for counting data points.

You can specify multiple `datapoints` blocks to define different metrics.

If no `datapoints` blocks are defined, the connector emits a default metric named `metric.datapoint.count`.

This block shares the same configuration structure as [`spans`][spans].
Refer to the [`spans`][spans] block documentation for the complete list of supported arguments and blocks.

### `logs`

The `logs` block configures a custom metric for counting log records.

You can specify multiple `logs` blocks to define different metrics.

If no `logs` blocks are defined, the connector emits a default metric named `log.record.count`.

This block shares the same configuration structure as [`spans`][spans].
Refer to the [`spans`][spans] block documentation for the complete list of supported arguments and blocks.

### `attributes`

The `attributes` block specifies an attribute to use for grouping counted telemetry data.

Each unique combination of attribute values generates a separate data point on the metric.

You can specify multiple `attributes` blocks within `spans`, `spanevents`, `metrics`, `datapoints`, or `logs` blocks to group by multiple attributes.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`key` | `string` | Attribute key name. | | yes
`default_value` | `any` | Default value if the attribute doesn't exist. | | no

Attribute precedence: span/log/metric attributes > scope attributes > resource attributes.

### `output`

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for traces, metrics, and logs.

All telemetry received through `input` is counted according to the configured blocks and emitted as metrics to the components specified in `output`.

## Component health

`otelcol.connector.count` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.connector.count` doesn't expose any component-specific debug information.

## Examples

### Default configuration

Use the count connector with minimal configuration to count all telemetry data using default metric names.

```alloy
otelcol.connector.count "default" {
  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}
```

This configuration generates the following default metrics:

- `trace.span.count` - Count of all spans
- `trace.span.event.count` - Count of all span events
- `metric.count` - Count of all metrics
- `metric.datapoint.count` - Count of all data points
- `log.record.count` - Count of all log records

### Custom metrics with conditions and attributes

Create custom count metrics with filtering conditions and group counts by specific attributes.

```alloy
otelcol.connector.count "default" {
  // Count only HTTP GET spans
  spans {
    name        = "http_get_requests"
    description = "Number of HTTP GET requests"
    conditions  = [
      "attributes[\"http.method\"] == \"GET\"",
    ]
  }

  // Count spans grouped by service and environment
  spans {
    name        = "spans_by_service"
    description = "Spans per service and environment"
    attributes {
      key           = "service.name"
      default_value = "unknown"
    }
    attributes {
      key           = "env"
      default_value = "production"
    }
  }

  // Count error and fatal logs only
  logs {
    name        = "error_logs"
    description = "Error and fatal log records"
    conditions  = [
      "severity_number >= 17",
    ]
  }

  // Count logs by environment
  logs {
    name        = "logs_by_env"
    description = "Log records per environment"
    attributes {
      key = "env"
    }
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}
```

### Complete pipeline with Prometheus export

Count spans, logs, and metrics, then export to Prometheus using the delta to cumulative processor.

```alloy
otelcol.receiver.otlp "default" {
  grpc {
    endpoint = "127.0.0.1:4317"
  }

  http {
    endpoint = "127.0.0.1:4318"
  }

  output {
    traces  = [otelcol.connector.count.default.input]
    metrics = [otelcol.connector.count.default.input]
    logs    = [otelcol.connector.count.default.input]
  }
}

otelcol.connector.count "default" {
  spans {
    name        = "traces_total"
    description = "Total number of spans received"
  }

  spans {
    name        = "http_get_spans"
    description = "HTTP GET requests"
    conditions  = [
      "attributes[\"http.method\"] == \"GET\"",
    ]
  }

  spans {
    name        = "spans_by_service"
    description = "Spans grouped by service"
    attributes {
      key           = "service.name"
      default_value = "unknown"
    }
  }

  logs {
    name        = "logs_total"
    description = "Total number of logs received"
  }

  logs {
    name        = "error_logs"
    description = "Error level logs"
    conditions  = [
      "severity_number >= 17",
    ]
  }

  metrics {
    name        = "metrics_total"
    description = "Total number of metrics received"
  }

  output {
    metrics = [otelcol.processor.deltatocumulative.default.input]
  }
}

// Convert delta metrics to cumulative for Prometheus compatibility
otelcol.processor.deltatocumulative "default" {
  output {
    metrics = [otelcol.exporter.prometheus.default.input]
  }
}

otelcol.exporter.prometheus "default" {
  forward_to                       = [prometheus.remote_write.default.receiver]
  add_metric_suffixes              = false
  resource_to_telemetry_conversion = true
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://localhost:9090/api/v1/write"
  }
}
```

## Technical details

`otelcol.connector.count` uses the count connector from OpenTelemetry Collector Contrib.

All generated metrics use the Sum data type with Delta aggregation temporality.

{{< admonition type="note" >}}
Prometheus doesn't natively support delta metrics.
Use [`otelcol.processor.deltatocumulative`][otelcol.processor.deltatocumulative] to convert delta metrics to cumulative before sending to Prometheus.

[otelcol.processor.deltatocumulative]: ../otelcol.processor.deltatocumulative/
{{< /admonition >}}

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.connector.count` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.connector.count` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

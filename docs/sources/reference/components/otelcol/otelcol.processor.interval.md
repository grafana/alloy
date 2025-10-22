---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.interval/
description: Learn about otelcol.processor.interval
labels:
  stage: experimental
  products:
    - oss
title: otelcol.processor.interval
---

# `otelcol.processor.interval`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.processor.interval` aggregates metrics and periodically forwards the latest values to the next component in the pipeline.
The processor supports aggregating the following metric types:

* Monotonically increasing, cumulative sums
* Monotonically increasing, cumulative histograms
* Monotonically increasing, cumulative exponential histograms
* Gauges
* Summaries

The following metric types won't be aggregated and will instead be passed, unchanged, to the next component in the pipeline:

* All delta metrics
* Non-monotonically increasing sums

{{< admonition type="note" >}}
Aggregating data over an interval is an inherently lossy process.
You lose precision for monotonically increasing cumulative sums, histograms, and exponential histograms, but you don't lose overall data.
You can lose data when you aggregate non-monotonically increasing sums, gauges, and summaries.
For example, a value can increase and decrease to the original value, and you can lose this change in the aggregation.
In most cases, this type of data loss is acceptable.
However, you can change the configuration so that these changed values pass through and aren't aggregated.
{{< /admonition >}}

{{< admonition type="warning" >}}
After exporting, any internal state is cleared.
If no new metrics come in, the next interval will export nothing.
{{< /admonition >}}

{{< admonition type="note" >}}
`otelcol.processor.interval` is a wrapper over the upstream OpenTelemetry Collector [`interval`][] processor.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`interval`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/intervalprocessor
{{< /admonition >}}

## Usage

```alloy
otelcol.processor.interval "<LABEL>" {
  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following argument with `otelcol.processor.interval`:

| Name       | Type       | Description                                                           | Default | Required |
|------------|------------|-----------------------------------------------------------------------|---------|----------|
| `interval` | `duration` | The interval in which the processor should export aggregated metrics. | `"60s"` | no       |

## Blocks

You can use the following blocks with `otelcol.processor.interval`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |
| [`passthrough`][passthrough]     | Configures metric types to be passed through instead of aggregated.        | no       |

[output]: #output
[debug_metrics]: #debug_metrics
[passthrough]: #passthrough

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `passthrough`

The `passthrough` block configures which metric types should be passed through instead of being aggregated.

The following attributes are supported:

| Name      | Type   | Description                                                                           | Default | Required |
|-----------|--------|---------------------------------------------------------------------------------------|---------|----------|
| `gauge`   | `bool` | Determines whether gauge metrics should be passed through as they're or aggregated.   | `false` | no       |
| `summary` | `bool` | Determines whether summary metrics should be passed through as they're or aggregated. | `false` | no       |

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for metrics.

## Component health

`otelcol.processor.interval` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.processor.interval` doesn't expose any component-specific debug information.

## Example

This example receives OTLP metrics and aggregates them for 30s before sending to the next exporter.

```alloy
otelcol.receiver.otlp "default" {
  grpc { ... }
  http { ... }

  output {
    metrics = [otelcol.processor.interval.default.input]
  }
}

otelcol.processor.interval "default" {
  interval = "30s"
  output {
    metrics = [otelcol.exporter.otlphttp.grafana_cloud.input]
  }
}

otelcol.exporter.otlphttp "grafana_cloud" {
  client {
    endpoint = "https://otlp-gateway-prod-gb-south-0.grafana.net/otlp"
    auth     = otelcol.auth.basic.grafana_cloud.handler
  }
}

otelcol.auth.basic "grafana_cloud" {
  username = env("<GRAFANA_CLOUD_USERNAME>")
  password = env("<GRAFANA_CLOUD_API_KEY>")
}
```

| Timestamp | Metric Name    | Aggregation Temporarility | Attributes          | Value |
|-----------|----------------|---------------------------|---------------------|------:|
| 0         | `test_metric`  | Cumulative                | `labelA: example1`  |   4.0 |
| 2         | `test_metric`  | Cumulative                | `labelA: example2`  |   3.1 |
| 4         | `other_metric` | Delta                     | `fruitType: orange` |  77.4 |
| 6         | `test_metric`  | Cumulative                | `labelA: example1`  |   8.2 |
| 8         | `test_metric`  | Cumulative                | `labelA: example1`  |  12.8 |
| 10        | `test_metric`  | Cumulative                | `labelA: example2`  |   6.4 |

The processor immediately passes the following metric to the next processor in the chain because it's a Delta metric.

| Timestamp | Metric Name    | Aggregation Temporarility | Attributes          | Value |
|-----------|----------------|---------------------------|---------------------|------:|
| 4         | `other_metric` | Delta                     | `fruitType: orange` |  77.4 |

At the next `interval` (15s by default), the processor passed the following metrics to the next processor in the chain.

| Timestamp | Metric Name   | Aggregation Temporarility | Attributes         | Value |
|-----------|---------------|---------------------------|--------------------|------:|
| 8         | `test_metric` | Cumulative                | `labelA: example1` |  12.8 |
| 10        | `test_metric` | Cumulative                | `labelA: example1` |   6.4 |

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.interval` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.interval` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

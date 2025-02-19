---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.interval/
description: Learn about otelcol.processor.interval
title: otelcol.processor.interval
---

<span class="badge docs-labels__stage docs-labels__item">Experimental</span>

# otelcol.processor.interval

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.processor.interval` aggregates metrics and periodically forwards the latest values to the next component in the pipeline.
The processor supports aggregating the following metric types:

- Monotonically increasing, cumulative sums
- Monotonically increasing, cumulative histograms
- Monotonically increasing, cumulative exponential histograms

The following metric types will _not_ be aggregated and will instead be passed, unchanged, to the next component in the pipeline:

- All delta metrics
- Non-monotonically increasing sums
- Gauges
- Summaries

{{< admonition type="warning" >}}
After exporting, any internal state is cleared. If no new metrics come in, the next interval will export nothing.
{{< /admonition >}}

{{< admonition type="note" >}}
`otelcol.processor.interval` is a wrapper over the upstream OpenTelemetry Collector `interval` processor.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

## Usage

```alloy
otelcol.processor.interval "LABEL" {
  output {
    metrics = [...]
  }
}
```

## Arguments

`otelcol.processor.interval` supports the following arguments:

| Name       | Type       | Description                                                         | Default | Required |
| ---------- | ---------- | ------------------------------------------------------------------- | ------- | -------- |
| `interval` | `duration` | The interval in the processor should export the aggregated metrics. | `"60s"` | no       |

## Blocks

The following blocks are supported inside the definition of `otelcol.processor.interval`:

| Hierarchy     | Block             | Description                                                                | Required |
| ------------- | ----------------- | -------------------------------------------------------------------------- | -------- |
| output        | [output][]        | Configures where to send received telemetry data.                          | yes      |
| debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no       |

[output]: #output-block
[debug_metrics]: #debug_metrics-block

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for metrics.

## Component health

`otelcol.processor.interval` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.processor.interval` does not expose any component-specific debug information.

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
  username = env("GRAFANA_CLOUD_USERNAME")
  password = env("GRAFANA_CLOUD_API_KEY")
}
```

| Timestamp | Metric Name  | Aggregation Temporarility | Attributes        | Value |
| --------- | ------------ | ------------------------- | ----------------- | ----: |
| 0         | test_metric  | Cumulative                | labelA: foo       |   4.0 |
| 2         | test_metric  | Cumulative                | labelA: bar       |   3.1 |
| 4         | other_metric | Delta                     | fruitType: orange |  77.4 |
| 6         | test_metric  | Cumulative                | labelA: foo       |   8.2 |
| 8         | test_metric  | Cumulative                | labelA: foo       |  12.8 |
| 10        | test_metric  | Cumulative                | labelA: bar       |   6.4 |

The processor immediately passes the following metric to the next processor in the chain because it is a Delta metric.

| Timestamp | Metric Name  | Aggregation Temporarility | Attributes        | Value |
| --------- | ------------ | ------------------------- | ----------------- | ----: |
| 4         | other_metric | Delta                     | fruitType: orange |  77.4 |

At the next `interval` (15s by default), the processor passed the following metrics to the next processor in the chain.

| Timestamp | Metric Name | Aggregation Temporarility | Attributes  | Value |
| --------- | ----------- | ------------------------- | ----------- | ----: |
| 8         | test_metric | Cumulative                | labelA: foo |  12.8 |
| 10        | test_metric | Cumulative                | labelA: bar |   6.4 |

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

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.cardinality_guardian/
description: Learn about otelcol.processor.cardinality_guardian
labels:
  stage: experimental
  products:
    - oss
title: otelcol.processor.cardinality_guardian
---

# `otelcol.processor.cardinality_guardian`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.processor.cardinality_guardian` detects metric labels with abnormal cardinality growth and either strips them or tags them for routing, before they reach your TSDB.

The processor measures the *delta* of new unique label values per (metric, label) pair within each epoch, using HyperLogLog++ sketches.
Because detection is based on growth rather than absolute cardinality, stable high-cardinality metrics aren't penalized.

Depending on the configured `enforcement_mode`, the processor handles offending attributes in one of the following ways:

* `tag_only`: Preserves all attributes and injects an `otel.metric.overflow: true` attribute for downstream routing.
  No data is modified.
* `overflow_attribute`: Replaces the high-cardinality attribute value with the sentinel string `otel.cardinality_overflow` and reaggregates data points that now share the same identity.
* `strip_and_reaggregate`: Removes the offending attribute and reaggregates data points that now share the same identity.

Reaggregation is only supported for delta sums and gauges.
Cumulative sums, histograms, exponential histograms, and summaries fall back to `tag_only` behavior with a warning log.

{{< admonition type="note" >}}
`tag_only` doesn't protect your TSDB on its own — high-cardinality labels still reach your backend unchanged.
Pair it with a downstream routing connector to split tagged metrics to cheaper storage.
{{< /admonition >}}

{{< admonition type="note" >}}
`otelcol.processor.cardinality_guardian` is a wrapper over the upstream OpenTelemetry Collector [`cardinality_guardian`][] processor.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`cardinality_guardian`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/cardinalityguardianprocessor
{{< /admonition >}}

## Usage

```alloy
otelcol.processor.cardinality_guardian "<LABEL>" {
  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.processor.cardinality_guardian`:

| Name                                | Type           | Description                                                                                                  | Default                         | Required |
|-------------------------------------|----------------|--------------------------------------------------------------------------------------------------------------|---------------------------------|----------|
| `max_cardinality_delta_per_epoch`   | `int`          | Maximum number of new unique label values allowed per metric and label-key combination within one epoch.     | `100`                           | no       |
| `epoch_duration_seconds`            | `int`          | How often the sliding cardinality window advances, in seconds. Must be at least `10`.                        | `300`                           | no       |
| `never_drop_labels`                 | `list(string)` | Label keys that the processor never strips or tags, regardless of cardinality growth.                        | `["http.status_code", "region"]` | no       |
| `enforcement_mode`                  | `string`       | How to handle high-cardinality attributes: `"tag_only"`, `"overflow_attribute"`, or `"strip_and_reaggregate"`. | `"tag_only"`                    | no       |
| `estimated_cost_per_metric_month`   | `float`        | Theoretical cost per active time series, used to populate the estimated savings metric. `0` disables tracking. | `0.05`                          | no       |
| `top_offenders_count`               | `int`          | Number of highest-delta (metric, label) pairs to report via the top offenders gauge. `0` disables the gauge. | `10`                            | no       |
| `max_tracker_count`                 | `int`          | Maximum number of concurrent (metric, label) tracking sketches across all shards. `0` means unlimited.       | `0`                             | no       |
| `metric_overrides`                  | `map(int)`     | Per-metric cardinality delta limits that override `max_cardinality_delta_per_epoch`.                         | `{}`                            | no       |
| `drop_log_max_per_epoch`            | `int`          | Maximum number of "Dropping high-cardinality attribute" warning logs emitted per epoch. `0` disables the cap. | `10`                            | no       |

## Blocks

You can use the following blocks with `otelcol.processor.cardinality_guardian`:

{{< docs/alloy-config >}}

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[output]: #output
[debug_metrics]: #debug_metrics

{{< /docs/alloy-config >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for metrics.

## Component health

`otelcol.processor.cardinality_guardian` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.processor.cardinality_guardian` doesn't expose any component-specific debug information.

## Example

This example receives OTLP metrics, strips labels whose cardinality grows by more than 100 new unique values per 5-minute epoch, and forwards the result to an OTLP exporter:

```alloy
otelcol.receiver.otlp "default" {
  grpc { ... }
  http { ... }

  output {
    metrics = [otelcol.processor.cardinality_guardian.default.input]
  }
}

otelcol.processor.cardinality_guardian "default" {
  max_cardinality_delta_per_epoch = 100
  epoch_duration_seconds          = 300
  enforcement_mode                = "strip_and_reaggregate"
  never_drop_labels               = ["region", "service.name"]

  metric_overrides = {
    "http.server.request.duration" = 5000,
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = "database:4317"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.cardinality_guardian` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.cardinality_guardian` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

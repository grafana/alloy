---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.cumulativetodelta/
aliases:
  - ../otelcol.processor.cumulativetodelta/ # /docs/alloy/latest/reference/otelcol.processor.cumulativetodelta/
description: Learn about otelcol.processor.cumulativetodelta
labels:
  stage: public-preview
  products:
    - oss
title: otelcol.processor.cumulativetodelta
---

# `otelcol.processor.cumulativetodelta`

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.processor.cumulativetodelta` accepts metrics from other `otelcol` components and converts metrics with the cumulative temporality to delta.

{{< admonition type="note" >}}
`otelcol.processor.cumulativetodelta` is a wrapper over the upstream OpenTelemetry Collector [`cumulativetodelta`][] processor.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`cumulativetodelta`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/cumulativetodeltaprocessor
{{< /admonition >}}

You can specify multiple `otelcol.processor.cumulativetodelta` components by giving them different labels.

## Usage

```alloy
otelcol.processor.cumulativetodelta "<LABEL>" {
  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.processor.cumulativetodelta`:

| Name            | Type       | Description                                                            | Default  | Required |
|-----------------|------------|------------------------------------------------------------------------|----------|----------|
| `initial_value` | `string`   | Handling of the first observed point for a given metric identity.      | `"auto"` | no       |
| `max_staleness` | `duration` | The total time a state entry will live past the time it was last seen. | `"0"`    | no       |

`otelcol.processor.cumulativetodelta` tracks incoming metric streams.
Sum and exponential histogram metrics with delta temporality are tracked and converted into cumulative temporality.

If a new sample hasn't been received since the duration specified by `max_staleness`, tracked streams are considered stale and dropped.
When set to `"0"`, the state is retained indefinitely.

The `initial_value` sets the handling of the first observed point for a given metric identity.
When the collector (re)starts, there's no record of how much of a given cumulative counter has already been converted to delta values.

* `"auto"` (default): Send the observed value if the start time is set AND the start time happens after the component started AND the start time is different from the timestamp.
  This is suitable for gateway deployments.
  This heuristic is like `drop`, but it keeps values for newly started counters which couldn't have had previous observed values.
* `"keep"`: Send the observed value as the delta value. This is suitable for when the incoming metrics haven't been observed before.
  For example, when you are running the collector as a sidecar, the collector lifecycle is tied to the metric source.
* `"drop"`: Keep the observed value but don't send it. This is suitable for gateway deployments.
  It guarantees that all delta counts it produces haven't been observed before, but drops the values between the first two observations.

## Blocks

You can use the following blocks with `otelcol.processor.cumulativetodelta`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |
| [`exclude`][exclude]             | Configures which metrics to not convert to delta.                          | no       |
| [`include`][include]             | Configures which metrics to convert to delta.                              | no       |

If metric matches both `include` and `exclude`, exclude takes preference.
If neither `include` nor `exclude` are supplied, no filtering is applied.

[include]: #include
[exclude]: #exclude
[output]: #output
[debug_metrics]: #debug_metrics

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `include`

The `include` block configures which metrics to convert to delta.

The following attributes are supported:

| Name           | Type           | Description                              | Default | Required |
|----------------|----------------|------------------------------------------|---------|----------|
| `match_type`   | `string`       | Match type to use, `strict` or `regexp`. |         | no       |
| `metric_types` | `list(string)` | Metric types to convert to delta.        |         | no       |
| `metrics`      | `list(string)` | Names or patterns to convert to delta.   |         | no       |

If one of `metrics` or `match_type` is supplied, the other must be supplied too.

Valid values for `metric_types` are `sum` and `histogram`.

### `exclude`

The `exclude` block configures which metrics not to convert to delta.
`exclude` takes precedence over `include`

The following attributes are supported:

| Name           | Type           | Description                                            | Default | Required |
|----------------|----------------|--------------------------------------------------------|---------|----------|
| `match_type`   | `string`       | Match type to use, `strict` or `regexp`.               |         | no       |
| `metric_types` | `list(string)` | Metric types to exclude when converting to delta.      |         | no       |
| `metrics`      | `list(string)` | Names or patterns to exclude when converting to delta. |         | no       |

If one of `metrics` or `match_type` is supplied, the other must be supplied too.

Valid values for `metric_types` are `sum` and `histogram`.

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for metrics.

## Component health

`otelcol.processor.cumulativetodelta` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.processor.cumulativetodelta` doesn't expose any component-specific debug information.

## Example

This example converts cumulative temporality metrics to delta before sending it to [`otelcol.exporter.otlp`][otelcol.exporter.otlp] for further processing.

```alloy
otelcol.processor.cumulativetodelta "default" {
  output {
    metrics = [otelcol.exporter.otlp.production.input]
  }
}

otelcol.exporter.otlp "production" {
  client {
    endpoint = sys.env("OTLP_SERVER_ENDPOINT")
  }
}
```

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.cumulativetodelta` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.cumulativetodelta` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

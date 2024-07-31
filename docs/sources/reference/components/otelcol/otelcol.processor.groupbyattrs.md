---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.groupbyattrs/
description: Learn about otelcol.processor.groupbyattrs
title: otelcol.processor.groupbyattrs
---

# otelcol.processor.groupbyattrs

`otelcol.processor.groupbyattrs` accepts telemetry data from other `otelcol`
components and reassociates spans, log records, and metric datapoints to a resource that matches the specified attributes. It groups telemetry data by specified attributes.

{{% admonition type="note" %}}
`otelcol.processor.groupbyattrs` is a wrapper over the upstream OpenTelemetry
Collector `groupbyattrs` processor. If necessary, bug reports or feature requests
will be redirected to the upstream repository.
{{% /admonition %}}

It is recommended to use the groupbyattrs processor together with [otelcol.processor.batch][], as a consecutive step, as this will reduce the fragmentation of data (by grouping records together under matching Resource/Instrumentation Library)

You can specify multiple `otelcol.processor.groupbyattrs` components by giving them
different labels.

## Usage

```river
otelcol.processor.groupbyattrs "LABEL" {
  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

The following arguments are supported:

| Name            | Type              | Description                                                                           | Default | Required |
|-----------------|-------------------|---------------------------------------------------------------------------------------|---------|----------|
| `keys`          | `array(string)`   | Keys that will be used to group the spans, log records or metric data points together |         | no       |
| `output`        | [output][]        | Configures where to send received telemetry data.                                     | yes     |          |
| `debug_metrics` | [debug_metrics][] | Configures the metrics that this component generates to monitor its state.            | no      |          |

[output]: #output-block
[debug_metrics]: #debug_metrics-block

### keys
`keys` is a string array that is used for grouping the data. If it is empty, the processor performs compaction and reassociates all spans with matching Resource and InstrumentationLibrary.


### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                   |
|---------|--------------------|---------------------------------------------------------------|
| `input` | `otelcol.Consumer` | Accepts `otelcol.Consumer` data for metrics, logs, or traces. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics,
logs, or traces).

## Component health

`otelcol.processor.groupbyattrs` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.processor.groupbyattrs` does not expose any component-specific debug
information.

## Debug metrics

`otelcol.processor.groupbyattrs` does not expose any component-specific debug metrics.

## Examples

### Grouping metrics by an attribute

This example reassociates the metrics based on the value of the `host.name` attribute.

```alloy
otelcol.processor.groupbyattrs "default" {
  keys = [
    "host.name",
  ]

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```

## Notes
- The data points with different data types aren't merged under the same metric. For example, a gauge and sum metric would not be merged.
- The data points without the specified keys remain under their respective resources.
- New resources inherit the attributes of the original resource and the specified attributes in the keys array.
- The grouping attributes in the keys array are removed from the output metrics.

[otelcol.processor.batch]: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.batch/

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.groupbyattrs/
description: Learn about otelcol.processor.groupbyattrs
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.processor.groupbyattrs
---

# `otelcol.processor.groupbyattrs`

`otelcol.processor.groupbyattrs` accepts spans, metrics, and traces from other `otelcol` components and groups them under the same resource.

{{< admonition type="note" >}}
`otelcol.processor.groupbyattrs` is a wrapper over the upstream OpenTelemetry Collector [`groupbyattrs`][] processor.
If necessary, bug reports or feature requests will be redirected to the upstream repository.

[`groupbyattrs`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/groupbyattrsprocessor
{{< /admonition >}}

We recommend you use the groupbyattrs processor together with [`otelcol.processor.batch`][otelcol.processor.batch], as a consecutive step.
This will reduce the fragmentation of data by grouping records together under the matching Resource/Instrumentation Library.

You can specify multiple `otelcol.processor.groupbyattrs` components by giving them different labels.

## Usage

```alloy
otelcol.processor.groupbyattrs "<LABEL>" {
  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

You can use the following argument with `otelcol.processor.groupbyattrs`:

| Name   | Type           | Description                                                                             | Default | Required |
| ------ | -------------- | --------------------------------------------------------------------------------------- | ------- | -------- |
| `keys` | `list(string)` | Keys that will be used to group the spans, log records, or metric data points together. | `[]`    | no       |

`keys` is a string array that's used for grouping the data.
If it's empty, the processor performs compaction and reassociates all spans with matching Resource and InstrumentationLibrary.

## Blocks

You can use the following blocks with `otelcol.processor.groupbyattrs`:

| Block                            | Description                                                                | Required |
| -------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[output]: #output
[debug_metrics]: #debug_metrics

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                   |
| ------- | ------------------ | ------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | Accepts `otelcol.Consumer` data for metrics, logs, or traces. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.processor.groupbyattrs` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.processor.groupbyattrs` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.processor.groupbyattrs` doesn't expose any component-specific debug metrics.

## Examples

### Group metrics

Consider the following metrics, all originally associated to the same Resource:

```text
Resource {host.name="localhost",source="prom"}
  Metric "gauge-1" (GAUGE)
    DataPoint {host.name="host-A",id="eth0"}
    DataPoint {host.name="host-A",id="eth0"}
    DataPoint {host.name="host-B",id="eth0"}
  Metric "gauge-1" (GAUGE) // Identical to previous Metric
    DataPoint {host.name="host-A",id="eth0"}
    DataPoint {host.name="host-A",id="eth0"}
    DataPoint {host.name="host-B",id="eth0"}
  Metric "mixed-type" (GAUGE)
    DataPoint {host.name="host-A",id="eth0"}
    DataPoint {host.name="host-A",id="eth0"}
    DataPoint {host.name="host-B",id="eth0"}
  Metric "mixed-type" (SUM)
    DataPoint {host.name="host-A",id="eth0"}
    DataPoint {host.name="host-A",id="eth0"}
  Metric "dont-move" (Gauge)
    DataPoint {id="eth0"}
```

With the following configuration, the groupbyattrs will re-associate the metrics with either `host-A` or `host-B`, based on the value of the `host.name` attribute.

```alloy
otelcol.processor.groupbyattrs "default" {
  keys = [ "host.name" ]
  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}
```

The output of the processor is:

```text
Resource {host.name="localhost",source="prom"}
  Metric "dont-move" (Gauge)
    DataPoint {id="eth0"}
Resource {host.name="host-A",source="prom"}
  Metric "gauge-1"
    DataPoint {id="eth0"}
    DataPoint {id="eth0"}
    DataPoint {id="eth0"}
    DataPoint {id="eth0"}
  Metric "mixed-type" (GAUGE)
    DataPoint {id="eth0"}
    DataPoint {id="eth0"}
  Metric "mixed-type" (SUM)
    DataPoint {id="eth0"}
    DataPoint {id="eth0"}
Resource {host.name="host-B",source="prom"}
  Metric "gauge-1"
    DataPoint {id="eth0"}
    DataPoint {id="eth0"}
  Metric "mixed-type" (GAUGE)
    DataPoint {id="eth0"}
```

This output demonstrates how `otelcol.processor.groupbyattrs` works in various situations:

* The DataPoints for the `gauge-1` (GAUGE) metric were originally split under 2 Metric instances and have been merged in the output.
* The DataPoints of the `mixed-type` (GAUGE) and `mixed-type` (SUM) metrics haven't been merged under the same Metric, because their DataType is different.
* The `dont-move` metric DataPoints don't have a `host.name` attribute and therefore remained under the original Resource.
* The new Resources inherited the attributes from the original Resource (`source="prom"`), plus the specified attributes from the processed metrics (`host.name="host-A"` or `host.name="host-B"`).
* The specified "grouping" attributes that are set on the new Resources are also removed from the metric DataPoints.
* While not shown in the above example, the processor also merges collections of records under matching InstrumentationLibrary.

### Compaction

Sometimes telemetry data can become fragmented due to multiple duplicated ResourceSpans/ResourceLogs/ResourceMetrics objects.
This leads to additional memory consumption, increased processing costs, inefficient serialization and increase of the export requests.
In such situations, `otelcol.processor.groupbyattrs` can be used to compact the data with matching Resource and InstrumentationLibrary properties.

For example, consider this input data:

```text
Resource {host.name="localhost"}
  InstrumentationLibrary {name="MyLibrary"}
  Spans
    Span {span_id=1, ...}
  InstrumentationLibrary {name="OtherLibrary"}
  Spans
    Span {span_id=2, ...}

Resource {host.name="localhost"}
  InstrumentationLibrary {name="MyLibrary"}
  Spans
    Span {span_id=3, ...}

Resource {host.name="localhost"}
  InstrumentationLibrary {name="MyLibrary"}
  Spans
    Span {span_id=4, ...}

Resource {host.name="otherhost"}
  InstrumentationLibrary {name="MyLibrary"}
  Spans
    Span {span_id=5, ...}
```

You can use `otelcol.processor.groupbyattrs` with its default configuration to compact the data:

```alloy
otelcol.processor.groupbyattrs "default" {
  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}
```

The output is:

```text
Resource {host.name="localhost"}
  InstrumentationLibrary {name="MyLibrary"}
  Spans
    Span {span_id=1, ...}
    Span {span_id=3, ...}
    Span {span_id=4, ...}
  InstrumentationLibrary {name="OtherLibrary"}
  Spans
    Span {span_id=2, ...}

Resource {host.name="otherhost"}
  InstrumentationLibrary {name="MyLibrary"}
  Spans
    Span {span_id=5, ...}
```

[otelcol.processor.batch]: ../otelcol.processor.batch/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.groupbyattrs` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.groupbyattrs` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

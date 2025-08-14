---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.deltatocumulative/
aliases:
  - ../otelcol.processor.deltatocumulative/ # /docs/alloy/latest/reference/otelcol.processor.deltatocumulative/
description: Learn about otelcol.processor.deltatocumulative
labels:
  stage: experimental
  products:
    - oss
title: otelcol.processor.deltatocumulative
---

# `otelcol.processor.deltatocumulative`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.processor.deltatocumulative` accepts metrics from other `otelcol` components and converts metrics with the delta temporality to cumulative.

{{< admonition type="note" >}}
`otelcol.processor.deltatocumulative` is a wrapper over the upstream OpenTelemetry Collector [`deltatocumulative`][] processor.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`deltatocumulative`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/deltatocumulativeprocessor
{{< /admonition >}}

You can specify multiple `otelcol.processor.deltatocumulative` components by giving them different labels.

## Usage

```alloy
otelcol.processor.deltatocumulative "<LABEL>" {
  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.processor.deltatocumulative`:

| Name          | Type       | Description                                                         | Default               | Required |
|---------------|------------|---------------------------------------------------------------------|-----------------------|----------|
| `max_stale`   | `duration` | How long to wait for a new sample before marking a stream as stale. | `"5m"`                | no       |
| `max_streams` | `number`   | Upper limit of streams to track. Set to `0` to disable.             | `9223372036854775807` | no       |

`otelcol.processor.deltatocumulative` tracks incoming metric streams.
Sum and exponential histogram metrics with delta temporality are tracked and converted into cumulative temporality.

If a new sample hasn't been received since the duration specified by `max_stale`, tracked streams are considered stale and dropped. `max_stale` must be set to a duration greater than `"0s"`.

The `max_streams` attribute configures the upper limit of streams to track.
If the limit of tracked streams is reached, new incoming streams are dropped.

## Blocks

You can use the following blocks with `otelcol.processor.deltatocumulative`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
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

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for metrics.

## Component health

`otelcol.processor.deltatocumulative` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.processor.deltatocumulative` doesn't expose any component-specific debug information.

## Debug metrics

* `otelcol_deltatocumulative_datapoints` (counter): Total number of datapoints processed (successfully or unsuccessfully).
* `otelcol_deltatocumulative_streams_limit` (gauge): Upper limit of tracked streams.
* `otelcol_deltatocumulative_streams_max_stale_seconds` (gauge): Duration without new samples after which streams are dropped.
* `otelcol_deltatocumulative_streams_tracked` (gauge): Number of streams currently tracked by the aggregation state.

## Examples

### Basic usage

This example converts delta temporality metrics to cumulative before sending it to [otelcol.exporter.otlp][] for further processing:

```alloy
otelcol.processor.deltatocumulative "default" {
  output {
    metrics = [otelcol.exporter.otlp.production.input]
  }
}

otelcol.exporter.otlp "production" {
  client {
    endpoint = sys.env("<OTLP_SERVER_ENDPOINT>")
  }
}
```

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/

### Export Prometheus data

This example converts delta temporality metrics to cumulative metrics before it's converted to Prometheus data, which requires cumulative temporality:

```alloy
otelcol.processor.deltatocumulative "default" {
  output {
    metrics = [otelcol.exporter.prometheus.default.input]
  }
}

otelcol.exporter.prometheus "default" {
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = sys.env("<PROMETHEUS_SERVER_URL>")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.deltatocumulative` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.deltatocumulative` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

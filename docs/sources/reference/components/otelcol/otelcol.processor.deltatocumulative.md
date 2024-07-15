---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.deltatocumulative/
aliases:
  - ../otelcol.processor.deltatocumulative/ # /docs/alloy/latest/reference/otelcol.processor.deltatocumulative/
description: Learn about otelcol.processor.deltatocumulative
title: otelcol.processor.deltatocumulative
---

<span class="badge docs-labels__stage docs-labels__item">Experimental</span>

# otelcol.processor.deltatocumulative

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.processor.deltatocumulative` accepts metrics from other `otelcol` components and converts metrics with the delta temporality to cumulative.

{{< admonition type="note" >}}
`otelcol.processor.deltatocumulative` is a wrapper over the upstream OpenTelemetry Collector `deltatocumulative` processor.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

You can specify multiple `otelcol.processor.deltatocumulative` components by giving them different labels.

## Usage

```alloy
otelcol.processor.deltatocumulative "LABEL" {
  output {
    metrics = [...]
  }
}
```

## Arguments

`otelcol.processor.deltatocumulative` supports the following arguments:

Name          | Type       | Description                                                         | Default | Required
------------- | ---------- | ------------------------------------------------------------------- | ------- | --------
`max_stale`   | `duration` | How long to wait for a new sample before marking a stream as stale. | `"5m"`  | no
`max_streams` | `number`   | Upper limit of streams to track. Set to `0` to disable.             | `0`     | no

`otelcol.processor.deltatocumulative` tracks incoming metric streams.
Sum and exponential histogram metrics with delta temporality are tracked and converted into cumulative temporality.

If a new sample hasn't been received since the duration specified by `max_stale`, tracked streams are considered stale and dropped. `max_stale` must be set to a duration greater than `"0s"`.

The `max_streams` attribute configures the upper limit of streams to track.
If the limit of tracked streams is reached, new incoming streams are dropped.
You can disable this behavior by setting `max_streams` to `0`.

## Blocks

The following blocks are supported inside the definition of `otelcol.processor.deltatocumulative`:

Hierarchy     | Block             | Description                                                                | Required
------------- | ----------------- | -------------------------------------------------------------------------- | --------
output        | [output][]        | Configures where to send received telemetry data.                          | yes
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

[output]: #output-block
[debug_metrics]: #debug_metrics-block

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for metrics.

## Component health

`otelcol.processor.deltatocumulative` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.processor.deltatocumulative` does not expose any component-specific debug information.

## Debug metrics

* `processor_deltatocumulative_streams_tracked` (gauge): Number of streams currently tracked by the aggregation state.
* `processor_deltatocumulative_streams_limit` (gauge): Upper limit of tracked streams.
* `processor_deltatocumulative_streams_evicted` (counter): Total number of streams removed from tracking to ingest newer streams.
* `processor_deltatocumulative_streams_max_stale` (gauge): Duration without new samples after which streams are dropped.
* `processor_deltatocumulative_datapoints_processed` (counter): Total number of datapoints processed (successfully or unsuccessfully).
* `processor_deltatocumulative_datapoints_dropped` (counter): Faulty datapoints that were dropped due to the reason given in the `reason` label.
* `processor_deltatocumulative_gaps_length` (counter): Total length of all gaps in the streams, such as being due to lost in transit.

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
    endpoint = env("OTLP_SERVER_ENDPOINT")
  }
}
```

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/

### Exporting Prometheus data

This example converts delta temporality metrics to cumulative metrics before it is converted to Prometheus data, which requires cumulative temporality:

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
    url = env("PROMETHEUS_SERVER_URL")
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

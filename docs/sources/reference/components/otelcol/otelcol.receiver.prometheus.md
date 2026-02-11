---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.prometheus/
aliases:
  - ../otelcol.receiver.prometheus/ # /docs/alloy/latest/reference/otelcol.receiver.prometheus/
description: Learn about otelcol.receiver.prometheus
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.receiver.prometheus
---

# `otelcol.receiver.prometheus`

`otelcol.receiver.prometheus` receives Prometheus metrics, converts them to the OpenTelemetry metrics format, and forwards them to other `otelcol.*` components.
This is a custom component built on a fork of the upstream OpenTelemetry Collector receiver.

You can specify multiple `otelcol.receiver.prometheus` components by giving them different labels.

{{< admonition type="note" >}}
Support for translating Prometheus native histograms into OTLP exponential histograms is a public preview feature.
To enable native histogram translation, run {{< param "PRODUCT_NAME" >}} with the `--stability.level=public-preview` configuration flag.
{{< /admonition >}}


{{< admonition type="note" >}}
In {{< param "PRODUCT_NAME" >}} v1.14.0, `otelcol.receiver.prometheus` no longer sets OTLP metric [start times][otlp-start-time] for Prometheus metrics. The receiver forwards metrics without adding start times when the Prometheus input does not have a [created time][om-suffixes].

Start time is a way to tell when a cumulative metric such as an OTLP "sum" or a Prometheus "counter" was last reset.
If your database uses start times for OTLP metrics, you can use `otelcol.processor.metric_start_time` to set it.
Grafana Mimir's OTLP endpoint and Grafana Cloud's OTLP Gateway both support OTLP metric start time.

To add the start time in the same way that `otelcol.receiver.prometheus` did it prior to {{< param "PRODUCT_NAME" >}} v1.14.0, 
configure `otelcol.processor.metric_start_time` with `strategy` set to `true_reset_point`.

In practice it is only necessary to use `true_reset_point` if your Mimir/Grafana Cloud instance is explicitly configured with a 
`-distributor.otel-created-timestamp-zero-ingestion-enabled=true` flag, 
since `true_reset_point` will send a sample with the same start and end time and Mimir by default won't create a zero sample for it.
For most Mimir and Grafana Cloud use cases there would be no change in behaviour even if no `otelcol.processor.metric_start_time` is added.

[om-suffixes]: https://github.com/prometheus/OpenMetrics/blob/v1.0.0/specification/OpenMetrics.md#suffixes
[otlp-start-time]: https://github.com/open-telemetry/opentelemetry-proto/blob/v1.9.0/opentelemetry/proto/metrics/v1/metrics.proto#L181-L187
{{< /admonition >}}

## Usage

```alloy
otelcol.receiver.prometheus "<LABEL>" {
  output {
    metrics = [...]
  }
}
```

## Arguments

The `otelcol.receiver.prometheus` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `otelcol.receiver.prometheus`:

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

This component exports the following fields that other components can reference:

| Name       | Type              | Description                                                          |
| ---------- | ----------------- | -------------------------------------------------------------------- |
| `receiver` | `MetricsReceiver` | A value that other components can use to send Prometheus metrics to. |

## Component health

`otelcol.receiver.prometheus` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.prometheus` doesn't expose any component-specific debug information.

## Example

This example uses the `otelcol.receiver.prometheus` component as a bridge between the Prometheus and OpenTelemetry ecosystems.
The component exposes a receiver which the `prometheus.scrape` component uses to send Prometheus metric data to.
The receiver converts the metrics to OTLP format and forwards them to the `otelcol.exporter.otlp` component, which sends them to an OTLP-capable endpoint:

```alloy
prometheus.scrape "default" {
    // Collect metrics from the default HTTP listen address.
    targets = [{"__address__"   = "127.0.0.1:12345"}]

    forward_to = [otelcol.receiver.prometheus.default.receiver]
}

otelcol.receiver.prometheus "default" {
  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = sys.env("OTLP_ENDPOINT")
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.prometheus` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.receiver.prometheus` has exports that can be consumed by the following components:

- Components that consume [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.prometheus/
aliases:
  - ../otelcol.exporter.prometheus/ # /docs/alloy/latest/reference/components/otelcol.exporter.prometheus/
description: Learn about otelcol.exporter.prometheus
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.exporter.prometheus
---

# `otelcol.exporter.prometheus`

`otelcol.exporter.prometheus` accepts OTLP-formatted metrics from other `otelcol` components, converts metrics to Prometheus-formatted metrics, and forwards the resulting metrics to `prometheus` components.

{{< admonition type="note" >}}
`otelcol.exporter.prometheus` is a custom component unrelated to the `prometheus` exporter from OpenTelemetry Collector.

Conversion of metrics are done according to the OpenTelemetry [Metrics Data Model][] specification.

[Metrics Data Model]: https://opentelemetry.io/docs/reference/specification/metrics/data-model/
{{< /admonition >}}

You can specify multiple `otelcol.exporter.prometheus` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.prometheus "<LABEL>" {
  forward_to = [...]
}
```

## Arguments

You can use the following arguments with `otelcol.exporter.prometheus`:

| Name                               | Type                    | Description                                                                    | Default | Required |
| ---------------------------------- | ----------------------- | ------------------------------------------------------------------------------ | ------- | -------- |
| `forward_to`                       | `list(MetricsReceiver)` | Where to forward converted Prometheus metrics.                                 |         | yes      |
| `add_metric_suffixes`              | `bool`                  | Whether to add type and unit suffixes to metric names.                         | `true`  | no       |
| `gc_frequency`                     | `duration`              | How often to clean up stale metrics from memory.                               | `"5m"`  | no       |
| `honor_metadata`                   | `bool`                  | (Experimental) Whether to send metric metadata to downstream components.       | `false` | no       |
| `include_scope_info`               | `bool`                  | Whether to include `otel_scope_info` metrics.                                  | `false` | no       |
| `include_scope_labels`             | `bool`                  | Whether to include additional OTLP labels in all metrics.                      | `true`  | no       |
| `include_target_info`              | `bool`                  | Whether to include `target_info` metrics.                                      | `true`  | no       |
| `resource_to_telemetry_conversion` | `bool`                  | Whether to convert OTel resource attributes to Prometheus labels.              | `false` | no       |

By default, OpenTelemetry resources are converted into `target_info` metrics.
OpenTelemetry instrumentation scopes are converted into `otel_scope_info` metrics.
Set the `include_scope_info` and `include_target_info` arguments to `false`, respectively, to disable the custom metrics.

{{< admonition type="warning" >}}
**EXPERIMENTAL**: The `honor_metadata` argument is an [experimental][] feature.
If you enable this argument, resource consumption may increase, particularly if you ingest many metrics with different names.
Some downstream components aren't compatible with Prometheus metadata.
The following components are compatible:

* `otelcol.receiver.prometheus`
* `prometheus.remote_write` only when configured for Remote Write v2.
* `prometheus.write_queue`

[experimental]: ../../../get-started/configuration-syntax/expressions/function_calls/#experimental-functions
{{< /admonition >}}

When `include_scope_labels` is `true`  the `otel_scope_name` and `otel_scope_version` labels are added to every converted metric sample.

When `include_target_info` is true, OpenTelemetry Collector resources are converted into `target_info` metrics.

{{< admonition type="note" >}}
OTLP metrics can have a lot of resource attributes.
Setting `resource_to_telemetry_conversion` to `true` would convert all of them to Prometheus labels, which may not be what you want.
Instead of using `resource_to_telemetry_conversion`, most users need to use `otelcol.processor.transform`
to convert OTLP resource attributes to OTLP metric datapoint attributes before using `otelcol.exporter.prometheus`.
Refer to [Create Prometheus labels from OTLP resource attributes][] for an example.

[Create Prometheus labels from OTLP resource attributes]: #create-prometheus-labels-from-otlp-resource-attributes
{{< /admonition >}}

## Blocks

You can use the following blocks with `otelcol.exporter.prometheus`:

| Block                            | Description                                                                | Required |
| -------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[debug_metrics]: #debug_metrics

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for metrics. Other telemetry signals are ignored.

Metrics sent to the `input` are converted to Prometheus-compatible metrics and are forwarded to the `forward_to` argument.

The following are dropped during the conversion process:

* Metrics that use the delta aggregation temporality.
  {{< admonition type="note" >}}
  Prometheus doesn't natively support delta metrics.
  If your {{< param "PRODUCT_NAME" >}} instance ingests delta OTLP metrics, you can convert them to cumulative OTLP metrics with [`otelcol.processor.deltatocumulative`][otelcol.processor.deltatocumulative] before you use `otelcol.exporter.prometheus`.

  [otelcol.processor.deltatocumulative]: ../otelcol.processor.deltatocumulative/
  {{< /admonition >}}

## Component health

`otelcol.exporter.prometheus` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.prometheus` doesn't expose any component-specific debug information.

## Example

## Basic usage

This example accepts metrics over OTLP and forwards it using `prometheus.remote_write`:

```alloy
otelcol.receiver.otlp "default" {
  grpc {}

  output {
    metrics = [otelcol.exporter.prometheus.default.input]
  }
}

otelcol.exporter.prometheus "default" {
  forward_to = [prometheus.remote_write.mimir.receiver]
}

prometheus.remote_write "mimir" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}
```

## Create Prometheus labels from OTLP resource attributes

This example uses `otelcol.processor.transform` to add extra `key1` and `key2` OTLP metric datapoint attributes from the `key1` and `key2` OTLP resource attributes.

`otelcol.exporter.prometheus` then converts `key1` and `key2` to Prometheus labels along with any other OTLP metric datapoint attributes.

This avoids the need to set `resource_to_telemetry_conversion` to `true`, which could have created too many unnecessary metric labels.

```alloy
otelcol.receiver.otlp "default" {
  grpc {}

  output {
    metrics = [otelcol.processor.transform.default.input]
  }
}

otelcol.processor.transform "default" {
  error_mode = "ignore"

  metric_statements {
    context = "datapoint"

    statements = [
      `set(attributes["key1"], resource.attributes["key1"])`,
      `set(attributes["key2"], resource.attributes["key2"])`,
    ]
  }

  output {
    metrics = [otelcol.exporter.prometheus.default.input]
  }
}

otelcol.exporter.prometheus "default" {
  forward_to = [prometheus.remote_write.mimir.receiver]
}

prometheus.remote_write "mimir" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.prometheus` can accept arguments from the following components:

- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)

`otelcol.exporter.prometheus` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.echo/
description: Learn about prometheus.echo
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.echo
---

# `prometheus.echo`

The `prometheus.echo` component receives Prometheus metrics and writes them to stdout in Prometheus exposition format.
This component is useful for debugging and testing the flow of metrics through a pipeline, allowing you to see exactly what metrics are being received at a particular point in your configuration.

## Usage

```alloy
prometheus.echo "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.echo`:

| Name     | Type     | Description                                                     | Default | Required |
| -------- | -------- | --------------------------------------------------------------- | ------- | -------- |
| `format` | `string` | The output format for metrics. Must be `text` or `openmetrics`. | `text`  | no       |

The `format` argument controls how metrics are encoded when written to stdout:

* `text` - Uses the Prometheus text exposition format (default).
* `openmetrics` - Uses the OpenMetrics text format.

## Blocks

The `prometheus.echo` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type                    | Description                              |
| ---------- | ----------------------- | ---------------------------------------- |
| `receiver` | `prometheus.Appendable` | A value that other components can use to send metrics to. |

## Component health

`prometheus.echo` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.echo` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.echo` doesn't expose any component-specific debug metrics.

## Example

This example creates a metrics generation and inspection pipeline:

```alloy
prometheus.exporter.unix "default" {
}

prometheus.scrape "demo" {
  targets    = prometheus.exporter.unix.default.targets
  forward_to = [prometheus.echo.debug.receiver]
}

prometheus.echo "debug" {
  format = "text"
}
```

In this example:

1. The `prometheus.exporter.unix` component exposes system metrics.
1. The `prometheus.scrape` component scrapes those metrics.
1. The `prometheus.echo` component receives the scraped metrics and writes them to stdout in Prometheus text format.

When you run this configuration, you'll see the metrics being written to stdout, which is useful to:

* Debug metric collection issues
* Verify metric labels and values
* Test metric transformations
* Understand the structure of metrics in your pipeline

### Example with OpenMetrics format

```alloy
prometheus.scrape "demo" {
  targets = [
    {"__address__" = "localhost:9090"},
  ]
  forward_to = [prometheus.echo.debug.receiver]
}

prometheus.echo "debug" {
  format = "openmetrics"
}
```

This example outputs metrics using the OpenMetrics format instead of the traditional Prometheus text format.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.echo` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)

## Has exports

- [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

<!-- END GENERATED COMPATIBLE COMPONENTS -->

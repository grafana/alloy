---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-debug-metrics-block/
description: Shared content, otelcol debug metrics block
headless: true
---

The `debug_metrics` block configures the metrics that this component generates to monitor its state.

The following arguments are supported:

Name                               | Type      | Description                                          | Default | Required
-----------------------------------|-----------|------------------------------------------------------|---------|---------
`disable_high_cardinality_metrics` | `boolean` | Whether to disable certain high cardinality metrics. | `true`  | no
`level` | `string` |  Controls level of detail for metrics emitted by the wrapped collector. Possible values are `none`, `basic`, `normal` and `detailed`) | `detailed`  | no

`disable_high_cardinality_metrics` is the Grafana Alloy equivalent to the `telemetry.disableHighCardinalityMetrics` feature gate in the OpenTelemetry Collector.
It removes attributes that could cause high cardinality metrics.
For example, attributes with IP addresses and port numbers in metrics about HTTP and gRPC connections are removed.

`level` is the Grafana Alloy equivalent to the `telemetry.metrics.level` feature gate in the OpenTelemetry Collector.
---
aliases:
- /docs/agent/shared/flow/reference/components/otelcol-debug-metrics-block/
- /docs/grafana-cloud/agent/shared/flow/reference/components/otelcol-debug-metrics-block/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/otelcol-debug-metrics-block/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/otelcol-debug-metrics-block/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/otelcol-debug-metrics-block/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/otelcol-debug-metrics-block/
description: Shared content, otelcol debug metrics block
headless: true
---

The `debug_metrics` block configures the metrics that this component generates to monitor its state.

The following arguments are supported:

Name                               | Type      | Description                                          | Default | Required
-----------------------------------|-----------|------------------------------------------------------|---------|---------
`disable_high_cardinality_metrics` | `boolean` | Whether to disable certain high cardinality metrics. | `true`  | no

`disable_high_cardinality_metrics` is the Grafana Agent equivalent to the `telemetry.disableHighCardinalityMetrics` feature gate in the OpenTelemetry Collector.
It removes attributes that could cause high cardinality metrics.
For example, attributes with IP addresses and port numbers in metrics about HTTP and gRPC connections are removed.

---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/modify-telemetry/
description: Learn where and how telemetry is modified in Grafana Alloy processing stages
menuTitle: Modify telemetry
title: Where Grafana Alloy modifies telemetry
weight: 300
---

# Where {{% param "FULL_PRODUCT_NAME" %}} modifies telemetry

Processing components are the only place where telemetry changes.

Receivers ingest data.
Exporters send data out.
Any change to telemetry happens between those two stages.

If you don't connect processing components in a path, telemetry passes through {{< param "PRODUCT_NAME" >}} without modification.

## Modification happens in processing stages

A typical path looks like this:

Receiver → Processing → Exporter

Processing components sit in the middle of that path.
They receive telemetry from upstream components and forward the resulting telemetry downstream.

Those components are responsible for any:

- Field updates
- Label changes
- Filtering decisions
- Routing logic
- Sampling behavior

If a processing component isn't part of a path, it has no effect on that telemetry.

## Modification is explicit

{{< param "PRODUCT_NAME" >}} doesn't apply automatic transformations.

It doesn't:

- Redact log content by default.
- Reduce metric cardinality automatically.
- Drop telemetry unless configured.
- Sample traces without an explicit sampling component.

If telemetry changes, the configuration defines where and how that change occurs.

This explicit model makes behavior predictable.
You can identify exactly which component modifies data by tracing the graph.

## Signal-aware processing

Metrics, logs, and traces move through separate pipelines.
Processing components operate on specific signal types.

For example:

- A metric-processing component only affects metric data.
- A log-processing component only affects log data.
- A trace-processing component only affects trace data.

A component can't modify telemetry it doesn't receive.
Signal type and graph connections both determine what gets processed.

## No modification at ingestion or export by default

Receivers focus on accepting telemetry and making it available inside the graph.
Exporters focus on delivering telemetry to external systems.

Unless explicitly documented otherwise for a specific component, receivers, and exporters don't implicitly modify telemetry passing through them.

All transformation logic belongs in configured processing stages.

## Trace modification paths

To determine where telemetry changes in a configuration:

1. Start at a receiver.
1. Follow downstream connections.
1. Identify any processing components in the path.
1. Inspect those components to understand their behavior.

Those components define how telemetry changes before it leaves {{< param "PRODUCT_NAME" >}}.

## Next steps

- [Read configurations as data flow](../read-configurations/) - Interpret configurations using the graph model to identify modification paths.

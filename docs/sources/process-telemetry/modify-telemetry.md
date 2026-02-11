---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/modify-telemetry/
description: Learn where and how telemetry is modified in Grafana Alloy processing stages
menuTitle: Modify telemetry
title: Where telemetry is modified
weight: 300
---

---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/modify-telemetry/
description: Understand where and how Grafana Alloy modifies telemetry within processing stages
menuTitle: Modify telemetry
title: Where Grafana Alloy modifies telemetry
weight: 30
---

# Where {{% param "FULL_PRODUCT_NAME" %}} modifies telemetry

Telemetry is only modified inside processing components.

Receivers ingest data.
Exporters send data out.
Any change to telemetry happens between those two stages.

If no processing components are connected in a path, telemetry passes through {{< param "PRODUCT_NAME" >}} without intermediate modification.

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

If a processing component is not part of a path, it has no effect on that telemetry.

## Modification is explicit

{{< param "PRODUCT_NAME" >}} does not apply automatic transformations.

It does not:

- Redact log content by default.
- Reduce metric cardinality automatically.
- Drop telemetry unless configured.
- Sample traces without an explicit sampling component.

If telemetry is altered, the configuration defines where and how that alteration occurs.

This explicit model makes behavior predictable.
You can identify exactly which component modifies data by tracing the graph.

## Signal-aware processing

Logs, metrics, and traces move through separate pipelines.
Processing components are designed to operate on specific signal types.

For example:

- A log-processing component only affects log data.
- A metric-processing component only affects metric data.
- A trace-processing component only affects trace data.

A component cannot modify telemetry it does not receive.
Signal type and graph connections both determine what gets processed.

## No modification at ingestion or export by default

Receivers focus on accepting telemetry and making it available inside the graph.
Exporters focus on delivering telemetry to external systems.

Unless explicitly documented otherwise for a specific component, receivers and exporters do not implicitly modify telemetry passing through them.

All transformation logic belongs in configured processing stages.

## Tracing modification paths

To determine where telemetry is modified in a configuration:

1. Start at a receiver.
1. Follow downstream connections.
1. Identify any processing components in the path.
1. Inspect those components to understand their behavior.

Those components define how telemetry changes before it leaves {{< param "PRODUCT_NAME" >}}.

Next, learn how to read configurations as data flow diagrams to make these paths easier to identify.

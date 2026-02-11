---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/modify-telemetry/
description: Learn where and how telemetry is modified in Grafana Alloy processing stages
menuTitle: Modify telemetry
title: Where Grafana Alloy modifies telemetry
weight: 300
---

# Where {{% param "FULL_PRODUCT_NAME" %}} modifies telemetry

Processors are the only place where telemetry changes.
Processors can filter, rewrite, enrich, sample, or route telemetry depending on how you configure them.

Receivers ingest data.
Exporters send data out.
Any change to telemetry happens between those two stages.

If you don't connect processors in a path, telemetry passes through {{< param "PRODUCT_NAME" >}} without modification.

## Modification happens in processing stages

A typical path looks like this:

Receiver → Processor → Exporter

Processors sit in the middle of that path.
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

If a processor isn't connected in the path between a receiver and an exporter, it has no effect on that telemetry.

## Signal-aware processing

Metrics, logs, and traces move through separate pipelines.
Processors operate on specific signal types.

For example:

- A metric-processing component only affects metric data.
- A log-processing component only affects log data.
- A trace-processing component only affects trace data.

A component can't modify telemetry it doesn't receive.
Signal type and graph connections both determine what gets processed.

## No modification at ingestion or export by default

Receivers handle ingestion and normalization. Exporters handle delivery to external systems.

Unless explicitly documented for a specific component, neither receivers nor exporters perform semantic transformations on telemetry passing through them.

All transformation logic belongs in configured processing stages.

## Trace modification paths

To determine where telemetry changes in a configuration:

1. Start at a receiver.
1. Follow downstream connections.
1. Identify any processors in the path.
1. Inspect those components to understand their behavior.

Those components define how telemetry changes before it leaves {{< param "PRODUCT_NAME" >}}.

Understanding where modification occurs makes it easier to design pipelines that filter data, reduce noise, or control what gets sent to downstream systems.

## Next steps

- [Read configurations as data flow](../read-configurations/) - Interpret configurations using the graph model to identify modification paths.

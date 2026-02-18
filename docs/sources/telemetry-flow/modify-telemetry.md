---
canonical: https://grafana.com/docs/alloy/latest/telemetry-flow/modify-telemetry/
description: Learn where and how Grafana Alloy modifies telemetry
menuTitle: Modify telemetry
title: Where Grafana Alloy modifies telemetry
weight: 300
---

# Where {{% param "FULL_PRODUCT_NAME" %}} modifies telemetry

Transformation components are the only place where telemetry changes.
Transformation components can filter, rewrite, enrich, sample, or route telemetry depending on how you configure them.

Ingestion components ingest data.
Output components forward data to their configured destinations.
Any change to telemetry happens between those two stages.

If you don't connect transformation components in a path, telemetry passes through {{< param "PRODUCT_NAME" >}} without modification.

## Modification happens inside transformation components

A typical path looks like this:

{{< mermaid >}}
flowchart LR
  Ingestion --> Transformation --> Output
{{< /mermaid >}}

Transformation components sit in the middle of that path.
They receive telemetry from earlier components in the path and forward the resulting telemetry to the next component.

Those components are responsible for any:

- Field updates
- Label changes
- Filtering decisions
- Routing logic
- Sampling behavior

If a transformation component isn't part of a path, it has no effect on that telemetry.

## Modification is explicit

{{< param "PRODUCT_NAME" >}} doesn't apply automatic transformations.

It doesn't:

- Redact log content by default.
- Reduce metric cardinality automatically.
- Drop telemetry unless configured.
- Sample traces without an explicit sampling component.

If telemetry changes, the configuration defines where and how that change occurs.

This explicit model makes behavior predictable.
You can identify exactly which component modifies data by tracing the connections.

If a transformation component isn't connected in the path between an ingestion and an output component, it has no effect on that telemetry.

## Signal-aware processing

Metric, log, and trace connections are all defined explicitly.
If a component supports multiple types of telemetry, each type needs to be explicitly connected to the next component in its pipeline.

For example:

- A metric-processing component only affects metric data.
- A log-processing component only affects log data.
- A trace-processing component only affects trace data.

## No modification at ingestion or output

Ingestion components handle ingestion and normalization.
Output components handle delivery to configured destinations.

Unless explicitly documented for a specific component, neither ingestion nor output components perform semantic transformations on telemetry passing through them.

## Find where modification occurs

To determine where telemetry changes in a configuration:

1. Start at an ingestion component.
1. Follow where each component sends telemetry.
1. Identify any transformation components in the path.
1. Inspect those components to understand their behavior.

Those components define how telemetry changes before it leaves {{< param "PRODUCT_NAME" >}}.

Understanding where modification occurs makes it easier to design pipelines that filter data, reduce noise, or control what gets sent to external systems.

## Next steps

- [Read configurations as data flow](../read-configurations/) - Interpret configurations by tracing telemetry paths.

---
canonical: https://grafana.com/docs/alloy/latest/introduction/telemetry/
description: Learn to trace telemetry flow through connected components and defined data paths
menuTitle: Telemetry flow
title: Trace telemetry flow in Grafana Alloy
weight: 225
---

# Trace telemetry flow in {{% param "FULL_PRODUCT_NAME" %}}

Your {{< param "PRODUCT_NAME" >}} configuration defines components and the connections between them.
Those connections control how telemetry moves through the system.
When you trace data flow, you can predict behavior, tune performance, and verify that telemetry reaches the right destinations.

## Define every connection explicitly

There are no automatic transformations, no implicit pipelines, and no default processing.

- You define every connection between components.
- You specify every transformation, filter, and routing rule.
- Telemetry moves only along the paths you create.

This requires more explicit configuration but makes behavior predictable and easier to debug.
If telemetry changes, it's because a component in the configuration changed it.
If telemetry reaches a destination, it's because a path leads there.

## Follow the pipeline pattern

Telemetry flows through pipelines in four stages:

<!-- vale Grafana.Spelling = NO -->

{{< mermaid >}}
flowchart LR

  Discovery[Discovery]
  Ingestion[Ingestion]
  Transformation[Transformation]
  Output[Output]

  Discovery -.->|targets| Ingestion
  Ingestion -->|telemetry| Transformation
  Transformation -->|telemetry| Output

  %% Grafana styling
  classDef grafana fill:#ffffff,stroke:#F05A28,stroke-width:2px,rx:8,ry:8,color:#1f2937,font-weight:600;

  class Discovery,Ingestion,Transformation,Output grafana
{{< /mermaid >}}

<!-- vale Grafana.Spelling = YES -->

Discovery is optional.
Use it for pull-based collection when you need to find scrape targets dynamically.
Push-based ingestion and static configurations start at ingestion.

### Discover scrape targets

Discovery components find scrape targets and pass them to ingestion components.
They don't collect telemetry themselves.

Discovery components can find targets from:

- Kubernetes resources
- Cloud provider APIs
- Service registries
- Docker and container engines
- File-based configuration
- HTTP endpoints
- DNS records

### Collect or receive telemetry

Ingestion components receive or collect telemetry from external systems and convert it into internal formats.

They decode protocols and normalize data.
They don't filter, sample, or redact telemetry unless a component's documentation says otherwise.

If an ingestion component isn't connected to another component, its telemetry goes nowhere.

### Transform telemetry in the pipeline

Transformation components change telemetry between ingestion and output.

Transformation components can:

- Modify fields and labels
- Filter or drop telemetry
- Route telemetry to different components
- Sample traces

Connect an ingestion component directly to an output component and telemetry passes through without modification.

### Send telemetry to destinations

Output components forward telemetry to configured destinations.

For example, an output component might send data to:

- Grafana Mimir or any Prometheus-compatible endpoint with `prometheus.remote_write`
- Grafana Loki or compatible log storage with `loki.write`
- Grafana Tempo or any OTLP-compatible endpoint with `otelcol.exporter.otlp`
- Another telemetry collector with `otelcol.exporter.otlp`

## Map components to pipeline stages

Different component families use different naming conventions, but the flow pattern stays the same:

| Pipeline type | Discovery      | Ingestion            | Transformation        | Output                    |
| ------------- | -------------- | -------------------- | --------------------- | ------------------------- |
| OpenTelemetry | —              | `otelcol.receiver.*` | `otelcol.processor.*` | `otelcol.exporter.*`      |
| Prometheus    | `discovery.*`  | `prometheus.scrape`  | `prometheus.relabel`  | `prometheus.remote_write` |
| Loki          | `discovery.*`  | `loki.source.*`      | `loki.process`        | `loki.write`              |
| Pyroscope     | `discovery.*`  | `pyroscope.scrape`   | `pyroscope.relabel`   | `pyroscope.write`         |

This table shows common components.
Some components don't fit these categories.
For example, `prometheus.exporter.*` components expose metrics from local or remote systems as scrape targets rather than discover existing targets.
Refer to the [component reference](../reference/components/) for a complete list.

## Connect each signal type separately

You define metric, log, trace, and profile connections explicitly.
When a component supports multiple signal types, connect each type to the next component in its pipeline.

Many transformation components, especially `otelcol.processor.*` components, handle multiple signal types.
Changes to one signal type don't affect another.

OpenTelemetry pipelines often share components across signal types.
Prometheus, Loki, and Pyroscope pipelines are signal-specific.

## Branch and merge pipelines

Pipelines aren't limited to straight lines.

Telemetry can:

- Branch to multiple downstream components
- Merge into shared output components
- Stay isolated from other signal types

One ingestion component can feed one output, multiple outputs, or multiple transformation chains.
Separate ingestion components can stay isolated, share transformation components, or converge on a shared output.

## Trace data flow in a configuration

To see how telemetry moves in a configuration, trace the data path:

1. Identify discovery components and note which ingestion components receive their targets.
1. Identify ingestion components and determine what signal type each handles.
1. Trace where each component sends telemetry.
1. Review transformation components and what they change.
1. Identify where each path ends.

Explicit connections let you trace the data path in the configuration.
Connection order determines execution order, not the textual order of components in the file.

### Troubleshoot unexpected telemetry

When telemetry behaves unexpectedly:

- Verify that the ingestion component connects to other components.
- Trace the full path from ingestion to output.
- Confirm transformation components sit in the expected position.
- Ensure the path ends at the correct output component.

Unexpected behavior usually means an unexpected or missing connection.

The {{< param "PRODUCT_NAME" >}} UI can help you visualize these connections.
Refer to [Debug](../../troubleshoot/debug/) for more information.

## Next steps

- Explore the [Component reference](../../reference/components/) for detailed behavior of individual components
- Follow [Get started](../../get-started/) to learn {{< param "PRODUCT_NAME" >}} configuration syntax
- Use [Troubleshoot](../../troubleshoot/) to find solutions to common issues and debug techniques

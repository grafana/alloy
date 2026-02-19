---
canonical: https://grafana.com/docs/alloy/latest/introduction/telemetry/
description: Learn how Grafana Alloy moves telemetry through connected components and defined data paths
menuTitle: Telemetry flow
title: How Grafana Alloy moves telemetry
weight: 225
---

# How {{% param "FULL_PRODUCT_NAME" %}} moves telemetry

The {{< param "PRODUCT_NAME" >}} configuration defines components and the connections between them.
Those connections determine exactly how telemetry moves through the system.

Understanding telemetry flow makes it easier to reason about behavior, performance, and outcomes.

## The explicit model

{{< param "PRODUCT_NAME" >}} executes exactly what you configure.

- You define every connection between components.
- You specify every transformation, filter, and routing rule.
- Telemetry moves only along the paths you create.

This gives you precise control and makes behavior predictable.
If telemetry changes, it's because a component in the configuration changed it.
If telemetry reaches a destination, it's because a path leads there.

There are no automatic transformations, no implicit pipelines, and no hidden behavior.

## The pipeline pattern

Telemetry flows through pipelines following this pattern:

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

Discovery is optional.
It's used for pull-based collection when you need to find scrape targets dynamically.
Push-based ingestion and static configurations start directly at ingestion.

### Discovery

Discovery components find scrape targets and pass them to ingestion components.
They don't collect telemetry themselves.

Discovery components can find targets from:

- Kubernetes resources
- Cloud provider APIs
- Service registries
- Container runtimes
- File-based configuration
- HTTP endpoints
- DNS records

### Ingestion

Ingestion components accept telemetry from external systems and convert it into internal formats.

They handle protocol decoding and normalization, but don't perform semantic transformations such as filtering, sampling, or redaction unless explicitly documented for that component.

If an ingestion component isn't connected to another component, its telemetry goes nowhere.

### Transformation

Transformation components operate on telemetry after ingestion and before export.
They're the only place where telemetry changes.

Transformation components can:

- Modify fields and labels
- Filter or drop telemetry
- Route telemetry to different components
- Sample traces

If you connect an ingestion component directly to an output component, telemetry passes through without modification.

### Output

Output components forward telemetry to configured destinations.

An output component might send data to:

- A metrics backend, such as Grafana Mimir or any Prometheus-compatible endpoint, using `prometheus.remote_write`
- A log backend, such as Grafana Loki or any compatible log storage, using `loki.write`
- A tracing backend, such as Grafana Tempo or any OTLP-compatible endpoint, using `otelcol.exporter.otlp`
- Another telemetry collector using `otelcol.exporter.otlp`
- Another component within {{< param "PRODUCT_NAME" >}} using `forward_to` arguments

## Component names by pipeline type

Different component families use different naming conventions, but the underlying flow pattern remains the same:

| Pipeline type | Discovery      | Ingestion            | Transformation        | Output                    |
| ------------- | -------------- | -------------------- | --------------------- | ------------------------- |
| OpenTelemetry | —              | `otelcol.receiver.*` | `otelcol.processor.*` | `otelcol.exporter.*`      |
| Prometheus    | `discovery.*`  | `prometheus.scrape`  | `prometheus.relabel`  | `prometheus.remote_write` |
| Loki          | `discovery.*`  | `loki.source.*`      | `loki.process`        | `loki.write`              |
| Pyroscope     | `discovery.*`  | `pyroscope.scrape`   | `pyroscope.relabel`   | `pyroscope.write`         |

This table shows common components.
Some components don't fit neatly into these categories.
For example, `prometheus.exporter.*` components expose local metrics as scrape targets rather than discovering external targets.
Refer to the [component reference](../reference/components/) for a complete list.

## Signal types

Metric, log, trace, and profile connections are all defined explicitly.
If a component supports multiple signal types, each type needs its own explicit connection to the next component in its pipeline.

A processing component only affects its specific signal type.
A metric processor doesn't touch logs, and a log processor doesn't touch traces.

Each signal type typically has its own pipeline, defined independently in the configuration.

## Branch and merge patterns

Pipelines aren't limited to straight lines.

Telemetry can:

- Branch to multiple receiving components
- Merge into shared output components
- Remain isolated from other signal types

A single ingestion component may feed one output, multiple outputs, or multiple transformation chains.
Separate ingestion components may remain isolated, share transformation components, or converge on a shared output.

## Read configurations as data flow

To understand how telemetry flows in a configuration, trace the data path:

1. Identify any discovery components and note which ingestion components receive their targets.
1. Identify ingestion components and determine what signal type each handles.
1. Trace where each component sends telemetry.
1. Note transformation components and understand their behavior.
1. Identify where each path ends.

Explicit connections let you trace the data path in the configuration.
Connection order determines execution order, not the textual order of components in the configuration file.

### Troubleshoot

When telemetry behaves unexpectedly:

- Verify that the ingestion component connects to other components.
- Trace the full path from ingestion to output.
- Confirm transformation components are in the expected position.
- Ensure the path ends at the correct output component.

Unexpected behavior usually reflects an unexpected connection or a missing one.

The {{< param "PRODUCT_NAME" >}} UI can help visualize these connections.
Refer to [Debug](../troubleshoot/debug/) for more information.

## Next steps

- [Component reference](../reference/components/) - Detailed behavior of individual components.
- [Get started](../get-started/) - Learn the fundamentals of {{< param "PRODUCT_NAME" >}} configuration syntax.
- [Troubleshoot](../troubleshoot/) - Find solutions to common issues and debugging techniques.

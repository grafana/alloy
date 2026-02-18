---
canonical: https://grafana.com/docs/alloy/latest/telemetry-flow/
description: Learn how Grafana Alloy moves telemetry through connected components and defined data paths
menuTitle: Telemetry flow
title: How Grafana Alloy moves telemetry
weight: 25
---

# How {{% param "FULL_PRODUCT_NAME" %}} moves telemetry

The {{< param "PRODUCT_NAME" >}} configuration defines components and the connections between them.
Those connections determine exactly how telemetry moves through the system.

Understanding telemetry flow makes it easier to reason about behavior, performance, and outcomes.

## The explicit model

{{< param "PRODUCT_NAME" >}} doesn't apply hidden behavior.

- Telemetry moves only along the connections you define.
- If two components aren't connected, no data passes between them.
- There's no global pipeline or automatic chaining.

You must define every transformation, filtering decision, routing rule, or sampling policy in the configuration.
{{< param "PRODUCT_NAME" >}} executes exactly what the configuration defines.

This explicit model is intentional.
It gives you precise control and makes behavior predictable.

## The pipeline pattern

Telemetry flows through pipelines following this pattern:

{{< mermaid >}}
flowchart LR
  Discovery -.->|targets| Ingestion -->|telemetry| Transformation -->|telemetry| Output
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
- Static lists
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
They don't filter or transform data.
They forward whatever they receive.

An output component might send data to:

- A metrics backend, such as Mimir or Grafana Cloud, using `prometheus.remote_write`
- A log backend, such as Loki, using `loki.write`
- A tracing backend, such as Tempo or Jaeger, using `otelcol.exporter.otlp`
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

`prometheus.exporter.*` components expose local metrics as scrape targets.
They act as metric sources that `prometheus.scrape` can collect from, rather than discovering external targets.

## Signal types

Metric, log, and trace connections are all defined explicitly.
If a component supports multiple types of telemetry, each type needs to be explicitly connected to the next component in its pipeline.

A metric-processing component only affects metric data.
A log-processing component only affects log data.
A trace-processing component only affects trace data.

Each signal type typically has its own pipeline, defined independently in the configuration.

## Branching and merging

Pipelines aren't limited to straight lines.

Telemetry can:

- Branch to multiple receiving components
- Merge into shared output components
- Remain isolated from other signal types

A single ingestion component may feed one output, multiple outputs, or multiple transformation chains.
Separate ingestion components may remain isolated, share transformation components, or converge on a shared output.

## Reading configurations

To understand how telemetry flows in a configuration, trace the data path:

1. Identify any discovery components and note which ingestion components receive their targets.
1. Identify ingestion components and determine what signal type each handles.
1. Trace where each component sends telemetry.
1. Note transformation components and understand their behavior.
1. Identify where each path ends.

Because connections are explicit, the path is visible in the configuration.
Connection order determines execution order, not the textual order of components in the configuration file.

### Troubleshooting

When telemetry behaves unexpectedly:

- Verify that the ingestion component connects to other components.
- Trace the full path from ingestion to output.
- Confirm transformation components are in the expected position.
- Ensure the path ends at the correct output component.

Unexpected behavior usually reflects an unexpected connection or a missing one.

## Next steps

- [Component reference](../reference/components/) - Detailed behavior of individual components.
- [Get started](../get-started/) - Learn the fundamentals of {{< param "PRODUCT_NAME" >}} configuration syntax.
- [Troubleshoot](../troubleshoot/) - Find solutions to common issues and debugging techniques.

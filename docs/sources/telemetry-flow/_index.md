---
canonical: https://grafana.com/docs/alloy/latest/telemetry-flow/
description: Learn how Grafana Alloy moves telemetry through connected components and defined data paths
menuTitle: Telemetry flow
title: How Grafana Alloy moves telemetry
weight: 25
---

# How {{% param "FULL_PRODUCT_NAME" %}} moves telemetry

The {{< param "PRODUCT_NAME" >}} configuration defines components and how they connect.

Those connections determine exactly how telemetry moves through the system.

Telemetry doesn't move automatically.
It follows only the paths you define.
If two components aren't connected, no telemetry passes between them.
If there's no transformation along a path, it passes through unchanged.

Understanding how telemetry flows through connected components makes it easier to reason about behavior, performance, and outcomes.

For detailed behavior of individual components, refer to the [component reference](../reference/components/).

## How {{< param "PRODUCT_NAME" >}} starts

An {{< param "PRODUCT_NAME" >}} configuration declares components.
Each component has a specific role, such as receiving telemetry, processing it, or exporting it.

When {{< param "PRODUCT_NAME" >}} starts, it:

1. Instantiates the configured components.
1. Connects them according to their declared relationships.
1. Begins passing telemetry along those connections.

Telemetry flows from one component to the next along defined connections.
The configuration defines the direction and structure of that flow.

No global pipeline automatically handles all data.
Every path is explicit.

## Telemetry follows defined paths

Telemetry flows through the pipeline following a pattern like this:

{{< mermaid >}}
flowchart LR
  Discovery -.->|targets| Ingestion -->|telemetry| Transformation -->|telemetry| Output
{{< /mermaid >}}

Discovery is optional. It's used for pull-based collection when you need to find scrape targets dynamically. Push-based ingestion and static configurations start directly at ingestion.

This is a simplified representation of a single path.
In practice, configurations often branch, merge, and contain multiple independent telemetry paths.

Within any given path:

- **Discovery components** find scrape targets and pass them to ingestion components.
  They don't collect telemetry themselves.
- **Ingestion components** handle protocol decoding and normalization so {{< param "PRODUCT_NAME" >}} can represent telemetry internally.
  They don't perform semantic transformations such as filtering, sampling, or redaction unless explicitly documented for that component.
  They only handle ingestion, decoding, and normalization.
- **Transformation components** operate on telemetry while it's inside {{< param "PRODUCT_NAME" >}}.
- **Output components** forward telemetry to configured destinations, whether external systems or other components.

These roles are logical.
An ingestion component doesn't modify data unless you configure it to do so.
An output component doesn't filter data. It forwards whatever it receives.

If you connect an ingestion component directly to an output component, telemetry passes through without intermediate modification.

### Component names vary by pipeline type

Different component families use different naming conventions, but the underlying flow pattern remains the same:

| Pipeline type | Discovery      | Ingestion            | Transformation        | Output                    |
| ------------- | -------------- | -------------------- | --------------------- | ------------------------- |
| OpenTelemetry | —              | `otelcol.receiver.*` | `otelcol.processor.*` | `otelcol.exporter.*`      |
| Prometheus    | `discovery.*`  | `prometheus.scrape`  | `prometheus.relabel`  | `prometheus.remote_write` |
| Loki          | `discovery.*`  | `loki.source.*`      | `loki.process`        | `loki.write`              |
| Pyroscope     | `discovery.*`  | `pyroscope.scrape`   | `pyroscope.relabel`   | `pyroscope.write`         |

`prometheus.exporter.*` components expose local metrics as scrape targets. They act as metric sources that `prometheus.scrape` can collect from, rather than discovering external targets.

Regardless of naming, the conceptual flow is the same: discovery components find targets, ingestion components collect telemetry, transformation components process it, and output components forward it to destinations.

## Explicit configuration

{{< param "PRODUCT_NAME" >}} doesn't apply hidden behavior.

It doesn't:

- Automatically discover pipelines.
- Automatically parse log content.
- Automatically filter metrics.
- Automatically sample traces.
- Automatically redact or rewrite data.

You must define every transformation, filtering decision, routing rule, or sampling policy in the configuration.
This includes decisions such as dropping telemetry, rewriting labels, sampling traces, or routing data to different backends.

This explicit model is intentional.
It gives you precise control over how {{< param "PRODUCT_NAME" >}} handles telemetry and avoids hidden behavior.

## Multiple independent paths

A single configuration can contain multiple independent telemetry paths.

For example:

- One path collects metrics and sends them to a metrics backend.
- Another path collects logs and sends them to a log backend.
- A third path receives traces and forwards them elsewhere.

These paths can share components, or they can remain completely separate.
Their behavior depends entirely on how you connect them.

There's no requirement that all signals pass through the same path.

## Think in terms of data flow

When reading or writing a configuration, focus on how telemetry moves:

- Where does telemetry enter?
- Which components receive it?
- Which components does it pass through next?
- Where does it leave {{< param "PRODUCT_NAME" >}}?

Following those connections reveals exactly what {{< param "PRODUCT_NAME" >}} does at runtime.

## Next steps

- [Components and connections](./components-and-connections/) - Learn how components link together to define telemetry paths.
- [Telemetry pipelines](./pipelines/) - Understand how telemetry flows from ingestion through processing to export.
- [Where telemetry is modified](./modify-telemetry/) - Learn where and how telemetry changes.
- [Read configurations as data flow](./read-configurations/) - Interpret configurations by tracing telemetry paths.

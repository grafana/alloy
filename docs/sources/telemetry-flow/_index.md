---
canonical: https://grafana.com/docs/alloy/latest/telemetry-flow/
description: Learn how Grafana Alloy moves telemetry through connected components and defined data paths
menuTitle: Telemetry flow
title: How Grafana Alloy moves telemetry
weight: 25
---

# How {{% param "FULL_PRODUCT_NAME" %}} moves telemetry

{{< param "PRODUCT_NAME" >}} runs a configuration that defines components and how they connect.

Those connections determine exactly how telemetry moves through the system.

Telemetry doesn't move automatically.
It follows only the paths you define.
If two components aren't connected, no data passes between them.
If a processor isn't included in a path, no processing occurs.

Understanding how telemetry flows through connected components makes it easier to reason about behavior, performance, and outcomes.

For detailed behavior of individual components, refer to the [component reference](../reference/components/).

## Telemetry follows defined paths

In most configurations, telemetry follows a pattern like this:

Receiver → Processor → Exporter

This is a simplified representation of a single path.
In practice, configurations often branch, merge, and contain multiple independent telemetry paths.

Within any given path:

- **Receivers** handle protocol decoding and normalization so {{< param "PRODUCT_NAME" >}} can represent telemetry internally.
- **Processors** operate on telemetry while it's inside {{< param "PRODUCT_NAME" >}}.
- **Exporters** send telemetry to external systems.

If you connect a receiver directly to an exporter, telemetry passes through without intermediate modification.

## Explicit configuration

{{< param "PRODUCT_NAME" >}} doesn't apply hidden behavior.

It doesn't:

- Automatically discover pipelines.
- Automatically parse log content.
- Automatically filter metrics.
- Automatically sample traces.
- Automatically redact or rewrite data.

You must define every transformation, filtering decision, routing rule, or sampling policy in the configuration.

This explicit model gives you precise control and predictable behavior.

## Multiple independent paths

A single configuration can contain multiple independent telemetry paths.

For example:

- One path collects metrics and sends them to a metrics backend.
- Another path collects logs and sends them to a log backend.
- A third path receives traces and forwards them elsewhere.

These paths can share components, or they can remain completely separate.
Their behavior depends entirely on how you connect them.

## Think in terms of data flow

When reading or writing a configuration, focus on how telemetry moves:

1. Where does telemetry enter?
2. Which components receive it?
3. Which components does it pass through next?
4. Where does it leave {{< param "PRODUCT_NAME" >}}?

Following those connections reveals exactly what {{< param "PRODUCT_NAME" >}} does at runtime.

## Next steps

- [Components and connections](./components-and-connections/) - Learn how components link together to define telemetry paths.
- [Telemetry pipelines](./pipelines/) - Understand how telemetry flows from ingestion through processing to export.
- [Where telemetry is modified](./modify-telemetry/) - Learn where and how telemetry changes.
- [Read configurations as data flow](./read-configurations/) - Interpret configurations by tracing telemetry paths.

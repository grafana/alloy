---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/
description: Learn how Grafana Alloy processes telemetry through component graphs and defined data paths
menuTitle: Process telemetry
title: How Grafana Alloy processes telemetry
weight: 25
---

# How Alloy processes telemetry

Alloy runs a configuration that defines a graph of components.
That graph determines exactly how telemetry moves through the system.

Telemetry doesn't "flow through Alloy" in a generic or automatic way.
It moves along explicit connections between components.
If two components aren't connected, no data passes between them.
If a processing stage isn't configured, no processing occurs.

If you understand this execution model, it makes it much easier to reason about behavior, performance, and outcomes.

## Alloy executes a component graph

An Alloy configuration declares components.
Each component has a specific role, such as receiving telemetry, processing it, or exporting it.

When Alloy starts, it:

1. Instantiates the configured components.
1. Connects them according to their declared relationships.
1. Begins passing telemetry along those connections.

The result is a directed graph.
Telemetry flows from upstream components to downstream components.
The direction and shape of that flow are entirely defined by the configuration.

There is no global pipeline that automatically handles all data.
Every path is explicit.

## Telemetry enters, moves, and exits

In most configurations, telemetry follows a pattern like this:

Receiver → Processing → Exporter

- Receivers accept telemetry from external sources.
- Processing components operate on that telemetry while it is inside Alloy.
- Exporters send telemetry to external backends or systems.

These roles are logical, not magical.
A receiver doesn't modify data unless configured to do so.
An exporter doesn't filter data unless something upstream has filtered it.

If a configuration connects a receiver directly to an exporter, telemetry passes through without intermediate modification.

## Nothing happens unless you configure it

Alloy doesn't:

- Automatically discover telemetry pipelines.
- Automatically parse log content.
- Automatically filter metrics.
- Automatically sample traces.
- Automatically redact or rewrite data.

Every transformation, filter, or routing decision must be defined in the configuration.

This explicit model is intentional.
It gives you precise control over how telemetry is handled and avoids hidden behavior.

## Multiple pipelines can coexist

A single Alloy configuration can contain multiple independent flows.

For example:

- One pipeline may collect metrics and send them to a metrics backend.
- Another pipeline may collect logs and send them to a log backend.
- A third pipeline may receive traces and forward them elsewhere.

These pipelines may share components, or they may be completely separate. Their behavior depends entirely on how they are connected.

There is no requirement that all signals pass through the same path.

## Think in terms of data flow

When reading or writing an Alloy configuration, it helps to think in terms of data movement:

1. Where does telemetry enter?
1. Which components receive it?
1. Which components does it flow through next?
1. Where does it leave Alloy?

Following those connections reveals exactly what Alloy will do at runtime.

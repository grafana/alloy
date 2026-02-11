---
canonical: https://grafana.com/docs/alloy/latest/telemetry-flow/components-and-connections/
description: Learn how components connect in Grafana Alloy to define telemetry paths
menuTitle: Components and connections
title: Components and connections in Grafana Alloy
weight: 100
---

# Components and connections in {{% param "FULL_PRODUCT_NAME" %}}

An {{< param "PRODUCT_NAME" >}} configuration defines components and the connections between them.

Each component performs a specific function.
Connections determine how telemetry moves from one component to another.

Instead of thinking about a linear pipeline, think about connected paths.
Telemetry moves only along the connections you define.

## Components as building blocks

A component is a configured instance of a specific capability.

Depending on its type, a component might:

- Receive telemetry from an external source (receiver).
- Process telemetry already inside {{< param "PRODUCT_NAME" >}} (processor).
- Send telemetry to an external system (exporter).

Multiple instances of the same component type can exist in one configuration.
Each operates independently unless connected.

## Connections define behavior

Telemetry flows only where components are connected.

If two components aren't connected, they don't share data.
If a receiver has no downstream connection, its telemetry goes nowhere.
If a processor isn't in a path, it has no effect.

{{< param "PRODUCT_NAME" >}} executes exactly the connections defined in the configuration.
There is no hidden pipeline or automatic chaining of components.

## Branches and merges

Connections can form:

- Straight paths
- Branching paths that send telemetry to multiple downstream components
- Merged paths where multiple upstream components feed into a shared exporter

A configuration may contain several independent paths for different signal types.

## Reason about connections

When reviewing a configuration, ask:

1. Which components exist?
2. Which components connect to each other?
3. Which components have no downstream consumers?
4. Where does each path end?

Answering these questions reveals how telemetry moves through the system.

## Next steps

- [Telemetry pipelines](../pipelines/) - Learn how telemetry flows from ingestion through processing to export.
- [Where telemetry is modified](../modify-telemetry/) - Learn where and how telemetry changes.
- [Read configurations as data flow](../read-configurations/) - Interpret configurations by tracing telemetry paths.

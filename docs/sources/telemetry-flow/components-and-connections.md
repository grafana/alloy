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

- Receive telemetry from an external source.
- Process telemetry already inside {{< param "PRODUCT_NAME" >}}.
- Send telemetry to an external system.

Each component exposes defined inputs and outputs.
These interfaces determine how it can connect to other components.

Multiple instances of the same component type can exist in a single configuration.
Each instance operates independently unless you explicitly connect them.

## Connections define data flow

Telemetry flows only along declared connections.

If two components aren't connected, they don't share data.
There's no implicit global pipeline or automatic chaining of components.

Connections have direction:

- Upstream components send telemetry.
- Downstream components receive it.

The direction and shape of the flow come entirely from the configuration.

This explicit model makes telemetry flow predictable.
You can determine exactly where data goes by following connections.

## No hidden behavior

{{< param "PRODUCT_NAME" >}} doesn't infer connections.
It doesn't automatically insert processing stages.
It doesn't route telemetry unless you configure it to do so.

If a component isn't connected to anything downstream, nothing consumes its output.
If a receiver isn't connected to a processor or exporter, its telemetry doesn't go anywhere.

You must define every data path.
{{< param "PRODUCT_NAME" >}} executes exactly the connections defined in the configuration.

## Multiple independent flows

A single configuration can contain multiple independent flows.

For example:

- One set of components collects and exports metrics.
- Another set handles logs.
- A third handles traces.

These flows can share components, or they can remain completely separate.
The configuration determines whether data paths intersect or remain isolated.

There's no requirement that all telemetry types follow the same structure.

## Branches and merges

Connections can form:

- Straight paths
- Branching paths that send telemetry to multiple downstream components
- Merged paths where multiple upstream components feed into a shared exporter

## Reason about connections

When reviewing a configuration, ask:

- Which components exist?
- Which components connect to each other?
- Which components have no downstream consumers?
- Where does each path end?

Answering these questions reveals how telemetry moves through the system.

## Next steps

- [Telemetry pipelines](../pipelines/) - Learn how telemetry flows from ingestion through processing to export.
- [Where telemetry is modified](../modify-telemetry/) - Learn where and how telemetry changes.
- [Read configurations as data flow](../read-configurations/) - Interpret configurations by tracing telemetry paths.

---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/component-graph/
description: Learn about the Grafana Alloy component graph and how components connect to define data flow
menuTitle: Component graph
title: The Grafana Alloy component graph
weight: 100
---

# The Grafana Alloy component graph

An {{< param "PRODUCT_NAME" >}} configuration defines a graph of components.

Each component performs a specific function.
Connections between components determine how telemetry moves.
Together, they form a directed graph that {{< param "PRODUCT_NAME" >}} executes at runtime.

Thinking in terms of a graph—not a linear pipeline—helps explain how complex configurations behave.

## Components are building blocks

A component is a configured instance of a specific capability.

Depending on its type, a component might:

- Receive telemetry from an external source.
- Process telemetry already inside {{< param "PRODUCT_NAME" >}}.
- Export telemetry to an external system.

Each component exposes defined inputs and outputs.
These interfaces determine how it can connect to other components.

Multiple instances of the same component type can exist in a single configuration.
Each instance operates independently unless you explicitly connect them.

## Connections define data flow

Telemetry flows only along declared connections.

If two components aren't connected, they don't share data.
There is no implicit global pipeline or automatic chaining of components.

Connections create directed edges:

- Upstream components send telemetry.
- Downstream components receive it.

The direction and shape of the graph come entirely from the configuration.

This explicit model makes telemetry flow predictable.
You can determine exactly where data goes by following connections.

## No hidden behavior

{{< param "PRODUCT_NAME" >}} doesn't infer connections.
It doesn't automatically insert processing stages.
It doesn't route telemetry unless you configure it to do so.

If a component isn't connected to anything downstream, its output isn't consumed.
If a receiver isn't connected to a processor or exporter, its telemetry doesn't go anywhere.

Every data path must be defined.

## Multiple graphs in one configuration

A single configuration can contain multiple independent flows.

For example:

- One set of components collects and exports metrics.
- Another set handles logs.
- A third handles traces.

These flows can share components, or they can remain completely separate.
The configuration determines whether data paths intersect or remain isolated.

There's no requirement that all telemetry types follow the same structure.

## Reasoning about the graph

When reviewing a configuration, focus on:

1. Which components exist?
1. Which components are connected?
1. Which components have no downstream consumers?
1. Where does each path terminate?

Following those edges reveals how {{< param "PRODUCT_NAME" >}} executes the configuration.

Next, explore how these graph structures form complete telemetry pipelines from ingestion to export.
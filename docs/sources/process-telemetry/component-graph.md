---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/component-graph/
description: Learn about the Grafana Alloy component graph and how components connect to define data flow
menuTitle: Component graph
title: The Grafana Alloy component graph
weight: 100
---

# The Grafana Alloy component graph

Purpose

Give users a mental model they can apply when reading configs.

Content Outline

1. Components are building blocks

Each component performs a specific function.

Components expose typed inputs and outputs.

Multiple instances of the same component type can exist.

2. Connections define data flow

Data flows only where components are connected.

No global pipeline exists.

Separate pipelines can coexist.

3. Multiple pipelines in one configuration

Logs, metrics, and traces may have separate chains.

One config can contain multiple independent flows.

Avoid syntax-heavy examples. Focus on conceptual diagrams.

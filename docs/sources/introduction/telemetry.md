---
canonical: https://grafana.com/docs/alloy/latest/introduction/telemetry/
description: Learn how Grafana Alloy moves telemetry through connected components and defined data paths
menuTitle: Telemetry flow
title: How telemetry flows through Grafana Alloy
weight: 225
---

# How telemetry flows through {{% param "FULL_PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} moves telemetry through connected components, from sources to backends.
Your configuration defines those components and the connections between them.
Follow those connections to see where data enters the pipeline and where it ends up.

## Define every connection explicitly

{{< param "PRODUCT_NAME" >}} doesn't transform, route, or process telemetry unless your configuration tells it to.
You define every connection between components, and telemetry moves only along the paths you create.

A changed component alters the telemetry it handles.
A missing connection means data doesn't reach the next stage.

Components connect through exports, receiver references, and attributes such as `forward_to`.
[Build data pipelines](../get-started/components/build-pipelines/) explains how these connections form pipelines.

## Follow the pipeline stages

Connected components form pipelines.
Most telemetry pipelines use some combination of four roles: discovery, ingestion, transformation, and output.
Discovery and transformation are optional, and you can chain multiple components in the same role or branch to multiple outputs.

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

In pull-based pipelines, discovery components pass scrape targets to ingestion components such as `prometheus.scrape`.
OpenTelemetry pipelines start at `otelcol.receiver.*` and skip discovery.

Ingestion collects or receives telemetry and converts it to an internal format.
Transformation components modify, filter, route, or sample that data.
You can also connect ingestion directly to output and skip transformation.

Output components send telemetry to backends.

When a component supports multiple signal types, connect each type separately through the pipeline.

[Build data pipelines](../get-started/components/build-pipelines/) has multi-stage examples and pipeline patterns.
[Choose a component](../collect/choose-component/) helps you pick components by signal type.

## See how telemetry moves in a configuration

When you read a configuration, follow the data path from source to destination:

1. Start at ingestion components and note what signal type each one handles.
1. If the pipeline uses discovery, follow targets from discovery components into ingestion.
1. Follow each component's output to the next component in the chain.
1. Note any transformation components and what they change.
1. Identify where each path ends at an output component.

Connection order determines execution order, not the textual order of components in the file.
Pipelines can branch to multiple outputs or share components across paths.
[Pipeline patterns](../get-started/components/build-pipelines/#pipeline-patterns) covers fan-out and chain processing.

The {{< param "PRODUCT_NAME" >}} UI visualizes these connections.
Use [Debug](../troubleshoot/debug/) to inspect component pipelines in a running instance.

## Next steps

- Start with [Get started](../get-started/) for configuration syntax and component basics.
- Use [Build data pipelines](../get-started/components/build-pipelines/) to connect components and apply pipeline patterns.
- Use [Choose a component](../collect/choose-component/) to pick components for metrics, logs, traces, and profiles.
- Follow [Collect and forward data](../collect/) for end-to-end collection examples.
- Read [How Alloy works](./how-alloy-works/) for architecture and capabilities.
- Look up behavior in the [Component reference](../reference/components/).

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
Understanding that flow helps you read configurations, build pipelines, and know where your data goes.

## Define every connection explicitly

There are no automatic transformations, implicit pipelines, or default processing.
You define every connection between components, and telemetry moves only along the paths you create.

If telemetry changes, a component in your configuration changed it.
If telemetry reaches a destination, a path leads there.

Components connect through exports, receiver references, and attributes such as `forward_to`.
Refer to [Build data pipelines](../get-started/components/build-pipelines/) to learn how these connections form pipelines.

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

**Discovery** components find scrape targets for pull-based ingestion components, such as `prometheus.scrape`.
OpenTelemetry pipelines start at `otelcol.receiver.*` components and skip discovery.

**Ingestion** components collect or receive telemetry and convert it to an internal format.

**Transformation** components modify, filter, route, or sample telemetry.
Connect an ingestion component directly to an output component to skip transformation.

**Output** components forward telemetry to backends.

When a component supports multiple signal types, connect each type separately through the pipeline.

Refer to [Build data pipelines](../get-started/components/build-pipelines/) for multi-stage examples and pipeline patterns.
Refer to [Choose a component](../collect/choose-component/) to select components by signal type.

## See how telemetry moves in a configuration

When you read a configuration, follow the data path from source to destination:

1. Start at ingestion components and note what signal type each one handles.
1. If the pipeline uses discovery, follow targets from discovery components into ingestion.
1. Follow each component's output to the next component in the chain.
1. Note any transformation components and what they change.
1. Identify where each path ends at an output component.

Connection order determines execution order, not the textual order of components in the file.
Pipelines can branch to multiple outputs or share components across paths.
Refer to [Pipeline patterns](../get-started/components/build-pipelines/#pipeline-patterns) for fan-out and chain processing examples.

The {{< param "PRODUCT_NAME" >}} UI visualizes these connections.
Refer to [Debug](../troubleshoot/debug/) to inspect component pipelines in a running instance.

## Next steps

- Refer to [Get started](../get-started/) to learn configuration syntax and component basics.
- Refer to [Build data pipelines](../get-started/components/build-pipelines/) to connect components and use pipeline patterns.
- Refer to [Choose a component](../collect/choose-component/) to select components for metrics, logs, traces, and profiles.
- Refer to [Collect and forward data](../collect/) for end-to-end collection examples.
- Refer to [How Alloy works](./how-alloy-works/) for architecture and capabilities.
- Refer to the [Component reference](../reference/components/) for detailed component behavior.

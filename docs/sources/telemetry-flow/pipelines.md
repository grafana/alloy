---
canonical: https://grafana.com/docs/alloy/latest/telemetry-flow/pipelines/
description: Learn how telemetry flows through Grafana Alloy from ingestion to export
menuTitle: Pipelines
title: Telemetry pipelines in Grafana Alloy
weight: 200
---

# Telemetry pipelines in {{% param "FULL_PRODUCT_NAME" %}}

Telemetry moves through {{< param "PRODUCT_NAME" >}} in defined paths.
Each path begins with ingestion, may include processing stages, and ends with export.

You construct these paths by connecting components in the configuration.
There's no default pipeline.
The configuration defines every stage.

A common pipeline looks like this:

{{< mermaid >}}
flowchart LR
  Ingestion --> Transformation --> Output
{{< /mermaid >}}

That pattern is conceptual.
The actual structure depends entirely on how you connect the components.

## Ingestion

Telemetry enters {{< param "PRODUCT_NAME" >}} through ingestion components.

Ingestion components accept telemetry from external systems such as:

- Applications emitting telemetry.
- Infrastructure exposing metrics.
- Log sources.
- Other telemetry collectors.

Ingestion components convert incoming data into the internal formats used within {{< param "PRODUCT_NAME" >}}.
From that point forward, telemetry moves between components inside the configured paths.

If an ingestion component has no downstream connection, its telemetry goes nowhere.

## Transformation

Transformation components operate on telemetry after ingestion and before export.

They sit between ingestion and output components in the path.
If you include transformation components in a path, telemetry flows through them.
If you don't, telemetry moves directly to the output component unchanged.

Transformation components can:

- Modify telemetry.
- Filter telemetry.
- Route telemetry to different downstream components.

Transformation only happens when you connect a transformation component in the path.

## Output

Output components forward telemetry to their configured destinations, whether that's an external system or another component within {{< param "PRODUCT_NAME" >}}.

An output component might send data to:

- A metrics backend.
- A log backend.
- A tracing backend.
- Another telemetry collector.
- Another component within {{< param "PRODUCT_NAME" >}}.

Output components are often the final stage in a pipeline, but they can also connect to other components, allowing you to chain pipelines together.

A pipeline can include:

- One output component.
- Multiple output components.
- No output components, in which case telemetry never leaves.

## Parallel and branching pipelines

A pipeline isn't limited to a straight line.

Because the configuration defines connected paths, telemetry can:

- Branch to multiple downstream components.
- Merge into shared output components.
- Remain isolated from other signal types.

For example:

- Metrics can flow through one path.
- Logs can flow through another.
- Traces can follow a third.

Each signal type typically has its own pipeline, defined independently in the configuration.

## Observe pipeline structure

To understand how telemetry flows in a configuration:

1. Identify the ingestion components.
1. Trace their downstream connections.
1. Note each transformation component in the path.
1. Identify where the path ends.

That path is the pipeline.

## Next steps

- [Where telemetry is modified](../modify-telemetry/) - Learn where and how telemetry changes.
- [Read configurations as data flow](../read-configurations/) - Interpret configurations by tracing telemetry paths.

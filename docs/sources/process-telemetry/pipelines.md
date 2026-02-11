---
canonical: https://grafana.com/docs/alloy/latest/process-telemetry/pipelines/
description: Learn how telemetry flows through Grafana Alloy pipelines from ingestion to export
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

A common shape looks like this:

Receiver → Processor → Exporter

That pattern is conceptual.
The actual structure depends entirely on how you connect the components.

## Ingestion

Telemetry enters {{< param "PRODUCT_NAME" >}} through receiver components.

Receivers accept telemetry from external systems such as:

- Applications emitting telemetry.
- Infrastructure exposing metrics.
- Log sources.
- Other telemetry collectors.

Receivers convert incoming data into the internal formats used within {{< param "PRODUCT_NAME" >}}.
From that point forward, telemetry moves between components inside the configured graph.

If a receiver has no downstream connection, its telemetry goes nowhere.

## Processors

Processors operate on telemetry after ingestion and before export.

They sit between receivers and exporters in the graph.
If you include processors in a path, telemetry flows through them.
If you don't, telemetry moves directly to the exporter unchanged.

Processing can:

- Modify telemetry.
- Filter telemetry.
- Route telemetry to different downstream components.

Processing only happens when you connect a processing component in the path.

## Export

Exporters send telemetry from {{< param "PRODUCT_NAME" >}} to external systems.

An exporter might send data to:

- A metrics backend.
- A log backend.
- A tracing backend.
- Another telemetry endpoint.

If telemetry reaches an exporter, it leaves {{< param "PRODUCT_NAME" >}} through that component.

A pipeline can include:

- One exporter.
- Multiple exporters.
- No exporters, in which case telemetry never leaves.

## Parallel and branching pipelines

A pipeline isn't limited to a straight line.

Because the configuration defines a graph, telemetry paths can:

- Branch to multiple downstream components.
- Merge into shared exporters.
- Remain isolated from other signal types.

For example:

- Metrics can flow through one path.
- Logs can flow through another.
- Traces can follow a third.

Each signal type typically has its own pipeline, defined independently in the configuration.

## Observe pipeline structure

To understand how telemetry flows in a configuration:

1. Identify the receivers.
1. Trace their downstream connections.
1. Note each processing component in the path.
1. Identify where the path ends.

That path is the pipeline.

## Next steps

- [Where telemetry is modified](../modify-telemetry/) - Understand where modification occurs in processing stages.
- [Read configurations as data flow](../read-configurations/) - Interpret configurations using the graph model.

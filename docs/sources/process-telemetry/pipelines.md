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

These paths are constructed from connected components in the configuration.
There is no default pipeline.
The configuration defines every stage.

A common shape looks like this:

Receiver → Processing → Exporter

That pattern is conceptual.
The actual structure depends entirely on how components are connected.

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

## Processing

Processing components operate on telemetry after ingestion and before export.

They sit between receivers and exporters in the graph.
If you include processing components in a path, telemetry flows through them.
If you do not, telemetry moves directly to the exporter unchanged.

Processing can:

- Modify telemetry.
- Filter telemetry.
- Route telemetry to different downstream components.

Nothing is processed unless a processing component is connected in the path.

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

A pipeline is not limited to a straight line.

Because the configuration defines a graph, telemetry paths can:

- Branch to multiple downstream components.
- Merge into shared exporters.
- Remain isolated from other signal types.

For example:

- Metrics can flow through one path.
- Logs can flow through another.
- Traces can follow a third.

Each signal type typically has its own pipeline, defined independently in the configuration.

## Observing pipeline structure

To understand how telemetry flows in a configuration:

1. Identify the receivers.
1. Trace their downstream connections.
1. Note each processing component in the path.
1. Identify where the path terminates.

That path is the pipeline.

Next, examine where telemetry is modified within those pipelines and how processing stages affect data before export.

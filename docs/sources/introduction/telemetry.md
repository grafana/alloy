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
Most telemetry pipelines use some combination of four functions: discovery, ingestion, transformation, and output.
Discovery and transformation are optional, and you can chain multiple components in the same function or branch to multiple outputs.

<!-- vale Grafana.WordList = NO -->
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
<!-- vale Grafana.WordList = YES -->

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

## Delivery failures

Output components don't always discard telemetry the moment a backend becomes unavailable.
Most retry with backoff for a bounded number of attempts or amount of time before giving up on a piece of data.
What happens during and after those retries depends on the component:

- **`loki.write`**: Buffers log entries in an internal send queue and blocks by default. When the queue fills, it stops accepting new entries until space frees up, which applies backpressure to earlier stages in the log pipeline. Batches that fail to send are retried separately with backoff, and only dropped once that retry limit is reached, or if you explicitly configure the queue not to block.
- **`prometheus.remote_write`**: Writes samples to a local WAL before sending them, so scraping continues uninterrupted during an outage. The remote-write queue retries from the WAL until the backend recovers or the WAL truncates old data. Refer to [Data durability](../requirements/#data-durability) for how long the WAL retains unsent samples.
- **`otelcol.exporter.*`**: Components such as `otelcol.exporter.otlp` buffer in a send queue and drop data by default once that queue fills, so earlier pipeline stages keep running uninterrupted. You can configure them to block instead, or to persist the queue to disk so it survives a restart.
- **`pyroscope.write`**: Doesn't queue at all. Each push is a synchronous call that retries with backoff inline, blocking the calling component (such as `pyroscope.scrape`) for the duration of the retries. Once retries are exhausted, that batch of profile data is dropped and the call returns.

Retry limits, buffering, and the choice between blocking and dropping vary by component.
Refer to each component's reference page for its specific behavior and configuration: [`loki.write`](../reference/components/loki/loki.write/), [`prometheus.remote_write`](../reference/components/prometheus/prometheus.remote_write/), [`otelcol.exporter.otlp`](../reference/components/otelcol/otelcol.exporter.otlp/), and [`pyroscope.write`](../reference/components/pyroscope/pyroscope.write/).

{{< admonition type="note" >}}
Component health in the {{< param "PRODUCT_NAME" >}} UI doesn't always reflect delivery failures or dropped data.
Check each component's Prometheus metrics, such as dropped-record counters, to confirm data loss.
{{< /admonition >}}

## Upstream failures

A component that fails to apply a configuration change doesn't clear its exports.
Downstream components keep using its last exported data indefinitely, until it recovers and exports something new, there's no timeout.

This means an unhealthy component doesn't necessarily stop a pipeline.
For example, if `discovery.kubernetes` can't reach the API server, `prometheus.scrape` keeps scraping its last known target list, now stale.

A component's health status and whether it's still processing data are different things:

- **Unhealthy**: The component's most recent configuration update failed, or the component reported an error itself. Its `Run` loop keeps executing, so it can still be processing and forwarding data.
- **Exited**: The component's `Run` loop returned, cleanly or with an error. Processing has actually stopped.

An unhealthy component is worth investigating, but it hasn't necessarily stopped moving data, check whether it exited instead.

## Fan-out to multiple destinations

When a component forwards telemetry to more than one destination, for example a `prometheus.relabel` sending to two `prometheus.remote_write` components, the sends happen synchronously, one destination after another, in the same call.
A slow or blocked destination delays delivery to the other destinations in the same fan-out, because they aren't sent to independently.

Keep this in mind when destinations have different reliability or latency: a struggling one can affect the others sharing the same source component.

`pyroscope.write` is the exception: with multiple `endpoint` blocks, each sends and retries independently, so a slow endpoint doesn't delay the others.
The call still doesn't return upstream until every endpoint finishes, so the slowest one still determines how long the caller blocks.

## Shutdown

When {{< param "PRODUCT_NAME" >}} shuts down, it cancels each component's context and stops components in dependency order, rather than killing all of them at once.
Each component has time to exit cleanly: output components use that time to flush what they can, for example `loki.write` drains its send queue up to its configured `drain_timeout`, and `prometheus.remote_write` flushes its queue manager before closing.

By default, a component has up to 10 minutes to exit before {{< param "PRODUCT_NAME" >}} gives up and logs an error.
Configure this with [`--feature.component-shutdown-deadline`](../reference/cli/run/).

## Next steps

- Start with [Get started](../get-started/) for configuration syntax and component basics.
- Use [Build data pipelines](../get-started/components/build-pipelines/) to connect components and apply pipeline patterns.
- Use [Choose a component](../collect/choose-component/) to pick components for metrics, logs, traces, and profiles.
- Follow [Collect and forward data](../collect/) for end-to-end collection examples.
- Read [How Alloy works](./how-alloy-works/) for architecture and capabilities.
- Look up behavior in the [Component reference](../reference/components/).

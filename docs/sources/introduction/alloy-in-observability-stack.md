---
canonical: https://grafana.com/docs/alloy/latest/introduction/alloy-in-observability-stack/
description: Understand where Grafana Alloy fits in your observability architecture
menuTitle: Alloy in the observability stack
title: Grafana Alloy in the observability stack
weight: 100
---

# {{% param "FULL_PRODUCT_NAME" %}} in the observability stack

Understanding where {{< param "PRODUCT_NAME" >}} fits in your observability architecture helps clarify its role and how it works with other tools.

## The observability architecture

A typical observability setup has three main layers:

1. **Data sources**: Systems that generate telemetry
1. **Collection**: Tools that gather and process telemetry
1. **Storage and visualization**: Databases and frontends for querying and exploring data

{{< param "PRODUCT_NAME" >}} operates in the collection layer, sitting between your data sources and your storage backends.

<!-- TODO: Add a diagram showing the observability architecture? -->

## What {{< param "PRODUCT_NAME" >}} does

{{< param "PRODUCT_NAME" >}} acts as the bridge between data sources and storage backends.

It performs three main functions:

### Collect telemetry data

{{< param "PRODUCT_NAME" >}} gathers telemetry from various sources using multiple methods:

- **Scraping**: Pull metrics from Prometheus endpoints
- **Receiving**: Accept pushed data via OpenTelemetry protocol or OTLP
- **Tailing**: Read logs from files or system outputs
- **Discovering**: Find services automatically in Kubernetes, Docker, or cloud environments
- **Integrating**: Collect from databases, message queues, and other systems

### Transform and process data

{{< param "PRODUCT_NAME" >}} processes telemetry before sending it:

- **Filtering**: Remove unwanted data to reduce costs
- **Enriching**: Add labels, metadata, or context
- **Sampling**: Reduce high-volume data while maintaining signal
- **Converting**: Transform between formats like Prometheus to OTLP
- **Routing**: Send different data to different destinations based on rules

### Send to backends

{{< param "PRODUCT_NAME" >}} delivers telemetry to your storage systems:

- Send to Grafana Cloud
- Export to self-hosted Grafana stack components
- Forward to any Prometheus-compatible database
- Push to any OpenTelemetry-compatible backend
- Write to multiple destinations simultaneously

## What {{< param "PRODUCT_NAME" >}} replaces

{{< param "PRODUCT_NAME" >}} can replace or consolidate several types of collectors:

### Prometheus components

- **Prometheus Agent**: {{< param "PRODUCT_NAME" >}} includes all Prometheus agent functionality
- **Prometheus Server for collection**: Use {{< param "PRODUCT_NAME" >}} for scraping and remote write, with remote storage for queries
- **Prometheus node exporter with transformation**: {{< param "PRODUCT_NAME" >}} can collect the same metrics with additional processing

### OpenTelemetry components

- **OpenTelemetry Collector**: {{< param "PRODUCT_NAME" >}} is a distribution of the Collector with additional capabilities
- **OpenTelemetry SDKs for infrastructure**: {{< param "PRODUCT_NAME" >}} can generate telemetry from systems without code instrumentation

### Other collectors

- **Grafana Agent**: {{< param "PRODUCT_NAME" >}} is the successor to Grafana Agent
- **Log collectors**: Replace tools like Promtail, Fluentd, or Filebeat for log collection
- **Specialized exporters**: The integrations in {{< param "PRODUCT_NAME" >}} replace many single-purpose exporters

You don't need to replace everything at once.
{{< param "PRODUCT_NAME" >}} can run alongside existing collectors during migration.

## How {{< param "PRODUCT_NAME" >}} integrates with the Grafana ecosystem

{{< param "PRODUCT_NAME" >}} is designed to work seamlessly with Grafana's observability stack.

### Grafana Cloud

Send telemetry directly to Grafana Cloud:

- Metrics to Grafana Cloud Metrics powered by Mimir
- Logs to Grafana Cloud Logs powered by Loki
- Traces to Grafana Cloud Traces powered by Tempo
- Profiles to Grafana Cloud Profiles powered by Pyroscope

{{< param "PRODUCT_NAME" >}} handles authentication and configuration for Grafana Cloud endpoints.

### Self-hosted Grafana stack

Deploy {{< param "PRODUCT_NAME" >}} with self-hosted components:

- **Loki**: Send logs for aggregation and querying
- **Mimir**: Send metrics for long-term storage and queries
- **Tempo**: Send traces for distributed tracing analysis
- **Pyroscope**: Send profiles for continuous profiling

### Grafana dashboards

{{< param "PRODUCT_NAME" >}} exposes its own metrics that you can visualize in Grafana:

- Monitor performance
- Track collection rates and volumes
- Debug pipeline issues
- Use pre-built dashboards from the mixin

## How {{< param "PRODUCT_NAME" >}} works with open source ecosystems

{{< param "PRODUCT_NAME" >}} embraces vendor neutrality and open standards.

### Prometheus ecosystem

- Compatible with Prometheus exposition format
- Supports Prometheus service discovery mechanisms
- Uses Prometheus remote write protocol
- Works with Prometheus-compatible databases like Thanos, Cortex, and VictoriaMetrics

### OpenTelemetry ecosystem

- Distribution of OpenTelemetry Collector
- Supports OTLP for metrics, logs, and traces
- Compatible with OpenTelemetry instrumentation
- Works with any OTLP-compatible backend

### Other ecosystems

- **InfluxDB**: Send metrics using Telegraf or native formats
- **Elasticsearch**: Send logs via various output formats
- **Cloud platforms**: Native integrations for AWS, GCP, Azure

## Deployment patterns

{{< param "PRODUCT_NAME" >}} adapts to different architectural needs.

### Edge collection

Deploy {{< param "PRODUCT_NAME" >}} close to data sources:

- Run as a DaemonSet in Kubernetes to collect from every node
- Install on each host for infrastructure monitoring
- Deploy alongside applications for local processing

### Gateway pattern

Deploy {{< param "PRODUCT_NAME" >}} as a central collection point:

- Applications send telemetry to {{< param "PRODUCT_NAME" >}} gateways
- Gateways process and forward to backends
- Simplifies backend configuration in applications

### Hybrid approach

Combine edge and gateway deployments:

- Edge instances do initial collection and filtering
- Gateway instances handle aggregation and final processing
- Reduces bandwidth and allows centralized policy enforcement

## What {{< param "PRODUCT_NAME" >}} doesn't replace

{{< param "PRODUCT_NAME" >}} is a collector, not a complete observability platform.

You still need:

- **Storage backends**: {{< param "PRODUCT_NAME" >}} doesn't store telemetry long-term
- **Visualization tools**: Use Grafana or other frontends to query and explore data
- **Instrumentation**: Applications still need to expose metrics or use OpenTelemetry SDKs
- **Exporters**: Specialized exporters might still be needed for very specific integrations

{{< param "PRODUCT_NAME" >}} complements these tools rather than replacing them.

## Example architecture

<!-- TODO: Add an application stack diagram? -->

Applications instrumented with OpenTelemetry SDKs send data via OTLP to {{< param "PRODUCT_NAME" >}} running as a DaemonSet in Kubernetes.
{{< param "PRODUCT_NAME" >}} collects from OTLP receivers, Prometheus endpoints, log files, and the Kubernetes API.
It transforms the data by filtering low-value data, adding cluster labels, and sampling traces.
The processed telemetry is sent to Grafana Cloud or a self-hosted stack, where it flows to Loki for logs, Mimir for metrics, Tempo for traces, and Pyroscope for profiles.
You can then query and visualize this data in Grafana Dashboard.

## Next steps

- Learn [when to use {{< param "PRODUCT_NAME" >}}][when] for your specific needs
- Understand the [components and architecture][concepts]
- Review [deployment patterns][deploy] for different environments
- Check [migration guides][migrate] for moving from other collectors

[when]: ../when-to-use-alloy/
[concepts]: ../../get-started/
[deploy]: ../../set-up/deploy/
[migrate]: ../../set-up/migrate/

---
canonical: https://grafana.com/docs/alloy/latest/introduction/alloy-in-observability-stack/
description: Understand where Grafana Alloy fits in your observability architecture
menuTitle: Alloy in the observability stack
title: Grafana Alloy in the observability stack
weight: 300
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

{{< param "PRODUCT_NAME" >}} acts as the bridge between data sources and storage backends, performing three main functions in your telemetry pipeline.

### Collect telemetry data

Use {{< param "PRODUCT_NAME" >}} to gather telemetry from any source in your infrastructure.
Configure it to scrape Prometheus endpoints for metrics, or set up receivers to accept pushed data via the OpenTelemetry protocol.
Tail log files or read from system outputs to capture application and infrastructure logs.
Enable service discovery to automatically find resources in Kubernetes, Docker, or cloud environments without maintaining static configuration.
Integrate with databases, message queues, and other systems to capture telemetry from specialized sources.

### Transform and process data

Process your telemetry before sending it to backends to optimize costs and improve data quality.
Create filters to remove unwanted data and reduce storage costs while focusing on high-value telemetry.
Add labels, metadata, or contextual information to enrich your data and make it more useful for analysis.
Implement sampling strategies to reduce high-volume data while preserving the signal you need for troubleshooting.
Convert between formats, such as transforming Prometheus metrics to OpenTelemetry format, to ensure compatibility with your backends.
Define routing rules to send different types of data to different destinations based on your operational requirements.

### Send to backends

Configure {{< param "PRODUCT_NAME" >}} to deliver processed telemetry to any storage system you choose.
Send data to Grafana Cloud for managed observability, or export to your self-hosted Grafana stack components.
Connect to any Prometheus-compatible database for metrics and any OpenTelemetry-compatible backend for all signal types.
Write to multiple destinations simultaneously, sending the same data to different systems or routing different data types to specialized backends based on your architecture.

## What {{< param "PRODUCT_NAME" >}} replaces

Consolidate your observability collectors by replacing multiple tools with {{< param "PRODUCT_NAME" >}}.

Replace Prometheus Agent with {{< param "PRODUCT_NAME" >}} to gain all the same functionality plus support for logs, traces, and profiles.
Migrate from Prometheus Server used primarily for collection by using {{< param "PRODUCT_NAME" >}} for scraping and remote write while keeping remote storage for queries.
Switch from Prometheus node exporter to {{< param "PRODUCT_NAME" >}} to collect the same infrastructure metrics with added processing capabilities.

Replace your OpenTelemetry Collector deployment with {{< param "PRODUCT_NAME" >}} to add native Prometheus support alongside OTLP.
Generate telemetry from systems without code instrumentation, replacing the need for OpenTelemetry SDKs in infrastructure components.

Migrate from Grafana Agent to {{< param "PRODUCT_NAME" >}} for enhanced capabilities and a more powerful configuration model.
Replace specialized log collectors like Promtail, Fluentd, or Filebeat with {{< param "PRODUCT_NAME" >}}'s unified collection approach.
Eliminate many single-purpose exporters by using the integrations built into {{< param "PRODUCT_NAME" >}}.

Run {{< param "PRODUCT_NAME" >}} alongside existing collectors during migration to transition gradually without disrupting your observability.

## How {{< param "PRODUCT_NAME" >}} integrates with the Grafana ecosystem

Connect {{< param "PRODUCT_NAME" >}} to Grafana's observability stack whether you use Grafana Cloud or self-hosted components.

Configure {{< param "PRODUCT_NAME" >}} to send telemetry directly to Grafana Cloud.
Route metrics to Grafana Cloud Metrics powered by Mimir, logs to Grafana Cloud Logs powered by Loki, traces to Grafana Cloud Traces powered by Tempo, and profiles to Grafana Cloud Profiles powered by Pyroscope.
Use built-in authentication and endpoint configuration to simplify your setup.

Deploy {{< param "PRODUCT_NAME" >}} with your self-hosted Grafana stack.
Send logs to Loki for aggregation and querying, metrics to Mimir for long-term storage, traces to Tempo for distributed tracing analysis, and profiles to Pyroscope for continuous profiling.

Visualize {{< param "PRODUCT_NAME" >}}'s own metrics in Grafana dashboards.
Monitor collector performance, track collection rates and volumes, debug pipeline issues, and use pre-built dashboards from the mixin to observe your collector infrastructure.

## How {{< param "PRODUCT_NAME" >}} works with open source ecosystems

Integrate {{< param "PRODUCT_NAME" >}} with any observability ecosystem through open standards.

Use {{< param "PRODUCT_NAME" >}} with the Prometheus ecosystem through full compatibility with the Prometheus exposition format and service discovery mechanisms.
Configure Prometheus remote write to send data to any Prometheus-compatible database, including Thanos, Cortex, and VictoriaMetrics.

Deploy {{< param "PRODUCT_NAME" >}} as your OpenTelemetry Collector distribution.
Receive OTLP data for metrics, logs, and traces from any OpenTelemetry instrumentation.
Send to any OTLP-compatible backend to maintain flexibility in your tool choices.

Connect {{< param "PRODUCT_NAME" >}} to other ecosystems beyond Prometheus and OpenTelemetry.
Send metrics to InfluxDB using Telegraf or native formats, export logs to Elasticsearch via various output formats, and use native integrations for cloud platforms like AWS, GCP, and Azure.

## Deployment patterns

Choose the deployment pattern that fits your architecture.

Deploy {{< param "PRODUCT_NAME" >}} at the edge, close to your data sources, for minimal latency.
Run it as a DaemonSet in Kubernetes to collect from every node, install it on each host for infrastructure monitoring, or deploy it alongside applications for local processing.
Use this pattern to minimize network hops and enable immediate data transformation at the source.

Deploy {{< param "PRODUCT_NAME" >}} as a gateway for centralized collection.
Configure your applications to send telemetry to {{< param "PRODUCT_NAME" >}} gateways, which process and forward data to backends.
Simplify backend configuration in your applications since they only need to know about the gateway endpoints.

Combine edge and gateway deployments in a hybrid approach.
Deploy edge instances to handle initial collection and filtering close to sources, then forward to gateway instances for aggregation and final processing.
Use this pattern to reduce bandwidth usage and enable centralized policy enforcement while maintaining local processing capabilities.for very specific integrations

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

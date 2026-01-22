---
canonical: https://grafana.com/docs/alloy/latest/introduction/how-alloy-works/
description: Learn how Grafana Alloy works and where it fits in your observability architecture
menuTitle: How Alloy works
title: How Grafana Alloy works
weight: 300
---

# How {{% param "FULL_PRODUCT_NAME" %}} works

Understanding the architecture and design of {{< param "PRODUCT_NAME" >}} helps you use it effectively.
This page explains where it fits in your observability stack and what makes it powerful.

## Where {{< param "PRODUCT_NAME" >}} fits

A typical observability setup has three layers: data sources that generate telemetry, collection tools that gather and process it, and storage backends with visualization frontends for querying and exploring data.

{{< param "PRODUCT_NAME" >}} operates in the collection layer, sitting between your data sources and your storage backends.
It acts as the bridge between them, performing three main functions in your telemetry pipeline.

### Collect telemetry data

{{< param "PRODUCT_NAME" >}} gathers telemetry from any source in your infrastructure.
You can configure it to scrape Prometheus endpoints for metrics or set up receivers to accept data pushed via the OpenTelemetry protocol.
It tails log files and reads from system outputs to capture application and infrastructure logs.
Service discovery automatically finds resources in Kubernetes, Docker, or cloud environments without requiring static configuration.
You can also integrate with databases, message queues, and other systems to capture telemetry from specialized sources.

### Transform and process data

Processing telemetry before sending it to backends optimizes costs and improves data quality.
Create filters to drop unwanted data or redact sensitive information like tokens and credentials from logs before they reach storage.
Add labels, metadata, or contextual information to enrich your data—for example, extract a cloud provider name from instance IDs to create useful aggregation labels.
Standardize attribute names across services when different teams use inconsistent naming conventions.
Implement sampling strategies to reduce high-volume data while preserving the signal you need for troubleshooting.
Convert between formats, such as transforming Prometheus metrics to OpenTelemetry format, to ensure compatibility with your backends.
Define routing rules to send different types of data to different destinations based on your operational requirements.

### Send to backends

{{< param "PRODUCT_NAME" >}} delivers processed telemetry to any storage system you choose.
Send data to Grafana Cloud for managed observability, or export to your self-managed Grafana stack components.
Connect to any Prometheus-compatible database for metrics and any OpenTelemetry-compatible backend for all signal types.
Write to multiple destinations simultaneously, sending the same data to different systems or routing different data types to specialized backends.

## Component-based architecture

{{< param "PRODUCT_NAME" >}} uses modular [components][] that work like building blocks.
Each component performs a specific task, such as collecting metrics from Prometheus endpoints, receiving OpenTelemetry data, transforming and filtering telemetry, or sending data to backends.

You connect these components together to [build pipelines][] that match your exact requirements.
This modular approach makes configurations easier to understand, test, and maintain.

## Programmable pipelines

{{< param "PRODUCT_NAME" >}} uses a rich, [expression-based configuration language][syntax] that lets you reference data from one component in another, create dynamic configurations that respond to changing conditions, build reusable pipelines you can share across teams, and use built-in [functions][expressions] to transform and filter data.

## Custom and shareable pipelines

You can create [custom components][] that combine multiple components into a single, reusable unit.
Share these custom components with your team or the community through the [module system][modules].
Use pre-built modules from the community or create your own.

## Enterprise-ready features

As your systems grow more complex, {{< param "PRODUCT_NAME" >}} scales with you.
[Clustering][] lets you configure instances to form a cluster for automatic workload distribution and high availability.
Centralized configuration retrieves settings from remote servers for fleet management.
Kubernetes-native capabilities let you interact with Kubernetes resources directly without learning separate operators.

## Built-in debugging tools

{{< param "PRODUCT_NAME" >}} includes a [built-in user interface][debug] that helps you visualize your component pipelines, inspect component states and outputs, troubleshoot configuration issues, and monitor performance.

## Deployment patterns

Choose the [deployment pattern][deploy] that fits your architecture.

**Edge deployment:** Deploy {{< param "PRODUCT_NAME" >}} close to your data sources for minimal latency.
Run it as a DaemonSet in Kubernetes to collect from every node, install it on each host for infrastructure monitoring, or deploy it alongside applications for local processing.

**Gateway deployment:** Deploy {{< param "PRODUCT_NAME" >}} as a centralized gateway.
Configure your applications to send telemetry to {{< param "PRODUCT_NAME" >}} gateways, which process and forward data to backends.
Applications only need to know about the gateway endpoints.

**Hybrid deployment:** Combine edge and gateway approaches.
Deploy edge instances to handle initial collection and filtering close to sources, then forward to gateway instances for aggregation and final processing.
This pattern reduces bandwidth usage and enables centralized policy enforcement while maintaining local processing capabilities.

## Integrations

{{< param "PRODUCT_NAME" >}} integrates with Grafana Cloud and self-managed Grafana stacks, routing metrics to Mimir, logs to Loki, traces to Tempo, and profiles to Pyroscope.
It also works with the broader Prometheus ecosystem through full compatibility with the Prometheus exposition format and service discovery mechanisms, and with any OpenTelemetry-compatible backend through OTLP support.

You can also connect to other ecosystems, including InfluxDB, Elasticsearch, and cloud platforms like AWS, Google Cloud Platform, and Azure.

## What {{< param "PRODUCT_NAME" >}} replaces

{{< param "PRODUCT_NAME" >}} can consolidate multiple collectors.
Replace Prometheus Agent to gain the same functionality plus support for logs, traces, and profiles.
Replace the OpenTelemetry Collector to add native Prometheus support alongside OTLP.
Migrate from Grafana Agent for enhanced capabilities and a more powerful configuration model.
Replace specialized log collectors like Promtail, `Fluentd`, or `Filebeat` with a unified collection approach.

You can also run {{< param "PRODUCT_NAME" >}} alongside collectors during migration to transition gradually without disrupting your observability.
Refer to the [migration guides][migrate] for step-by-step instructions.

## Next steps

- [Install][Install] {{< param "PRODUCT_NAME" >}} to get started
- Learn core [concepts][Concepts] including components, expressions, and pipelines
- Follow [tutorials][tutorials] for hands-on experience
- Explore the [component reference][reference] to see available components

[Install]: ../../set-up/install/
[Concepts]: ../../get-started/
[tutorials]: ../../tutorials/
[reference]: ../../reference/
[components]: ../../get-started/components/
[build pipelines]: ../../get-started/components/build-pipelines/
[syntax]: ../../get-started/syntax/
[expressions]: ../../get-started/expressions/
[custom components]: ../../get-started/components/custom-components/
[modules]: ../../get-started/modules/
[Clustering]: ../../get-started/clustering/
[debug]: ../../troubleshoot/debug/
[deploy]: ../../set-up/deploy/
[migrate]: ../../set-up/migrate/
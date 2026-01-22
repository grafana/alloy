---
canonical: https://grafana.com/docs/alloy/latest/introduction/why-alloy/
description: Understand when Grafana Alloy is the right choice for your telemetry collection needs
menuTitle: Why Alloy
title: Why Grafana Alloy
weight: 200
---

# Why {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} simplifies telemetry collection by consolidating multiple collectors into one solution.

## The telemetry collection challenge

Telemetry collection in production environments can quickly become complex as different teams develop different needs over time.

Consider a common scenario.
You start with infrastructure observability, using Prometheus Node Exporter to collect and export metrics.
Your metrics flow to a Prometheus database, and you visualize them in Grafana dashboards.
This works well for monitoring infrastructure.

Later, you want to add application observability and start analyzing distributed traces.
Prometheus doesn't support traces, so you add the OpenTelemetry Collector.
Now you're running two different collectors, each with its own configuration syntax, deployment requirements, and operational overhead.

As your observability needs grow, you might add a separate collector for logs, another tool for continuous profiling, and different agents for different environments.
Before long, you're managing multiple collectors, learning different configuration languages, troubleshooting various failure modes, and dealing with increased memory and CPU overhead.

{{< param "PRODUCT_NAME" >}} addresses these challenges by handling all signal types in a single deployment with one configuration language.
You learn one tool, deploy one collector, and maintain one system.

## When to use {{% param "PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} excels in several scenarios.
The following sections help you identify whether it fits your needs.

### You need multiple signal types

{{< param "PRODUCT_NAME" >}} natively supports metrics from both Prometheus and OpenTelemetry sources, application and system logs, distributed traces using OpenTelemetry, and continuous profiling data.

If you're running separate collectors for metrics and traces, or planning to add log collection to your metrics pipeline, {{< param "PRODUCT_NAME" >}} lets you consolidate into a single solution.

For example, if you monitor infrastructure with Prometheus but want to add distributed tracing for your microservices, you can use {{< param "PRODUCT_NAME" >}} to handle both with one collector instead of deploying multiple collectors.

### You want to reduce collector complexity

Running multiple collectors creates operational overhead.
You have to learn different configuration languages, manage separate deployments and upgrades, troubleshoot different failure modes, and monitor multiple systems—all while consuming more resources.

{{< param "PRODUCT_NAME" >}} consolidates all of this.
You learn one configuration language, manage one deployment, and use a single built-in UI for debugging.
This unified approach reduces both complexity and resource consumption.

For example, if your team runs Prometheus for metrics, `Fluentd` for logs, and Jaeger agents for traces, {{< param "PRODUCT_NAME" >}} can replace all three and simplify your telemetry architecture.

### You need both Prometheus and OpenTelemetry

{{< param "PRODUCT_NAME" >}} works with both ecosystems simultaneously.
It includes native Prometheus remote write support, full OpenTelemetry protocol support, Prometheus service discovery mechanisms, OpenTelemetry instrumentation compatibility, and the ability to convert between formats.

You don't have to choose between Prometheus and OpenTelemetry.
If you have Prometheus deployments and instrumentation but your applications use OpenTelemetry, {{< param "PRODUCT_NAME" >}} collects from both while you standardize on one collector.

### You value vendor neutrality

{{< param "PRODUCT_NAME" >}} supports sending data to Grafana Cloud, a self-managed Grafana stack with Loki, Mimir, Tempo, and Pyroscope, any Prometheus-compatible database, any OpenTelemetry-compatible backend, or multiple destinations simultaneously.

This flexibility means you're not locked into a single vendor or backend.
You can send data to Grafana Cloud for some telemetry and self-managed systems for others, or change backends without changing your collector.

For example, if you want to send metrics to Grafana Cloud but keep logs on-premises for compliance reasons, {{< param "PRODUCT_NAME" >}} can send metrics to the cloud and logs to your local Loki instance from the same configuration.

### Your observability needs are growing

{{< param "PRODUCT_NAME" >}} provides features for scaling, including clustering to distribute workload across multiple instances for high availability and horizontal scaling, remote configuration to manage fleet-wide configurations from a central location, and automatic workload distribution across cluster members.

Start with a single {{< param "PRODUCT_NAME" >}} instance and scale to clusters as your needs grow, without changing your approach.

### You're running on Kubernetes

{{< param "PRODUCT_NAME" >}} offers Kubernetes-native features including first-class support for discovering Kubernetes resources, components that interact with Kubernetes APIs directly, native understanding of pods, services, and custom resources, and support for DaemonSet and Deployment patterns.

No separate Kubernetes operator is required.
The Kubernetes discovery components automatically find and scrape pods as they start and stop, without additional configuration.

### You want programmable pipelines

The {{< param "PRODUCT_NAME" >}} configuration language lets you create conditional logic in your pipelines, reference data from one component in another, build reusable pipeline modules, transform and filter data with built-in functions, and respond dynamically to changing conditions.

If you need more than basic "collect and forward" functionality, the programmable approach provides the flexibility you need.
Common scenarios include routing high-priority metrics to one backend while sampling lower-priority data, extracting useful labels from high-cardinality fields to manage storage costs, standardizing attribute names when different teams use inconsistent conventions, and redacting sensitive tokens or credentials from logs before they reach storage.

### You want to share pipelines across teams

The module system allows you to create custom components that combine multiple steps, package and share pipelines with your team, use community-contributed modules, and maintain consistent collection patterns across services.

Your platform team can create a standard monitoring module that application teams import and configure with their specific settings, without understanding the underlying complexity.

## When {{% param "PRODUCT_NAME" %}} might not be the right choice

{{< param "PRODUCT_NAME" >}} is powerful and flexible, but it's not always the best fit.

Consider alternatives if you only need basic Prometheus metrics scraping with no additional features, as Prometheus Agent might be simpler.
Similarly, if you're deeply integrated with a specific collector's ecosystem and don't need multi-signal support, or if you have very specific requirements that available components don't address, evaluate whether the benefits outweigh staying with your current solution.

## Next steps

- [Install][Install] {{< param "PRODUCT_NAME" >}} to get started
- Review [supported platforms][supported platforms] to confirm compatibility
- Learn about the [architecture and components][Concepts]
- Follow a [tutorial][tutorial] for hands-on experience

[Install]: ../../set-up/install/
[supported platforms]: ../../set-up/supported-platforms/
[Concepts]: ../../get-started/
[tutorial]: ../../tutorials/

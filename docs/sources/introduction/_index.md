---
canonical: https://grafana.com/docs/alloy/latest/introduction/
description: Grafana Alloy simplifies telemetry collection by combining metrics, logs, traces, and profiles into one powerful, vendor-neutral collector
menuTitle: Introduction
title: Introduction to Grafana Alloy
weight: 10
---

# Introduction to {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} is an open source telemetry collector that simplifies how you gather and send observability data.
It combines the power of multiple collectors into one solution, supporting metrics, logs, traces, and profiles in a single, unified platform.

{{< param "PRODUCT_NAME" >}} is an [OpenTelemetry Collector distribution][OpenTelemetry] with built-in Prometheus pipelines and native support for Loki, Pyroscope, and other observability backends.
It's compatible with popular open source standards including OpenTelemetry and Prometheus.

{{< youtube bFyGd_Sr5W4 >}}

{{< docs/learning-journeys title="Send logs to Grafana Cloud using Alloy" url="/docs/learning-journeys/send-logs-alloy-loki/" >}}

## Where {{< param "PRODUCT_NAME" >}} fits in your observability stack

Understanding where {{< param "PRODUCT_NAME" >}} fits helps clarify what it does and when to use it.

Your observability architecture typically has three layers:

1. **Data sources**: Infrastructure, applications, and external services
1. **Collection and processing**: Collectors or agents that gather telemetry data
1. **Storage and visualization**: Databases and frontends for querying and exploring data

{{< param "PRODUCT_NAME" >}} operates in the collection and processing layer.
It sits between your data sources and your observability backends, collecting telemetry data and sending it to the databases of your choice.

{{< figure src="/media/docs/alloy/flow-diagram-small-alloy.png" alt="Alloy in the observability stack" >}}

## When to use {{< param "PRODUCT_NAME" >}}

{{< param "PRODUCT_NAME" >}} is a strong choice when you need:

- **Multiple signal types**: Metrics, logs, traces, and profiles from a single collector
- **Simplified operations**: Fewer tools to learn, configure, and maintain
- **Vendor neutrality**: Flexibility to send data to multiple backends and ecosystems
- **Prometheus and OpenTelemetry together**: Native support for both without running separate collectors
- **Scalability**: Growing observability needs that require enterprise-ready features
- **Kubernetes-native collection**: First-class support for Kubernetes resources without additional operators

If you're starting fresh or consolidating existing collectors, {{< param "PRODUCT_NAME" >}} provides a unified approach to telemetry collection.

## Get started with {{< param "PRODUCT_NAME" >}}

Ready to try {{< param "PRODUCT_NAME" >}}? Start with these resources:

- [Install][Install] {{< param "PRODUCT_NAME" >}} on your platform
- Learn core [Concepts][Concepts] including components, expressions, and pipelines
- Follow [tutorials][tutorials] for hands-on experience with common use cases
- Explore the [component reference][reference] to see what {{< param "PRODUCT_NAME" >}} can do

## Learn more

- [The challenge][The challenge] - Understand the telemetry collection problems {{< param "PRODUCT_NAME" >}} solves
- [When to use Alloy][When to use Alloy] - Determine which scenarios {{< param "PRODUCT_NAME" >}} is designed for
- [Alloy in the observability stack][Alloy in the observability stack] - See how {{< param "PRODUCT_NAME" >}} integrates with other tools
- [How Alloy works][How Alloy works] - Learn what makes {{< param "PRODUCT_NAME" >}} powerful
- [Supported platforms][Supported platforms] - Check platform and architecture compatibility
- [Estimate resource usage][Estimate resource usage] - Plan your deployment resource requirements
- [Migrate from other collectors][migrate] - Move from OpenTelemetry Collector, Prometheus Agent, or Grafana Agent

[OpenTelemetry]: https://opentelemetry.io/ecosystem/distributions/
[Install]: ../set-up/install/
[Concepts]: ../get-started/
[tutorials]: ../tutorials/
[reference]: ../reference/
[The challenge]: ./the-challenge/
[When to use Alloy]: ./when-to-use-alloy/
[Alloy in the observability stack]: ./alloy-in-observability-stack/
[How Alloy works]: ./how-alloy-works/
[Supported platforms]: ./supported-platforms/
[Estimate resource usage]: ./estimate-resource-usage/
[migrate]: ../set-up/migrate/

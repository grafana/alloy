---
canonical: https://grafana.com/docs/alloy/latest/introduction/overview/
description: Overview of Grafana Alloy and where it fits in your observability architecture
menuTitle: Overview
title: Grafana Alloy overview
weight: 150
---

# {{% param "FULL_PRODUCT_NAME" %}} overview

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

## Next steps

- Learn about [the challenge][The challenge] that {{< param "PRODUCT_NAME" >}} solves
- Review detailed [scenarios for when to use Alloy][When to use Alloy]
- Understand [how Alloy works][How Alloy works]
- [Install][Install] {{< param "PRODUCT_NAME" >}} to get started

[The challenge]: ../the-challenge/
[When to use Alloy]: ../when-to-use-alloy/
[How Alloy works]: ../how-alloy-works/
[Install]: ../../set-up/install/

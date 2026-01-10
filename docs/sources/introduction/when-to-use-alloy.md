---
canonical: https://grafana.com/docs/alloy/latest/introduction/when-to-use-alloy/
description: Learn when Grafana Alloy is the right choice for your telemetry collection needs
menuTitle: When to use Alloy
title: When to use Grafana Alloy
weight: 250
---

# When to use {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} excels at simplifying telemetry collection, especially when you need to gather multiple signal types or consolidate existing collectors.
This page helps you determine when {{< param "PRODUCT_NAME" >}} is the right choice for your observability needs.

## You need multiple signal types

Use {{< param "PRODUCT_NAME" >}} when you want to collect more than one type of telemetry data.

{{< param "PRODUCT_NAME" >}} natively supports:

- **Metrics**: Prometheus metrics and OpenTelemetry metrics
- **Logs**: Application logs, system logs, and structured logs
- **Traces**: Distributed traces using OpenTelemetry
- **Profiles**: Continuous profiling data

If you're currently running separate collectors for metrics and traces, or planning to add log collection to your metrics pipeline, {{< param "PRODUCT_NAME" >}} lets you consolidate these into a single solution.

### Tracing with metrics collection

You monitor infrastructure with Prometheus but want to add distributed tracing for your microservices.
Instead of deploying the OpenTelemetry Collector alongside your existing setup, use {{< param "PRODUCT_NAME" >}} to handle both metrics and traces with one collector.

## You want to reduce collector complexity

Use {{< param "PRODUCT_NAME" >}} when managing multiple collectors creates operational overhead.

Running multiple collectors means:

- Learning different configuration languages
- Managing separate deployments and upgrades
- Troubleshooting different failure modes
- Monitoring multiple systems
- Increased resource consumption

{{< param "PRODUCT_NAME" >}} consolidates these into:

- One configuration language
- One deployment to manage
- Unified troubleshooting approach
- Single built-in UI for debugging
- Lower memory and CPU usage

### Multiple collector consolidation

Your team runs Prometheus for metrics, Fluentd for logs, and Jaeger agents for traces.
Each tool has its own configuration format and operational requirements.
{{< param "PRODUCT_NAME" >}} can replace all three, simplifying your telemetry architecture.

## You need both Prometheus and OpenTelemetry

Use {{< param "PRODUCT_NAME" >}} when you want to leverage both the Prometheus and OpenTelemetry ecosystems.

{{< param "PRODUCT_NAME" >}} includes:

- Native Prometheus remote write support
- Full OpenTelemetry protocol support
- Prometheus service discovery mechanisms
- OpenTelemetry instrumentation compatibility
- Ability to convert between formats

You don't need to choose between Prometheus and OpenTelemetry.
{{< param "PRODUCT_NAME" >}} works with both simultaneously.

### Prometheus and OpenTelemetry together

You have existing Prometheus deployments and instrumentation, but new applications use OpenTelemetry.
{{< param "PRODUCT_NAME" >}} collects from both, letting you standardize on one collector while supporting both ecosystems.

## You value vendor neutrality

Use {{< param "PRODUCT_NAME" >}} when you want flexibility in where you send your data.

{{< param "PRODUCT_NAME" >}} supports sending data to:

- Grafana Cloud
- Self-hosted Grafana stack with Loki, Mimir, Tempo, and Pyroscope
- Any Prometheus-compatible database
- Any OpenTelemetry-compatible backend
- Multiple destinations simultaneously

This "big tent" approach means you're not locked into a single vendor or backend.
You can send data to Grafana Cloud for some telemetry and self-hosted systems for others, or change backends without changing your collector.

### Hybrid cloud and on-premises deployment

You send metrics to Grafana Cloud but want to keep logs on-premises for compliance reasons.
{{< param "PRODUCT_NAME" >}} can send metrics to the cloud and logs to your local Loki instance from the same configuration.

## Your observability needs are growing

Use {{< param "PRODUCT_NAME" >}} when you expect your observability requirements to become more complex.

{{< param "PRODUCT_NAME" >}} provides enterprise-ready features:

- **Clustering**: Distribute workload across multiple {{< param "PRODUCT_NAME" >}} instances for high availability and horizontal scaling
- **Remote configuration**: Manage fleet-wide configurations from a central location
- **Workload distribution**: Automatically balance collection tasks across cluster members

Start simple with a single {{< param "PRODUCT_NAME" >}} instance and scale to clusters as your needs grow, without changing your approach.

### Single instance to cluster

You start with one {{< param "PRODUCT_NAME" >}} instance monitoring a few applications.
As your system grows, you add more {{< param "PRODUCT_NAME" >}} instances and configure them as a cluster to distribute the workload automatically.

## You're running on Kubernetes

Use {{< param "PRODUCT_NAME" >}} when collecting telemetry from Kubernetes environments.

{{< param "PRODUCT_NAME" >}} offers Kubernetes-native features:

- First-class support for discovering Kubernetes resources
- Components that interact with Kubernetes APIs directly
- No separate Kubernetes operator required
- Native understanding of pods, services, and custom resources
- DaemonSet and Deployment patterns for different collection strategies

### Pod discovery in Kubernetes

You need to collect metrics from all pods in your Kubernetes cluster.
The Kubernetes discovery components automatically find and scrape pods as they start and stop, without additional configuration.

## You want programmable pipelines

Use {{< param "PRODUCT_NAME" >}} when you need flexible, dynamic telemetry pipelines.

The {{< param "PRODUCT_NAME" >}} configuration language lets you:

- Create conditional logic in your pipelines
- Reference data from one component in another
- Build reusable pipeline modules
- Transform and filter data with built-in functions
- Respond dynamically to changing conditions

If you need more than basic "collect and forward" functionality, the programmable approach in {{< param "PRODUCT_NAME" >}} provides the flexibility you need.

### Metric routing by priority

You want to send high-priority service metrics to one backend and lower-priority metrics to another, with different sampling rates.
The {{< param "PRODUCT_NAME" >}} pipeline logic lets you route and transform data based on labels, values, or other conditions.

## You want to share pipelines across teams

Use {{< param "PRODUCT_NAME" >}} when you want to standardize telemetry collection across your organization.

The module system in {{< param "PRODUCT_NAME" >}} allows you to:

- Create custom components that combine multiple steps
- Package and share pipelines with your team
- Use community-contributed modules
- Maintain consistent collection patterns across services

Build reusable components once and share them across teams, reducing duplication and ensuring consistency.

### Module sharing across teams

Your platform team creates a standard monitoring module that all application teams can use.
Application teams import the module and configure it with their specific settings, without understanding the underlying complexity.

## When {{< param "PRODUCT_NAME" >}} might not be the right choice

{{< param "PRODUCT_NAME" >}} is powerful and flexible, but it's not always the best fit.

Consider alternatives when:

- You only need basic Prometheus metrics scraping with no additional features. Prometheus Agent might be simpler.
- You're deeply integrated with a specific collector's ecosystem and don't need multi-signal support
- You have very specific requirements that existing components don't address and you can't build using the primitives in {{< param "PRODUCT_NAME" >}}
- You need features that are unique to other collectors and not available in {{< param "PRODUCT_NAME" >}}

In these cases, evaluate whether the benefits of {{< param "PRODUCT_NAME" >}} outweigh staying with your current solution.
Consider unified collection, reduced complexity, and programmability.

## Next steps

- [Install][] {{< param "PRODUCT_NAME" >}} to get started
- Review [supported platforms][] to confirm compatibility
- Learn about the [architecture and components][Concepts]
- Follow a [tutorial][] for hands-on experience

[Install]: ../../set-up/install/
[supported platforms]: ../supported-platforms/
[Concepts]: ../../get-started/
[tutorial]: ../../tutorials/

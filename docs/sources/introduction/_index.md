---
canonical: https://grafana.com/docs/alloy/latest/introduction/
description: Grafana Alloy is a flexible, high performance, vendor-neutral distribution of the OTel Collector
menuTitle: Introduction
title: Introduction to Grafana Alloy
weight: 10
---

# Introduction to {{% param "FULL_PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} is a flexible, high performance, vendor-neutral distribution of the [OpenTelemetry][] Collector.
It combines the strengths of leading observability collectors into one unified solution.
Whether observing applications, infrastructure, or both, {{< param "PRODUCT_NAME" >}} can collect, process, and export telemetry signals to scale and strengthen your observability approach.

{{< param "PRODUCT_NAME" >}} is compatible with the most popular open source observability standards such as OpenTelemetry and Prometheus, while providing advanced features like programmable pipelines, automatic clustering, and enterprise-grade reliability that go beyond traditional collectors.

## Why choose {{% param "PRODUCT_NAME" %}}?

**Unify your telemetry collection strategy.** Many organizations struggle with managing multiple specialized collectors, each with different configurations, maintenance overhead, and operational complexity.
{{< param "PRODUCT_NAME" >}} eliminates this fragmentation by providing native support for all telemetry signals—metrics, logs, traces, and profiles—in a single, cohesive solution.

**Scale effortlessly with built-in clustering.** Unlike traditional collectors that require complex external orchestration for high availability, {{< param "PRODUCT_NAME" >}} provides automatic workload distribution and clustering out of the box.
Scale horizontally with minimal operational overhead while maintaining enterprise-grade reliability.

**Program powerful pipelines with confidence.** {{< param "PRODUCT_NAME" >}}'s rich expression-based syntax and component architecture make complex observability pipelines straightforward to build, understand, and maintain.
Create reusable components, share pipelines with your team, and adapt quickly to changing requirements.

{{< docs/learning-journeys title="Send logs to Grafana Cloud using Alloy" url="/docs/learning-journeys/send-logs-alloy-loki/" >}}

## Key capabilities

{{< param "PRODUCT_NAME" >}} provides enterprise-grade observability collection that scales with your organization:

### Unified telemetry collection

* **All signals, one platform:** {{< param "PRODUCT_NAME" >}} provides native support for metrics, logs, traces, and profiles without having to learn, configure, and manage multiple collectors.
  This unified approach simplifies your observability stack and reduces operational complexity.
* **Components covering popular technologies:** You can collect telemetry data from applications, databases, cloud services, and infrastructure using the built-in components.
  These components cover popular technologies and services across your entire technology stack.
* **Multi-ecosystem support:** {{< param "PRODUCT_NAME" >}} works with the OpenTelemetry, Prometheus, Grafana Loki, and Grafana Pyroscope ecosystems.
  This compatibility ensures you can integrate with your current observability tools and workflows.

### Enterprise scalability and reliability

* **Automatic clustering:** {{< param "PRODUCT_NAME" >}} provides built-in workload distribution and high availability without requiring external orchestration tools.
  The clustering feature automatically manages load balancing and fault tolerance to ensure continuous operation.
* **Dynamic scaling:** You can add or remove instances as load changes with automatic target redistribution across the cluster.
  This elasticity allows your observability infrastructure to adapt to changing demands without manual intervention.
* **Consistent hashing:** {{< param "PRODUCT_NAME" >}} uses efficient load distribution algorithms that minimize disruption when cluster topology changes.
  This approach ensures stable performance during scaling operations and node replacements.

### Developer productivity

* **Programmable pipelines:** {{< param "PRODUCT_NAME" >}} offers a rich expression-based syntax for creating sophisticated data processing workflows.
  This programming model makes complex observability pipelines straightforward to build, understand, and maintain.
* **Reusable components:** You can build components once and use them everywhere across your organization.
  Create custom components and share them across teams to promote consistency and reduce development time.
* **GitOps compatibility:** {{< param "PRODUCT_NAME" >}} can pull configurations from Git repositories, S3 buckets, HTTP endpoints, and other sources for automated deployments.
  This integration enables version-controlled, infrastructure-as-code approaches to observability configuration.

### Operational excellence

* **Built-in debugging:** {{< param "PRODUCT_NAME" >}} includes an embedded UI and troubleshooting tools that help you identify and resolve configuration issues quickly.
  These diagnostic features reduce time-to-resolution for observability pipeline problems.
* **Security-first:** {{< param "PRODUCT_NAME" >}} provides integrated credential management with HashiCorp Vault and Kubernetes secret support.
  This security integration ensures {{< param "PRODUCT_NAME" >}} handles sensitive information safely throughout your observability infrastructure.
* **Vendor neutrality:** {{< param "PRODUCT_NAME" >}} follows a "big tent" approach that ensures compatibility with any observability backend or open source database.
  This flexibility protects your investment and prevents vendor lock-in.

## Common use cases

You can use {{< param "PRODUCT_NAME" >}} in many different ways, allowing you to modernize your monitoring, scale as your company grows, bridge your ecosystems, and even implement cloud-native observability.

### Modernize legacy monitoring

Replace multiple specialized agents and collectors with a unified solution that handles all telemetry types.
Reduce operational complexity while gaining modern features like clustering and programmable pipelines.

### Scale observability infrastructure

Start with a single instance and grow to enterprise-scale deployments without architectural changes.
{{< param "PRODUCT_NAME" >}} uses clustering to automatically distribute the workload as you add capacity.

### Bridge observability ecosystems

Connect OpenTelemetry applications with Prometheus infrastructure, or send Prometheus metrics to OpenTelemetry backends.
{{< param "PRODUCT_NAME" >}} natively supports both ecosystems without conversion overhead.

### Implement cloud-native observability

Deploy {{< param "PRODUCT_NAME" >}} on Kubernetes with native resource discovery, automatic configuration updates, and seamless integration with cloud providers.
{{< param "PRODUCT_NAME" >}} is built for containerized environments and provides dynamic service discovery that automatically adapts to changes in your infrastructure.
This cloud-native design eliminates manual configuration overhead and ensures comprehensive observability coverage as your services scale.

## How {{% param "PRODUCT_NAME" %}} compares

**vs. OpenTelemetry Collector:** {{< param "PRODUCT_NAME" >}} includes OpenTelemetry Collector capabilities while adding native Prometheus pipelines, advanced clustering, programmable configuration syntax, and enterprise features like centralized configuration management.
This combination provides the flexibility of OpenTelemetry with additional production-ready capabilities.

**vs. Prometheus Agent:** While Prometheus Agent focuses solely on metrics collection, it lacks support for logs, traces, and profiles.
{{< param "PRODUCT_NAME" >}} provides unified collection for all telemetry signals with the same operational simplicity, eliminating the need for multiple specialized collectors.

**vs. Traditional monitoring agents:** Legacy agents typically handle one signal type, require complex clustering solutions, and use static configuration files that are difficult to maintain and update.
{{< param "PRODUCT_NAME" >}} provides dynamic, programmable pipelines with built-in high availability and centralized configuration management.

**vs. Vendor-specific agents:** Proprietary agents lock you into specific backends and ecosystems, creating vendor dependency and limiting your flexibility.
{{< param "PRODUCT_NAME" >}}'s vendor-neutral approach ensures flexibility to change backends without reconfiguring collection infrastructure, protecting your investment and preventing vendor lock-in.

{{< admonition type="note" >}}
For a detailed comparison of {{< param "PRODUCT_NAME" >}} with other observability collectors and migration guidance, refer to [Why choose {{< param "PRODUCT_NAME" >}}](https://grafana.com/docs/alloy/latest/introduction/why-choose-alloy/).
{{< /admonition >}}

## How does {{% param "PRODUCT_NAME" %}} work as an OpenTelemetry collector?

{{< figure src="/media/docs/alloy/flow-diagram-small-alloy.png" alt="Alloy flow diagram" >}}

### Collect

{{< param "PRODUCT_NAME" >}} uses more than 120 components to collect telemetry data from applications, databases, and OpenTelemetry collectors.
These components provide comprehensive coverage across your technology stack, from application-level metrics to infrastructure monitoring.

{{< param "PRODUCT_NAME" >}} supports collection using multiple ecosystems, including OpenTelemetry and Prometheus.
This multi-ecosystem approach allows you to integrate with your current monitoring setup while providing a migration path to modern observability standards.

Telemetry data can be either pushed to {{< param "PRODUCT_NAME" >}} through various protocols and interfaces, or {{< param "PRODUCT_NAME" >}} can actively pull data from your sources using service discovery and scraping mechanisms.
This flexibility accommodates different application architectures and deployment patterns.

### Transform

{{< param "PRODUCT_NAME" >}} processes and transforms telemetry data before sending it to its destination.
The transformation capabilities allow you to modify, enrich, and filter data to meet your specific observability requirements.

You can use transformations to inject extra metadata into telemetry signals, filter out unwanted or sensitive data, aggregate metrics, and normalize data formats.
These transformations ensure that only relevant, properly formatted data reaches your observability backends.

### Write

{{< param "PRODUCT_NAME" >}} sends processed telemetry data to OpenTelemetry-compatible databases or collectors, the Grafana stack, or Grafana Cloud.
The flexible output capabilities support multiple destinations simultaneously, allowing you to route different data types to appropriate storage systems.

{{< param "PRODUCT_NAME" >}} can also write alerting rules to compatible databases, enabling you to define monitoring conditions directly within your collection pipeline.
This integration streamlines the process of setting up comprehensive observability and alerting.

## Next steps

* [Install][] {{< param "PRODUCT_NAME" >}} on your preferred platform to get started with unified telemetry collection.
* Learn about the core [Concepts][] of {{< param "PRODUCT_NAME" >}} to understand its architecture and operational model.
* Follow the [tutorials][] for hands-on learning about {{< param "PRODUCT_NAME" >}} configuration and deployment scenarios.
* Learn how to [collect and forward data][Collect] with {{< param "PRODUCT_NAME" >}} to set up your observability pipelines.
* Check out the [reference][] documentation to find detailed information about {{< param "PRODUCT_NAME" >}} components, configuration blocks, and command line tools.

[OpenTelemetry]: https://opentelemetry.io/ecosystem/distributions/
[Install]: ../set-up/install/
[Concepts]: ../get-started/
[Collect]: ../collect/
[tutorials]: ../tutorials/
[reference]: ../reference/

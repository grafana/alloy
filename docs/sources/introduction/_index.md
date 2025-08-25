---
canonical: https://grafana.com/docs/alloy/latest/introduction/
description: Grafana Alloy is a flexible, high performance, vendor-neutral distribution of the OTel Collector
menuTitle: Introduction
title: Introduction to Grafana Alloy
weight: 10
---

# Introduction to {{% param "FULL_PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} is a flexible, high performance, vendor-neutral distribution of the [OpenTelemetry][] Collector that combines the strengths of leading observability collectors into one unified solution.
Whether observing applications, infrastructure, or both, {{< param "PRODUCT_NAME" >}} can collect, process, and export telemetry signals to scale and future-proof your observability approach.

{{< param "PRODUCT_NAME" >}} is fully compatible with the most popular open source observability standards such as OpenTelemetry and Prometheus, while providing advanced features like programmable pipelines, automatic clustering, and enterprise-grade reliability that go beyond traditional collectors.

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

* **All signals, one platform:** Native support for metrics, logs, traces, and profiles without requiring multiple specialized tools
* **120+ components:** Collect from applications, databases, cloud services, and infrastructure using battle-tested integrations
* **Multi-ecosystem support:** Works seamlessly with OpenTelemetry, Prometheus, Grafana Loki, and Grafana Pyroscope ecosystems

### Enterprise scalability and reliability

* **Automatic clustering:** Built-in workload distribution and high availability without external orchestration
* **Dynamic scaling:** Add or remove instances as load changes with automatic target redistribution
* **Consistent hashing:** Efficient load distribution that minimizes disruption when cluster topology changes

### Developer productivity

* **Programmable pipelines:** Rich expression-based syntax for creating sophisticated data processing workflows
* **Reusable components:** Build once, use everywhere—create custom components and share them across teams
* **GitOps compatibility:** Pull configurations from Git, S3, HTTP endpoints, and other sources for automated deployments

### Operational excellence

* **Built-in debugging:** Embedded UI and troubleshooting tools help identify and resolve configuration issues quickly
* **Security-first:** Integrated credential management with HashiCorp Vault and Kubernetes secret support
* **Vendor neutrality:** "Big tent" approach ensures compatibility with any observability backend or open source database

## Common use cases

### Modernize legacy monitoring

Replace multiple specialized agents and collectors with a unified solution that handles all telemetry types. Reduce operational complexity while gaining modern features like clustering and programmable pipelines.

### Scale observability infrastructure

Start with a single instance and grow to enterprise-scale deployments without architectural changes. {{< param "PRODUCT_NAME" >}}'s clustering automatically distributes workload as you add capacity.

### Bridge observability ecosystems

Connect OpenTelemetry applications with Prometheus infrastructure, or send Prometheus metrics to OpenTelemetry backends. {{< param "PRODUCT_NAME" >}} natively supports both ecosystems without conversion overhead.

### Implement cloud-native observability

Deploy on Kubernetes with native resource discovery, automatic configuration updates, and seamless integration with cloud providers. Purpose-built for containerized environments.

## How {{% param "PRODUCT_NAME" %}} compares

**vs. OpenTelemetry Collector:** {{< param "PRODUCT_NAME" >}} includes all OpenTelemetry Collector capabilities plus native Prometheus pipelines, advanced clustering, programmable configuration syntax, and enterprise features like centralized configuration management.

**vs. Prometheus Agent:** While focused solely on metrics, Prometheus Agent lacks support for logs, traces, and profiles.
{{< param "PRODUCT_NAME" >}} provides unified collection for all telemetry signals with the same operational simplicity.

**vs. Traditional monitoring agents:** Legacy agents typically handle one signal type, require complex clustering solutions, and use static configuration files.
{{< param "PRODUCT_NAME" >}} provides dynamic, programmable pipelines with built-in high availability.

**vs. Vendor-specific agents:** Proprietary agents lock you into specific backends and ecosystems. {{< param "PRODUCT_NAME" >}}'s vendor-neutral approach ensures flexibility to change backends without reconfiguring collection infrastructure.

{{< admonition type="note" >}}
For a detailed comparison of {{< param "PRODUCT_NAME" >}} with other observability collectors and migration guidance, refer to [Why choose {{< param "PRODUCT_NAME" >}}](why-choose-alloy/).
{{< /admonition >}}

## How does {{% param "PRODUCT_NAME" %}} work as an OpenTelemetry collector?

{{< figure src="/media/docs/alloy/flow-diagram-small-alloy.png" alt="Alloy flow diagram" >}}

### Collect

{{< param "PRODUCT_NAME" >}} uses more than 120 components to collect telemetry data from applications, databases, and OpenTelemetry collectors.
{{< param "PRODUCT_NAME" >}} supports collection using multiple ecosystems, including OpenTelemetry and Prometheus.

Telemetry data can be either pushed to {{< param "PRODUCT_NAME" >}}, or {{< param "PRODUCT_NAME" >}} can pull it from your data sources.

### Transform

{{< param "PRODUCT_NAME" >}} processes data and transforms it for sending.

You can use transformations to inject extra metadata into telemetry or filter out unwanted data.

### Write

{{< param "PRODUCT_NAME" >}} sends data to OpenTelemetry-compatible databases or collectors, the Grafana stack, or Grafana Cloud.

{{< param "PRODUCT_NAME" >}} can also write alerting rules in compatible databases.

## Next steps

* [Install][] {{< param "PRODUCT_NAME" >}}.
* Learn about the core [Concepts][] of {{< param "PRODUCT_NAME" >}}.
* Follow the [tutorials][] for hands-on learning about {{< param "PRODUCT_NAME" >}}.
* Learn how to [collect and forward data][Collect] with {{< param "PRODUCT_NAME" >}}.
* Check out the [reference][] documentation to find information about the {{< param "PRODUCT_NAME" >}} components, configuration blocks, and command line tools.

[OpenTelemetry]: https://opentelemetry.io/ecosystem/distributions/
[Install]: ../set-up/install/
[Concepts]: ../get-started/
[Collect]: ../collect/
[tutorials]: ../tutorials/
[reference]: ../reference/

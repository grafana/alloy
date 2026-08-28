---
canonical: https://grafana.com/docs/alloy/latest/introduction/
description: Grafana Alloy simplifies telemetry collection by combining metrics, logs, traces, and profiles into one powerful, vendor-neutral collector
menuTitle: Introduction
title: Introduction to Grafana Alloy
weight: 10
---

# Introduction to {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} is an open source telemetry collector that simplifies how you gather and send observability data.
It's an [OpenTelemetry Collector distribution][OpenTelemetry] with built-in Prometheus pipelines and native support for Loki, Pyroscope, and other observability backends.

{{< param "PRODUCT_NAME" >}} collects metrics, logs, traces, and profiles in one unified solution.
Instead of running separate collectors for each signal type, you configure a single tool that handles all your telemetry needs.
This approach reduces operational complexity while giving you the flexibility to send data to any compatible backend, whether that's Grafana Cloud, a self-managed Grafana stack, or other observability platforms.

{{< youtube bFyGd_Sr5W4 >}}

{{< docs/learning-journeys title="Send logs to Grafana Cloud using Alloy" url="/docs/learning-journeys/send-logs-alloy-loki/" >}}

## Get started

- [Install][Install] {{< param "PRODUCT_NAME" >}} on your platform
- Learn core [concepts][Concepts] including components, expressions, and pipelines
- Follow [tutorials][tutorials] for hands-on experience
- Explore [alloy-scenarios][scenarios] for real-world configuration examples
- Try the [Alloy for Beginners][beginners] workshop for interactive, scenario-based learning
- Explore the [component reference][reference] to see available components

## Learn more

- [Why Alloy][Why Alloy]: Understand when {{< param "PRODUCT_NAME" >}} is the right choice
- [How Alloy works][How Alloy works]: Learn about the architecture and key capabilities
- [Requirements and expectations][Requirements]: Review deployment considerations and constraints
- [Supported platforms][Supported platforms]: Check platform compatibility
- [Estimate resource usage][Estimate resource usage]: Plan your deployment
- [Access and permissions][Access and permissions]: Hardening, identity, network exposure, and secrets
- [Migrate from other collectors][migrate]: Move from OpenTelemetry Collector, Prometheus Agent, or Grafana Agent

[OpenTelemetry]: https://opentelemetry.io/docs/collector/distributions/
[Install]: ../set-up/install/
[Concepts]: ../get-started/
[tutorials]: ../tutorials/
[reference]: ../reference/
[Why Alloy]: ./why-alloy/
[How Alloy works]: ./how-alloy-works/
[Requirements]: ./requirements/
[Supported platforms]: ../set-up/supported-platforms/
[Estimate resource usage]: ../set-up/estimate-resource-usage/
[Access and permissions]: ../access_permissions/
[migrate]: ../set-up/migrate/
[beginners]: https://github.com/grafana/Grafana-Alloy-for-Beginners
[scenarios]: https://github.com/grafana/alloy-scenarios

## Frequently asked questions

{{< qa-list >}}
{{< qa question="What is Grafana Alloy?" >}}
Grafana Alloy is an open source telemetry collector that simplifies how you gather and send telemetry.
Alloy is a distribution of the OpenTelemetry Collector, an open source collector that receives, processes, and sends telemetry.
Alloy also includes built-in support for Prometheus, a system for collecting metrics, and can send data to Loki (logs), Pyroscope (profiles), and other telemetry backends.
{{< /qa >}}
{{< qa question="Do I need a separate collector for metrics, logs, traces, and profiles?" >}}
Not with Alloy.
It's designed to collect metrics, logs, traces, and profiles all in one tool, cutting down on the number of collectors you have to run and maintain.
You can still route that data to Grafana Cloud, a self-managed stack, or other compatible backends.
{{< /qa >}}
{{< qa question="How does Grafana Alloy work?" >}}
You build pipelines out of components which are small building blocks that each do one job, like receiving data, transforming it, or sending it to a backend.
You connect components together in a configuration file.
One component's output feeds into another's input.
Alloy runs that configuration as a continuous pipeline, so data flows from source to backend without you managing separate tools for each step.
{{< /qa >}}
{{< /qa-list >}}

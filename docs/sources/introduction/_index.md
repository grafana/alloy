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
- [Supported platforms][Supported platforms]: Check platform compatibility
- [Estimate resource usage][Estimate resource usage]: Plan your deployment
- [Migrate from other collectors][migrate]: Move from OpenTelemetry Collector, Prometheus Agent, or Grafana Agent

[OpenTelemetry]: https://opentelemetry.io/ecosystem/distributions/
[Install]: ../set-up/install/
[Concepts]: ../get-started/
[tutorials]: ../tutorials/
[reference]: ../reference/
[Why Alloy]: ./why-alloy/
[How Alloy works]: ./how-alloy-works/
[Supported platforms]: ../set-up/supported-platforms/
[Estimate resource usage]: ../set-up/estimate-resource-usage/
[migrate]: ../set-up/migrate/
[beginners]: https://github.com/grafana/Grafana-Alloy-for-Beginners
[scenarios]: https://github.com/grafana/alloy-scenarios
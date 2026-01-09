---
canonical: https://grafana.com/docs/alloy/latest/introduction/the-challenge/
description: Understand the telemetry collection challenges that Grafana Alloy solves
menuTitle: The challenge
title: The telemetry collection challenge
weight: 200
---

# The telemetry collection challenge

Telemetry collection in production environments can quickly become complex.
Different teams have different needs, and those needs evolve over time.

Consider this common scenario:

You start with infrastructure observability, using Prometheus node exporter to collect and export metrics.
Your metrics flow to a Prometheus database, and you visualize them in Grafana dashboards.
This works well for monitoring your infrastructure.

Later, you want to add application observability and start analyzing distributed traces.
Prometheus doesn't support traces, so you need to add the OpenTelemetry Collector.
Now you're running two different collectors, each with its own configuration syntax, deployment requirements, and operational overhead.

As your observability needs grow, you might add:

- A separate collector for logs
- Another tool for continuous profiling
- Different agents for different environments

Before long, you're managing multiple collectors, learning different configuration languages, troubleshooting various failure modes, and dealing with increased memory and CPU overhead.
The more collectors you run, the more complex your setup becomes.

This example is simplified.
In reality, production environments often have even more collectors, data format conversions, and operational challenges.

## How {{< param "PRODUCT_NAME" >}} simplifies telemetry collection

{{< param "PRODUCT_NAME" >}} addresses these challenges by combining multiple collectors into one powerful solution.

Instead of running separate collectors for metrics, logs, traces, and profiles, {{< param "PRODUCT_NAME" >}} handles all signal types in a single deployment.
It includes native pipelines for:

- **Prometheus**: Metrics collection and remote write
- **OpenTelemetry**: Metrics, logs, and traces
- **Grafana Loki**: Log aggregation
- **Grafana Pyroscope**: Continuous profiling

With {{< param "PRODUCT_NAME" >}}, you have:

- **One solution to learn**: A single configuration language and component model
- **One solution to deploy**: Unified installation and deployment process
- **One solution to maintain**: Simplified troubleshooting and upgrades
- **Reduced resource usage**: Less memory consumption and CPU overhead compared to running multiple collectors

Whether you're monitoring infrastructure, applications, or both, {{< param "PRODUCT_NAME" >}} helps you manage telemetry collection without juggling multiple tools.

## Next steps

- Learn [when to use Alloy][When to use Alloy] for your observability needs
- Understand [how Alloy works][How Alloy works] and what makes it powerful
- [Install][Install] {{< param "PRODUCT_NAME" >}} to get started

[When to use Alloy]: ../when-to-use-alloy/
[How Alloy works]: ../how-alloy-works/
[Install]: ../../set-up/install/

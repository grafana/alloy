---
canonical: https://grafana.com/docs/alloy/latest/introduction/how-alloy-works/
description: Learn how Grafana Alloy works and what makes it a powerful telemetry collector
menuTitle: How Alloy works
title: How Grafana Alloy works
weight: 350
---

# How {{% param "FULL_PRODUCT_NAME" %}} works

The design of {{< param "PRODUCT_NAME" >}} makes it both simple to start with and powerful for complex use cases.

## Component-based architecture

{{< param "PRODUCT_NAME" >}} uses modular [components][] that work like building blocks.
Each component performs a specific task, such as:

- Collecting metrics from Prometheus endpoints
- Receiving OpenTelemetry data
- Transforming and filtering telemetry
- Sending data to backends

You connect these components together to [build pipelines][] for exactly the pipeline you need.
This modular approach makes configurations easier to understand, test, and maintain.

## Programmable pipelines

{{< param "PRODUCT_NAME" >}} uses a rich, [expression-based configuration language][syntax] that lets you:

- Reference data from one component in another
- Create dynamic configurations that respond to changing conditions
- Build reusable pipelines you can share across teams
- Use built-in [functions][expressions] to transform and filter data

## Big tent philosophy

{{< param "PRODUCT_NAME" >}} embraces Grafana's "big tent" philosophy.
You can use {{< param "PRODUCT_NAME" >}} with multiple vendors and open source databases.
It's designed to integrate seamlessly with various telemetry ecosystems, not lock you into a single approach.

## Custom and shareable pipelines

You can create [custom components][] that combine multiple existing components into a single, reusable unit.
Share these custom components with your team or the community through the [module system][modules] in {{< param "PRODUCT_NAME" >}}.
Use pre-built modules from the community or create your own.

## Enterprise-ready features

As your systems grow more complex, {{< param "PRODUCT_NAME" >}} scales with you:

- **[Clustering][]**: Configure {{< param "PRODUCT_NAME" >}} instances to form a cluster for automatic workload distribution and high availability
- **Centralized configuration**: Retrieve configuration from remote servers for fleet management
- **Kubernetes-native**: Interact with Kubernetes resources directly without learning separate operators

## Debugging utilities

{{< param "PRODUCT_NAME" >}} includes a built-in user interface that helps you:

- Visualize your component pipelines
- Inspect component states and outputs
- Troubleshoot configuration issues
- Monitor performance

## The collection process

{{< param "PRODUCT_NAME" >}} follows a three-stage process: collect, transform, and write.

### Collect

{{< param "PRODUCT_NAME" >}} uses more than 120 components to collect telemetry data from applications, databases, and other collectors.
It supports collection using multiple ecosystems, including OpenTelemetry and Prometheus.

Telemetry data can be pushed to {{< param "PRODUCT_NAME" >}} or pulled from your data sources.
Applications can send OpenTelemetry data to {{< param "PRODUCT_NAME" >}}, or {{< param "PRODUCT_NAME" >}} can scrape Prometheus metrics from your systems.

### Transform

{{< param "PRODUCT_NAME" >}} processes data and transforms it before sending.

Use transformations to:

- Add metadata to enrich telemetry
- Filter out unwanted data to reduce costs
- Sample high-volume telemetry
- Convert between formats
- Route data to different destinations

### Write

{{< param "PRODUCT_NAME" >}} sends data to OpenTelemetry-compatible databases or collectors, the Grafana stack, or Grafana Cloud.

{{< param "PRODUCT_NAME" >}} can also write alerting rules to compatible databases, letting you define alerts alongside your collection pipelines.

## Next steps

- [Install][Install] {{< param "PRODUCT_NAME" >}} to get started
- Learn core [Concepts][Concepts] including components, expressions, and pipelines
- Follow [tutorials][tutorials] for hands-on experience with common use cases
- Explore the [component reference][reference] to see what {{< param "PRODUCT_NAME" >}} can do

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

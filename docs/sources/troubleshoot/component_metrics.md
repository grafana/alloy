---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/component_metrics/
aliases:
  - ../tasks/monitor/component_metrics/ # /docs/alloy/latest/tasks/monitor/component_metrics/
description: Learn how to monitor component metrics
title: Monitor components
weight: 200
---

# Monitor components

{{< param "PRODUCT_NAME" >}} [components][] may optionally expose Prometheus metrics which can be used to investigate the behavior of that component.
These component-specific metrics are only generated when an instance of that component is running.

> Component-specific metrics are different than any metrics processed by the component.
> Component-specific metrics are used to expose the state of a component for observability, alerting, and debugging.

Component-specific metrics are exposed at the `/metrics` HTTP endpoint of the {{< param "PRODUCT_NAME" >}} HTTP server, which defaults to listening on `http://localhost:12345`.

> The documentation for the [`alloy run`][alloy run] command describes how to modify the address {{< param "PRODUCT_NAME" >}} listens on for HTTP traffic.

Component-specific metrics have a `component_id` label matching the component ID generating those metrics.
For example, component-specific metrics for a `prometheus.remote_write` component labeled `production` has a `component_id` label with the value `prometheus.remote_write.production`.

The [reference documentation][] for each component described the list of component-specific metrics that the component exposes.
Not all components expose metrics.

## Filter metrics by component label

You can use the `component` query parameter to filter the `/metrics` output.

This parameter allows you to retrieve only metrics emitted by specific components.
You can pass the `component` parameter multiple times.
For example, `http://localhost:12345/metrics?component=prometheus.scrape&component=otelcol.receiverz`.

The endpoint will only return metrics that contain a `component` label with one of the provided values.

If you don't provide a `component` filter, the default behavior is unchanged, and the endpoint returns all metrics.

[components]: ../../get-started/components/
[alloy run]: ../../reference/cli/run/
[reference documentation]: ../../reference/components/

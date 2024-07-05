---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/controller_metrics/
aliases:
  - ../tasks/monitor/controller_metrics/ # /docs/alloy/latest/tasks/monitor/controller_metrics/
description: Learn how to monitor controller metrics
title: Monitor the Grafana Alloy component controller
menuTitle: Monitor the controller
weight: 100
---

# Monitor the {{< param "FULL_PRODUCT_NAME" >}} component controller

The {{< param "PRODUCT_NAME" >}} [component controller][] exposes Prometheus metrics which you can use to investigate the controller state.

Metrics for the controller are exposed at the `/metrics` HTTP endpoint of the {{< param "PRODUCT_NAME" >}} HTTP server, which defaults to listening on `http://localhost:12345`.

> The documentation for the [`alloy run`][alloy run] command describes how to modify the address {{< param "PRODUCT_NAME" >}} listens on for HTTP traffic.

The controller exposes the following metrics:

* `alloy_component_controller_evaluating` (Gauge): Set to `1` whenever the  component controller is currently evaluating components.
  This value may be misrepresented depending on how fast evaluations complete or how often evaluations occur.
* `alloy_component_controller_running_components` (Gauge): The current number of running components by health.
   The health is represented in the `health_type` label.
* `alloy_component_evaluation_seconds` (Histogram): The time it takes to evaluate components after one of their dependencies is updated.
* `alloy_component_dependencies_wait_seconds` (Histogram): Time spent by components waiting to be evaluated after one of their dependencies is updated.
* `alloy_component_evaluation_queue_size` (Gauge): The current number of component evaluations waiting to be performed.

[component controller]: ../../get-started/component_controller/
[alloy run]: ../../reference/cli/run/

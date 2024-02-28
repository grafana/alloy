---
aliases:
- /docs/grafana-cloud/agent/flow/tasks/monitor/controller_metrics/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tasks/monitor/controller_metrics/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tasks/monitor/controller_metrics/
- /docs/grafana-cloud/send-data/agent/flow/tasks/monitor/controller_metrics/
- controller-metrics/ # /docs/agent/latest/flow/tasks/monitor/controller-metrics/
# Previous page aliases for backwards compatibility:
- /docs/grafana-cloud/agent/flow/monitoring/controller_metrics/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/monitoring/controller_metrics/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/monitoring/controller_metrics/
- /docs/grafana-cloud/send-data/agent/flow/monitoring/controller_metrics/
- ../../monitoring/controller-metrics/ # /docs/agent/latest/flow/monitoring/controller-metrics/
- ../../monitoring/controller_metrics/ # /docs/agent/latest/flow/monitoring/controller_metrics/
canonical: https://grafana.com/docs/agent/latest/flow/monitoring/controller_metrics/
description: Learn how to monitor controller metrics
title: Monitor controller
weight: 100
---

# How to monitor controller

The {{< param "PRODUCT_NAME" >}} [component controller][] exposes Prometheus metrics which you can use to investigate the controller state.

Metrics for the controller are exposed at the `/metrics` HTTP endpoint of the {{< param "PRODUCT_NAME" >}} HTTP server, which defaults to listening on `http://localhost:12345`.

> The documentation for the [`grafana-agent run`][grafana-agent run] command describes how to modify the address {{< param "PRODUCT_NAME" >}} listens on for HTTP traffic.

The controller exposes the following metrics:

* `agent_component_controller_evaluating` (Gauge): Set to `1` whenever the  component controller is currently evaluating components.
  This value may be misrepresented depending on how fast evaluations complete or how often evaluations occur.
* `agent_component_controller_running_components` (Gauge): The current number of running components by health.
   The health is represented in the `health_type` label.
* `agent_component_evaluation_seconds` (Histogram): The time it takes to evaluate components after one of their dependencies is updated.
* `agent_component_dependencies_wait_seconds` (Histogram): Time spent by components waiting to be evaluated after one of their dependencies is updated.
* `agent_component_evaluation_queue_size` (Gauge): The current number of component evaluations waiting to be performed.

{{% docs/reference %}}
[component controller]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/component_controller.md"
[component controller]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/component_controller.md"
[grafana-agent run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/cli/run.md"
[grafana-agent run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/cli/run.md"
{{% /docs/reference %}}

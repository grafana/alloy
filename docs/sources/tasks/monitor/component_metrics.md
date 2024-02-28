---
aliases:
- /docs/grafana-cloud/agent/flow/tasks/monitor/component_metrics/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tasks/monitor/component_metrics/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tasks/monitor/component_metrics/
- /docs/grafana-cloud/send-data/agent/flow/tasks/monitor/component_metrics/
- component-metrics/ # /docs/agent/latest/flow/tasks/monitor/component-metrics/
# Previous page aliases for backwards compatibility:
- /docs/grafana-cloud/agent/flow/monitoring/component_metrics/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/monitoring/component_metrics/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/monitoring/component_metrics/
- /docs/grafana-cloud/send-data/agent/flow/monitoring/component_metrics/
- ../../monitoring/component-metrics/ # /docs/agent/latest/flow/monitoring/component-metrics/
- ../../monitoring/component_metrics/ # /docs/agent/latest/flow/monitoring/component_metrics/
canonical: https://grafana.com/docs/agent/latest/flow/monitoring/component_metrics/
description: Learn how to monitor component metrics
title: Monitor components
weight: 200
---

# How to monitor components

{{< param "PRODUCT_NAME" >}} [components][] may optionally expose Prometheus metrics which can be used to investigate the behavior of that component.
These component-specific metrics are only generated when an instance of that component is running.

> Component-specific metrics are different than any metrics being processed by the component.
> Component-specific metrics are used to expose the state of a component for observability, alerting, and debugging.

Component-specific metrics are exposed at the `/metrics` HTTP endpoint of the {{< param "PRODUCT_NAME" >}} HTTP server, which defaults to listening on `http://localhost:12345`.

> The documentation for the [`grafana-agent run`][grafana-agent run] command describes how to > modify the address {{< param "PRODUCT_NAME" >}} listens on for HTTP traffic.

Component-specific metrics have a `component_id` label matching the component ID generating those metrics.
For example, component-specific metrics for a `prometheus.remote_write` component labeled `production` will have a `component_id` label with the value `prometheus.remote_write.production`.

The [reference documentation][] for each component described the list of component-specific metrics that the component exposes.
Not all components expose metrics.

{{% docs/reference %}}
[components]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/components.md"
[components]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/components.md"
[grafana-agent run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/cli/run.md"
[grafana-agent run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/cli/run.md"
[reference documentation]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components"
[reference documentation]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components"
{{% /docs/reference %}}
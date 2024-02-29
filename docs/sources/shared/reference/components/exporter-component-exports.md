---
aliases:
- /docs/agent/shared/flow/reference/components/exporter-component-exports/
- /docs/grafana-cloud/agent/shared/flow/reference/components/exporter-component-exports/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/exporter-component-exports/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/shared/flow/reference/components/exporter-component-exports/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/exporter-component-exports/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/exporter-component-exports/
description: Shared content, exporter component exports
headless: true
---

The following fields are exported and can be referenced by other components.

Name      | Type                | Description
----------|---------------------|----------------------------------------------------------
`targets` | `list(map(string))` | The targets that can be used to collect exporter metrics.

For example, the `targets` can either be passed to a `discovery.relabel` component to rewrite the targets' label sets or to a `prometheus.scrape` component that collects the exposed metrics.

The exported targets use the configured [in-memory traffic][] address specified by the [run command][].

[in-memory traffic]: {{< relref "../../../../flow/concepts/component_controller.md#in-memory-traffic" >}}
[run command]: {{< relref "../../../../flow/reference/cli/run.md" >}}

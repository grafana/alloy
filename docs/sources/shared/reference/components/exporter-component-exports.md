---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/exporter-component-exports/
description: Shared content, exporter component exports
headless: true
---

The following fields are exported and can be referenced by other components.

Name      | Type                | Description
----------|---------------------|----------------------------------------------------------
`targets` | `list(map(string))` | The targets that can be used to collect exporter metrics.

For example, the `targets` can either be passed to a `discovery.relabel` component to rewrite the targets' label sets or to a `prometheus.scrape` component that collects the exposed metrics.

The exported targets use the configured [in-memory traffic][] address specified by the [run command][].

[in-memory traffic]: ../../../../get-started/component_controller/#in-memory-traffic
[run command]: ../../../cli/run/

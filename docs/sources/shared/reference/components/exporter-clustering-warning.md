---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/exporter-clustering-warning/
description: Shared content, exporter clustering warning
headless: true
---

{{< admonition type="note" >}}
We do not recommend using this exporter with [clustering](../../../../get-started/clustering/) enabled.

The default `instance` label set by this exporter is the hostname of the machine running {{< param "PRODUCT_NAME" >}}.
{{< param "PRODUCT_NAME" >}} clustering uses consistent hashing to distribute targets across the instances.
This requires the discovered targets to be the same and have the same labels across all cluster instances.

If you do need to use this component in a cluster, use a dedicated `prometheus.scrape` component that's used to scrape
this exporter and doesn't have clustering enabled. Alternatively, use `discovery.relabel` to set the `instance` label to a
value that is the same across all cluster instances.
{{< /admonition >}}

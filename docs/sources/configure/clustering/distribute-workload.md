---
canonical: https://grafana.com/docs/alloy/latest/configure/clustering/distribute-workload/
aliases:
  - ../../../tasks/distribute-prometheus-scrape-load/ # /docs/alloy/latest/tasks/distribute-prometheus-scrape-load/
  - ./distribute-prometheus-scrape-load/ # /docs/alloy/latest/configure/clustering/distribute-prometheus-scrape-load/
description: Learn how to distribute workload across cluster nodes
menuTitle: Distribute workload
title: Distribute workload across cluster nodes
weight: 200
---

# Distribute workload across cluster nodes

After you [enable clustering in {{< param "PRODUCT_NAME" >}}][configure-alloy], you can configure individual components to distribute their workload across the cluster.

Clustering with target auto-distribution allows a fleet of {{< param "PRODUCT_NAME" >}} instances to dynamically distribute workload and provides high-availability and horizontal scalability.

## Before you begin

- [Configure clustering][configure-alloy] in your {{< param "PRODUCT_NAME" >}} installation.
- Ensure that all clustered {{< param "PRODUCT_NAME" >}} instances have the same configuration file.

## Enable clustering in components

To enable workload distribution, add a `clustering` block with `enabled = true` to each component that should participate.

{{< admonition type="note" >}}
Components don't automatically participate in clustering.
You must explicitly enable clustering in each component that should distribute workload.
{{< /admonition >}}

### Components that support clustering

The following components support the `clustering` block:

**Prometheus metrics collection:**

- [`prometheus.scrape`][prometheus.scrape]
- [`prometheus.operator.podmonitors`][prometheus.operator.podmonitors]
- [`prometheus.operator.servicemonitors`][prometheus.operator.servicemonitors]
- [`prometheus.operator.scrapeconfigs`][prometheus.operator.scrapeconfigs]
- [`prometheus.operator.probes`][prometheus.operator.probes]

**Pyroscope profiling:**

- [`pyroscope.scrape`][pyroscope.scrape]

**Loki log collection:**

- [`loki.source.kubernetes`][loki.source.kubernetes]
- [`loki.source.podlogs`][loki.source.podlogs]

## Example: Distribute Prometheus metrics scrape load

This example shows how to configure `prometheus.scrape` to distribute scrape targets across cluster nodes.

1. Add a `clustering` block to your `prometheus.scrape` component:

   ```alloy
   prometheus.scrape "default" {
     targets    = discovery.kubernetes.pods.targets
     forward_to = [prometheus.remote_write.default.receiver]

     clustering {
       enabled = true
     }
   }
   ```

1. Restart or reload {{< param "PRODUCT_NAME" >}} for it to use the new configuration.

1. Validate that auto-distribution works:

   1. Using the {{< param "PRODUCT_NAME" >}} [UI][] on each node, navigate to the details page for the `prometheus.scrape` component.

   1. Compare the **Debug Info** sections between two different nodes to ensure that they don't scrape the same sets of targets.

## Example: Distribute log collection

This example shows how to configure `loki.source.kubernetes` to distribute log collection across cluster nodes.

```alloy
loki.source.kubernetes "pods" {
  targets    = discovery.kubernetes.pods.targets
  forward_to = [loki.write.default.receiver]

  clustering {
    enabled = true
  }
}
```

## Example: Distribute profiling targets

This example shows how to configure `pyroscope.scrape` to distribute profiling targets across cluster nodes.

```alloy
pyroscope.scrape "default" {
  targets    = discovery.kubernetes.pods.targets
  forward_to = [pyroscope.write.default.receiver]

  clustering {
    enabled = true
  }
}
```

## How workload distribution works

When you enable clustering in a component:

1. All cluster nodes use a consistent hashing algorithm to determine target ownership.
1. Each node only processes the subset of targets it's responsible for.
1. When a node joins or leaves the cluster, targets are automatically redistributed.
1. Approximately 1/N of targets are redistributed when cluster membership changes. This minimizes disruption.

For more information about clustering concepts, refer to [Clustering][clustering].

[configure-alloy]: ../configure-alloy/
[clustering]: ../../../get-started/clustering/
[UI]: ../../../troubleshoot/debug/#component-detail-page
[prometheus.scrape]: ../../../reference/components/prometheus/prometheus.scrape/#clustering
[prometheus.operator.podmonitors]: ../../../reference/components/prometheus/prometheus.operator.podmonitors/#clustering
[prometheus.operator.servicemonitors]: ../../../reference/components/prometheus/prometheus.operator.servicemonitors/#clustering
[prometheus.operator.scrapeconfigs]: ../../../reference/components/prometheus/prometheus.operator.scrapeconfigs/#clustering
[prometheus.operator.probes]: ../../../reference/components/prometheus/prometheus.operator.probes/#clustering
[pyroscope.scrape]: ../../../reference/components/pyroscope/pyroscope.scrape/#clustering
[loki.source.kubernetes]: ../../../reference/components/loki/loki.source.kubernetes/#clustering
[loki.source.podlogs]: ../../../reference/components/loki/loki.source.podlogs/#clustering

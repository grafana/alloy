---
canonical: https://grafana.com/docs/alloy/latest/configure/clustering/
aliases:
  - ../tasks/configure-alloy-clustering/ # /docs/alloy/latest/tasks/configure-alloy-clustering/
description: Learn how to configure Grafana Alloy clustering
menuTitle: Clustering
title: Configure clustering
weight: 100
---

# Configure clustering

You can configure {{< param "PRODUCT_NAME" >}} to run with [clustering][] so that individual {{< param "PRODUCT_NAME" >}} instances can work together for workload distribution and high availability.

To configure clustering, complete these two steps:

1. **Enable clustering in {{< param "PRODUCT_NAME" >}}**: Configure {{< param "PRODUCT_NAME" >}} itself to join a cluster using Helm Chart settings or command-line flags.
1. **Enable clustering in components**: Add a `clustering` block to each component that should participate in workload distribution.

{{< admonition type="note" >}}
Cluster mode at the {{< param "PRODUCT_NAME" >}} level doesn't automatically distribute workload.
You must also enable clustering in each component that should participate in the cluster.
{{< /admonition >}}

## Enable clustering in components

Several components support workload distribution through clustering, including `prometheus.scrape`, `loki.source.kubernetes`, and `pyroscope.scrape`.
Refer to [Distribute workload across cluster nodes][distribute-workload] for the complete list.

To enable clustering in a component, add a `clustering` block with `enabled = true`:

```alloy
prometheus.scrape "example" {
  targets    = discovery.kubernetes.pods.targets
  forward_to = [prometheus.remote_write.default.receiver]

  clustering {
    enabled = true
  }
}
```

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}} for clustering][configure-alloy]: Enable clustering in your {{< param "PRODUCT_NAME" >}} installation.
- [Distribute workload across cluster nodes][distribute-workload]: Configure components to distribute workload.

[clustering]: ../../get-started/clustering/
[configure-alloy]: ./configure-alloy/
[distribute-workload]: ./distribute-workload/

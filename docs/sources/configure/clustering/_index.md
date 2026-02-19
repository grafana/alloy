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

You can configure {{< param "PRODUCT_NAME" >}} to run with [clustering][] so that individual instances can work together for workload distribution and high availability.

To configure clustering, complete these two steps:

1. **Enable clustering in {{< param "PRODUCT_NAME" >}}**: Configure {{< param "PRODUCT_NAME" >}} itself to join a cluster using Helm Chart settings or command-line flags.
1. **Enable clustering in components**: Add a `clustering` block to each component that should participate in workload distribution.

{{< admonition type="note" >}}
Cluster mode at the {{< param "PRODUCT_NAME" >}} level doesn't automatically distribute workload.
You must also enable clustering in each component that should participate in the cluster.
{{< /admonition >}}

## Components that support clustering

The following components support workload distribution through clustering:

| Component                                                                    | Description                                                         |
| ---------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| [`prometheus.scrape`][prometheus.scrape]                                     | Distributes Prometheus metrics scrape targets across cluster nodes. |
| [`prometheus.operator.podmonitors`][prometheus.operator.podmonitors]         | Distributes PodMonitor scrape targets across cluster nodes.         |
| [`prometheus.operator.servicemonitors`][prometheus.operator.servicemonitors] | Distributes ServiceMonitor scrape targets across cluster nodes.     |
| [`prometheus.operator.scrapeconfigs`][prometheus.operator.scrapeconfigs]     | Distributes ScrapeConfig scrape targets across cluster nodes.       |
| [`prometheus.operator.probes`][prometheus.operator.probes]                   | Distributes Probe scrape targets across cluster nodes.              |
| [`pyroscope.scrape`][pyroscope.scrape]                                       | Distributes Pyroscope profiling targets across cluster nodes.       |
| [`loki.source.kubernetes`][loki.source.kubernetes]                           | Distributes Kubernetes log collection across cluster nodes.         |
| [`loki.source.podlogs`][loki.source.podlogs]                                 | Distributes PodLogs log collection across cluster nodes.            |

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
[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/#clustering
[prometheus.operator.podmonitors]: ../../reference/components/prometheus/prometheus.operator.podmonitors/#clustering
[prometheus.operator.servicemonitors]: ../../reference/components/prometheus/prometheus.operator.servicemonitors/#clustering
[prometheus.operator.scrapeconfigs]: ../../reference/components/prometheus/prometheus.operator.scrapeconfigs/#clustering
[prometheus.operator.probes]: ../../reference/components/prometheus/prometheus.operator.probes/#clustering
[pyroscope.scrape]: ../../reference/components/pyroscope/pyroscope.scrape/#clustering
[loki.source.kubernetes]: ../../reference/components/loki/loki.source.kubernetes/#clustering
[loki.source.podlogs]: ../../reference/components/loki/loki.source.podlogs/#clustering

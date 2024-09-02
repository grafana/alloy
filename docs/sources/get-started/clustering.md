---
canonical: https://grafana.com/docs/alloy/latest/get-started/clustering/
aliases:
  - ../concepts/clustering/ # /docs/alloy/latest/concepts/clustering/
description: Learn about Grafana Alloy clustering concepts
menuTitle: Clustering
title: Clustering
weight: 500
---

# Clustering

Clustering enables a fleet of {{< param "PRODUCT_NAME" >}} deployments to work together for workload distribution and high availability.
It helps create horizontally scalable deployments with minimal resource and operational overhead.

To achieve this, {{< param "PRODUCT_NAME" >}} makes use of an eventually consistent model that assumes all participating
{{< param "PRODUCT_NAME" >}} deployments are interchangeable and converge on using the same configuration file.

The behavior of a standalone, non-clustered {{< param "PRODUCT_NAME" >}} is the same as if it were a single-node cluster.

You configure clustering by passing `cluster` command-line flags to the [run][] command.

## Use cases

### Target auto-distribution

Target auto-distribution is the most basic use case of clustering.
It allows scraping components running on all peers to distribute the scrape load between themselves.
Target auto-distribution requires that all {{< param "PRODUCT_NAME" >}} deployments in the same cluster can reach the same service discovery APIs and scrape the same targets.

You must explicitly enable target auto-distribution on components by defining a `clustering` block.

```alloy
prometheus.scrape "default" {
    clustering {
        enabled = true
    }

    ...
}
```

A cluster state change is detected when a new node joins or an existing node leaves.
All participating components locally recalculate target ownership and re-balance the number of targets they’re scraping without explicitly communicating ownership over the network.

Target auto-distribution allows you to dynamically scale the number of {{< param "PRODUCT_NAME" >}} deployments to distribute workload during peaks.
It also provides resiliency because targets are automatically picked up by one of the node peers if a node leaves.

{{< param "PRODUCT_NAME" >}} uses a local consistent hashing algorithm to distribute targets, meaning that, on average, only ~1/N of the targets are redistributed.

Refer to component reference documentation to discover whether it supports clustering, such as:

- [prometheus.scrape][]
- [pyroscope.scrape][]
- [prometheus.operator.podmonitors][]
- [prometheus.operator.servicemonitors][]

## Cluster monitoring and troubleshooting

You can use the {{< param "PRODUCT_NAME" >}} UI [clustering page][] to monitor your cluster status.
Refer to [Debugging clustering issues][debugging] for additional troubleshooting information.

[run]: ../../reference/cli/run/#clustering
[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/#clustering-block
[pyroscope.scrape]: ../../reference/components/pyroscope/pyroscope.scrape/#clustering-block
[prometheus.operator.podmonitors]: ../../reference/components/prometheus/prometheus.operator.podmonitors/#clustering-block
[prometheus.operator.servicemonitors]: ../../reference/components/prometheus/prometheus.operator.servicemonitors/#clustering-block
[clustering page]: ../../troubleshoot/debug/#clustering-page
[debugging]: ../../troubleshoot/debug/#debugging-clustering-issues

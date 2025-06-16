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

Clustering allows a fleet of {{< param "PRODUCT_NAME" >}} deployments to work together for workload distribution and high availability.
It enables horizontally scalable deployments with minimal resource and operational overhead.

{{< param "PRODUCT_NAME" >}} uses an eventually consistent model to achieve clustering.
This model assumes all participating {{< param "PRODUCT_NAME" >}} deployments are interchangeable and converge on the same configuration file.

A standalone, non-clustered {{< param "PRODUCT_NAME" >}} behaves the same as a single-node cluster.

You configure clustering by passing `cluster` command-line flags to the [run][] command.

## Use cases

### Target auto-distribution

Target auto-distribution is the simplest use case of clustering.
It lets scraping components running on all peers distribute the scrape load among themselves.
Target auto-distribution requires all {{< param "PRODUCT_NAME" >}} deployments in the same cluster to access the same service discovery APIs and scrape the same targets.

You must explicitly enable target auto-distribution on components by defining a `clustering` block.

```alloy
prometheus.scrape "default" {
    clustering {
        enabled = true
    }

    ...
}
```

A cluster detects state changes when a node joins or leaves.
All participating components locally recalculate target ownership and re-balance the number of targets they're scraping without explicitly communicating ownership over the network.

Target auto-distribution lets you dynamically scale the number of {{< param "PRODUCT_NAME" >}} deployments to handle workload peaks.
It also provides resiliency because one of the node peers automatically picks up targets if a node leaves.

{{< param "PRODUCT_NAME" >}} uses a local consistent hashing algorithm to distribute targets.
On average, only ~1/N of the targets are redistributed.

Refer to the component reference documentation to check if a component supports clustering, such as:

- [`prometheus.scrape`][prometheus.scrape]
- [`pyroscope.scrape`][pyroscope.scrape]
- [`prometheus.operator.podmonitors`][prometheus.operator.podmonitors]
- [`prometheus.operator.servicemonitors`][prometheus.operator.servicemonitors]

## Cluster monitoring and troubleshooting

You can monitor your cluster status using the {{< param "PRODUCT_NAME" >}} UI [clustering page][].
Refer to [Debug clustering issues][debugging] for additional troubleshooting information.

[run]: ../../reference/cli/run/#clustering
[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/#clustering
[pyroscope.scrape]: ../../reference/components/pyroscope/pyroscope.scrape/#clustering
[prometheus.operator.podmonitors]: ../../reference/components/prometheus/prometheus.operator.podmonitors/#clustering
[prometheus.operator.servicemonitors]: ../../reference/components/prometheus/prometheus.operator.servicemonitors/#clustering
[clustering page]: ../../troubleshoot/debug/#clustering-page
[debugging]: ../../troubleshoot/debug/#debug-clustering-issues

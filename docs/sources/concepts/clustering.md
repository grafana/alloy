---
aliases:
- /docs/grafana-cloud/agent/flow/concepts/clustering/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/clustering/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/clustering/
- /docs/grafana-cloud/send-data/agent/flow/concepts/clustering/
canonical: https://grafana.com/docs/agent/latest/flow/concepts/clustering/
description: Learn about Grafana Agent clustering concepts
labels:
  stage: beta
menuTitle: Clustering
title: Clustering (beta)
weight: 500
---

# Clustering (beta)

Clustering enables a fleet of {{< param "PRODUCT_ROOT_NAME" >}}s to work together for workload distribution and high availability.
It helps create horizontally scalable deployments with minimal resource and operational overhead.

To achieve this, {{< param "PRODUCT_NAME" >}} makes use of an eventually consistent model that assumes all participating
{{< param "PRODUCT_ROOT_NAME" >}}s are interchangeable and converge on using the same configuration file.

The behavior of a standalone, non-clustered {{< param "PRODUCT_ROOT_NAME" >}} is the same as if it were a single-node cluster.

You configure clustering by passing `cluster` command-line flags to the [run][] command.

## Use cases

### Target auto-distribution

Target auto-distribution is the most basic use case of clustering.
It allows scraping components running on all peers to distribute the scrape load between themselves.
Target auto-distribution requires that all {{< param "PRODUCT_ROOT_NAME" >}} in the same cluster can reach the same service discovery APIs and scrape the same targets.

You must explicitly enable target auto-distribution on components by defining a `clustering` block.

```river
prometheus.scrape "default" {
    clustering {
        enabled = true
    }

    ...
}
```

A cluster state change is detected when a new node joins or an existing node leaves.
All participating components locally recalculate target ownership and re-balance the number of targets they’re scraping without explicitly communicating ownership over the network.

Target auto-distribution allows you to dynamically scale the number of {{< param "PRODUCT_ROOT_NAME" >}}s to distribute workload during peaks.
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

{{% docs/reference %}}
[run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/cli/run.md#clustering-beta"
[run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/cli/run.md#clustering-beta"
[prometheus.scrape]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.scrape.md#clustering-beta"
[prometheus.scrape]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.scrape.md#clustering-beta"
[pyroscope.scrape]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/pyroscope.scrape.md#clustering-beta"
[pyroscope.scrape]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/pyroscope.scrape.md#clustering-beta"
[prometheus.operator.podmonitors]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.operator.podmonitors.md#clustering-beta"
[prometheus.operator.podmonitors]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.operator.podmonitors.md#clustering-beta"
[prometheus.operator.servicemonitors]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.operator.servicemonitors.md#clustering-beta"
[prometheus.operator.servicemonitors]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.operator.servicemonitors.md#clustering-beta"
[clustering page]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/debug.md#clustering-page"
[clustering page]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/debug.md#clustering-page"
[debugging]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/debug.md#debugging-clustering-issues"
[debugging]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/debug.md#debugging-clustering-issues"
{{% /docs/reference %}}
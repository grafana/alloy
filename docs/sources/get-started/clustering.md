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

## Best practices

### Avoid issues with disproportionately large targets

When your environment has a mix of very large and average-sized targets, avoid running too cluster many instances. While clustering generally does a good job of sharding targets to achieve balanced workload distribution, significant target size disparity can lead to uneven load distribution. When you have a few disproportionately large targets among many instances, the nodes assigned these large targets will experience much higher load compard to others (e.g. samples/second in case of Prometheus metrics), potentially causing uneven load balancing or hitting resource limitations. In these scenarios, it's often better to scale vertically rather than horizontally to reduce the impact of outlier large targets. This approach ensures more consistent resource utilization across your deployment and prevents overloading specific instances.

### Use `--cluster.wait-for-size`, but with caution

When using clustering in a deployment where a single instance cannot handle the entire load, it's recommended to use the `--cluster.wait-for-size` flag to ensure a minimum cluster size before accepting traffic. However, leave a significant safety margin when configuring this value by setting it significantly smaller than your typical expected operational number of instances. When this condition is not met, the instances will completely stop processing traffic in cluster-enabled components so it's important to leave room for any unexpected events.

For example, if you're using Horizontal Pod Autoscalers (HPA) or PodDisruptionBudgets (PDB) in Kubernetes, ensure that the `--cluster.wait-for-size` flag is set to a value well below what your HPA and PDB minimums allow. This prevents traffic from stopping when Kubernetes instance counts temporarily drop below these thresholds during normal operations like pod termination or rolling updates. 

We recommend to use the `--cluster.wait-timeout` flag to set a reasonable timeout for the waiting period to limit the impact of potential misconfiguration. The appropriate timeout duration should be based on how quickly you expect your orchestration or incident response team to provision required number of instances. Be aware that when timeout passes the cluster may be too small to handle traffic and run into further issues.

### Do not enable clustering when you don't need it

While clustering scales to very large numbers of instances, it introduces additional overhead in the form of logs, metrics, potential alerts, and processing requirements. If you're not using components that specifically support and benefit from clustering, it's best to not enable clustering at all. A particularly common mistake is enabling clustering on logs collecting DaemonSets. Collecting logs from mounted node's pod logs does not benefit from having clustering enabled since each instance typically collects logs only from its own node. In such cases, enabling clustering only adds unnecessary complexity and resource usage without providing functional benefits.

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

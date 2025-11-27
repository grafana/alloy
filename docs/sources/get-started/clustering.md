---
canonical: https://grafana.com/docs/alloy/latest/get-started/clustering/
aliases:
  - ../concepts/clustering/ # /docs/alloy/latest/concepts/clustering/
description: Learn about Grafana Alloy clustering concepts
title: Clustering
weight: 70
---

# Clustering

You learned about components, expressions, syntax, and modules in the previous sections.
Now you'll learn about clustering, which allows multiple {{< param "PRODUCT_NAME" >}} deployments to work together for distributed data collection.

Clustering provides workload distribution and high availability.
It enables horizontally scalable deployments with minimal resource and operational overhead.

{{< param "PRODUCT_NAME" >}} uses an eventually consistent model with a gossip protocol to achieve clustering.
This model assumes all participating {{< param "PRODUCT_NAME" >}} deployments are interchangeable and use identical configurations.
The cluster uses a consistent hashing algorithm to distribute work among nodes.

A standalone, non-clustered {{< param "PRODUCT_NAME" >}} behaves the same as a single-node cluster.

You configure clustering by passing `--cluster.*` command-line flags to the [`alloy run`][run] command.
Cluster-enabled components must explicitly enable clustering through a `clustering` block in their configuration.

## Use cases

### Target auto-distribution

Target auto-distribution is the most common use case of clustering.
It lets scraping components running on all peers distribute the scrape load among themselves.

For target auto-distribution to work:

1. All {{< param "PRODUCT_NAME" >}} deployments in the same cluster must access the same service discovery APIs.
1. All deployments must scrape the same targets.

You must explicitly enable target auto-distribution on components by defining a `clustering` block.
This integrates with the component system you learned about in previous sections:

```alloy
prometheus.scrape "default" {
    targets = discovery.kubernetes.pods.targets

    clustering {
        enabled = true
    }

    forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
    endpoint {
        url = "https://prometheus.example.com/api/v1/write"
    }
}
```

When a cluster detects state changes (when a node joins or leaves), all participating components locally recalculate target ownership using a consistent hashing algorithm.
Components re-balance the targets they're scraping without explicitly communicating ownership over the network.
Each node uses 512 tokens in the hash ring for optimal load distribution.

Target auto-distribution lets you dynamically scale the number of {{< param "PRODUCT_NAME" >}} deployments to handle workload peaks.
It also provides resiliency because remaining node peers automatically pick up targets if a node leaves the cluster.

{{< param "PRODUCT_NAME" >}} uses a local consistent hashing algorithm to distribute targets.
When the cluster size changes, this algorithm redistributes only approximately 1/N of the targets, minimizing disruption.

Refer to the component reference documentation to check if a component supports clustering, such as:

- [`prometheus.scrape`][prometheus.scrape]
- [`pyroscope.scrape`][pyroscope.scrape]
- [`prometheus.operator.podmonitors`][prometheus.operator.podmonitors]
- [`prometheus.operator.servicemonitors`][prometheus.operator.servicemonitors]

## Best practices

### Avoid issues with disproportionately large targets

When your environment has a mix of very large and average-sized targets, avoid running too many cluster instances.
While clustering generally does a good job of sharding targets to achieve balanced workload distribution, significant target size disparity can lead to uneven load distribution.
When you have a few disproportionately large targets among many instances, the nodes assigned these large targets experience much higher load compared to others, for example samples per second for Prometheus metrics, potentially causing uneven load balancing or hitting resource limitations.
In these scenarios, it's often better to scale vertically rather than horizontally to reduce the impact of outlier large targets.
This approach ensures more consistent resource utilization across your deployment and prevents overloading specific instances.

### Use `--cluster.wait-for-size`, but with caution

When you use clustering in a deployment where a single instance can't handle the entire load, use the `--cluster.wait-for-size` flag to ensure a minimum cluster size before accepting traffic.
However, leave a significant safety margin when you configure this value by setting it significantly smaller than your typical expected operational number of instances.
When this condition isn't met, the instances stop processing traffic in cluster-enabled components, so it's important to leave room for any unexpected events.

For example, if you're using Horizontal Pod Autoscalers (HPA) or PodDisruptionBudgets (PDB) in Kubernetes, set the `--cluster.wait-for-size` flag to a value well below what your HPA and PDB minimums allow.
This prevents traffic from stopping when Kubernetes instance counts temporarily drop below these thresholds during normal operations like Pod termination or rolling updates.

It's recommended to use the `--cluster.wait-timeout` flag to set a reasonable timeout for the waiting period to limit the impact of potential misconfiguration.
You can base the timeout duration on how quickly you expect your orchestration or incident response team to provision the required number of instances.
Be aware that when the timeout passes, the cluster may be too small to handle traffic and can run into further issues.

### Don't enable clustering if you don't need it

While clustering scales to very large numbers of instances, it introduces additional overhead in the form of logs, metrics, potential alerts, and processing requirements.
If you're not using components that specifically support and benefit from clustering, it's best to not enable clustering at all.
A particularly common mistake is enabling clustering on logs collecting DaemonSets.
Collecting logs from Pods on the mounted node doesn't benefit from having clustering enabled since each instance typically collects logs only from Pods on its own node.
In such cases, enabling clustering only adds unnecessary complexity and resource usage without providing functional benefits.

## Cluster monitoring and troubleshooting

You can monitor your cluster status using the {{< param "PRODUCT_NAME" >}} UI [clustering page][].
Refer to [Debug clustering issues][debugging] for additional troubleshooting information.

## Next steps

Now that you understand how clustering works with {{< param "PRODUCT_NAME" >}} components, explore these topics:

- [Deploy {{< param "PRODUCT_NAME" >}}][deploy] - Set up clustered deployments in production environments.
- [Monitor {{< param "PRODUCT_NAME" >}}][monitor] - Learn about monitoring cluster health and performance.
- [Troubleshooting][debugging] - Debug clustering issues and interpret cluster metrics.

For detailed configuration:

- [`alloy run` command reference][run] - Configure clustering using command-line flags.
- [Component reference][components] - Explore clustering-enabled components like `prometheus.scrape` and `pyroscope.scrape`.

[run]: ../../reference/cli/run/#clustering
[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/#clustering
[pyroscope.scrape]: ../../reference/components/pyroscope/pyroscope.scrape/#clustering
[prometheus.operator.podmonitors]: ../../reference/components/prometheus/prometheus.operator.podmonitors/#clustering
[prometheus.operator.servicemonitors]: ../../reference/components/prometheus/prometheus.operator.servicemonitors/#clustering
[clustering page]: ../../troubleshoot/debug/#clustering-page
[debugging]: ../../troubleshoot/debug/#debug-clustering-issues
[components]: ../../reference/components/
[deploy]: ../../set-up/deploy/
[monitor]: ../../monitor/

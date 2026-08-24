---
canonical: https://grafana.com/docs/alloy/latest/configure/clustering/configure-alloy/
description: Learn how to enable clustering in Grafana Alloy
menuTitle: Configure Alloy
title: Configure Alloy for clustering
weight: 100
---

# Configure {{< param "PRODUCT_NAME" >}} for clustering

You can enable clustering in {{< param "PRODUCT_NAME" >}} with Helm chart settings or command-line flags.

{{< admonition type="note" >}}
Cluster mode in {{< param "PRODUCT_NAME" >}} allows instances to discover each other and form a cluster.
To distribute workload, you must also [enable clustering in individual components][distribute-workload].
{{< /admonition >}}

## Configure clustering with a Helm chart

You can enable clustering when {{< param "PRODUCT_NAME" >}} is installed on Kubernetes with a [Helm chart][install-helm].

### Before you begin

For multi-replica clustering on Kubernetes, Grafana Alloy recommends setting `controller.type` to `statefulset`.
Other controller types can still pass clustering flags when `alloy.clustering.enabled=true`, but `statefulset` is the recommended topology for stable clustered operation.

### Steps

To configure clustering:

1. Amend your existing `values.yaml` file to add `clustering.enabled=true` inside the `alloy` block:

   ```yaml
   alloy:
     clustering:
       enabled: true
   ```

1. Upgrade your installation to use the new `values.yaml` file:

   ```bash
   helm upgrade --namespace <NAMESPACE> <RELEASE_NAME> grafana/alloy -f values.yaml
   ```

   Replace the following:

   - _`<NAMESPACE>`_: The Kubernetes namespace where you installed the Helm chart.
   - _`<RELEASE_NAME>`_: The name of the installation you chose when you installed the Helm chart.

1. Use the {{< param "PRODUCT_NAME" >}} [UI][] to verify the cluster status:

   1. Click **Clustering** in the navigation bar.

   1. Ensure that all expected nodes appear in the resulting table.

## Configure clustering with command-line flags

If you run {{< param "PRODUCT_NAME" >}} outside of Kubernetes or without the Helm chart, use command-line flags to enable clustering.

### Required flags

Pass `--cluster.enabled` and one of the following peer-discovery flags to the [`alloy run`][run] command:

| Flag                       | Description                                                 |
| -------------------------- | ----------------------------------------------------------- |
| `--cluster.enabled`        | Enables clustering mode.                                    |
| `--cluster.join-addresses` | Comma-separated list of addresses of cluster nodes to join. |
| `--cluster.discover-peers` | Key-value tuples used to discover peers dynamically.        |

### Example

```bash
alloy run config.alloy \
  --cluster.enabled \
  --cluster.join-addresses=alloy-1:7946,alloy-2:7946
```

### Optional flags

You can customize clustering behavior with additional flags:

| Flag                             | Description                                      | Default    |
| -------------------------------- | ------------------------------------------------ | ---------- |
| `--cluster.advertise-address`    | Address to advertise to other cluster nodes.     | `""`       |
| `--cluster.advertise-interfaces` | Network interfaces to use for advertisement.     | `eth0,en0` |
| `--cluster.enable-tls`           | Enables TLS for cluster communication.           | `false`    |
| `--cluster.max-join-peers`       | Number of peers to join from the discovered set. | `5`        |
| `--cluster.name`                 | Name to prevent mixing clusters.                 | `""`       |
| `--cluster.node-name`            | The name to use for this node.                   | `""`       |
| `--cluster.rejoin-interval`      | Interval to rejoin the cluster.                  | `60s`      |
| `--cluster.tls-ca-path`          | Path to the CA certificate file.                 | `""`       |
| `--cluster.tls-cert-path`        | Path to the certificate file.                    | `""`       |
| `--cluster.tls-key-path`         | Path to the key file.                            | `""`       |
| `--cluster.tls-server-name`      | Server name to use for TLS communication.        | `""`       |
| `--cluster.wait-for-size`        | Minimum cluster size before traffic processing.  | `0`        |
| `--cluster.wait-timeout`         | Timeout for cluster size wait.                   | `0`        |

For production deployments, set `--cluster.wait-for-size` to your expected cluster size and `--cluster.wait-timeout` to a reasonable duration.
This ensures all nodes join before processing begins, which reduces duplicate scrapes and out-of-order samples at remote write during startup.

If you leave `--cluster.wait-for-size` at the default `0`, nodes can scrape before enough peers have joined, so two nodes may write the same series and trigger errors.
Refer to [Out of order errors][remote-write-out-of-order] for additional troubleshooting.

The following example configures a 3-node cluster where each node waits up to 5 minutes for all cluster members to join before it starts processing traffic:

```bash
alloy run config.alloy \
   --cluster.enabled \
   --cluster.join-addresses=alloy-1:7946,alloy-2:7946,alloy-3:7946 \
   --cluster.wait-for-size=3 \
   --cluster.wait-timeout=5m
```


Refer to the [`alloy run` reference][run] for complete details on all clustering flags.

## Verify cluster status

After you enable clustering, verify that all nodes have joined the cluster:

1. Open the {{< param "PRODUCT_NAME" >}} UI on any cluster node.
1. Click **Clustering** in the navigation bar.
1. Verify that all expected nodes appear in the cluster members table.

## Next steps

After you enable clustering in {{< param "PRODUCT_NAME" >}}, [configure components to distribute workload][distribute-workload].

[distribute-workload]: ../distribute-workload/
[install-helm]: ../../../set-up/install/kubernetes/
[remote-write-out-of-order]: ../../../reference/components/prometheus/prometheus.remote_write/#out-of-order-errors
[UI]: ../../../troubleshoot/debug/#clustering-page
[run]: ../../../reference/cli/run/#clustering

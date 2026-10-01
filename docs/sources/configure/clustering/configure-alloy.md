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

For multi-replica clustering on Kubernetes, set `controller.type` to `statefulset`.
A StatefulSet gives each Pod a stable network identity for peer discovery.
It also lets you choose how many {{< param "PRODUCT_NAME" >}} Pods to run instead of running one Pod on every node.
Use a DaemonSet only when your configuration must collect node-local data.

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

The first node that bootstraps a new cluster can omit peer discovery flags or connect to itself.

Cluster peers communicate over HTTP/2 on the built-in HTTP server.
Each node must accept connections on `--server.http.listen-addr` and on the address defined or inferred by `--cluster.advertise-address`.
If `--cluster.advertise-address` isn't set, {{< param "PRODUCT_NAME" >}} tries to infer an address from `--cluster.advertise-interfaces`.
The default interfaces are `eth0` and `en0`.
On Windows, set `--cluster.advertise-interfaces` to a valid network interface or set `--cluster.advertise-address` explicitly.
{{< param "PRODUCT_NAME" >}} fails to start if it can't determine the advertised address.

### Optional flags

You can customize clustering behavior with additional flags:

| Flag                             | Description                                                  | Default    |
| -------------------------------- | ------------------------------------------------------------ | ---------- |
| `--cluster.advertise-address`    | Address to advertise to other cluster nodes.                 | `""`       |
| `--cluster.advertise-interfaces` | Network interfaces to use for advertisement.                 | `eth0,en0` |
| `--cluster.enable-tls`           | Enables TLS for cluster communication.                       | `false`    |
| `--cluster.max-join-peers`       | Number of peers to join from the discovered set.             | `5`        |
| `--cluster.name`                 | Cluster name that nodes must share to join the same cluster. | `""`       |
| `--cluster.node-name`            | The name to use for this node.                               | `""`       |
| `--cluster.rejoin-interval`      | Interval to rejoin the cluster.                              | `60s`      |
| `--cluster.tls-ca-path`          | Path to the CA certificate file.                             | `""`       |
| `--cluster.tls-cert-path`        | Path to the certificate file.                                | `""`       |
| `--cluster.tls-key-path`         | Path to the key file.                                        | `""`       |
| `--cluster.tls-server-name`      | Server name to use for TLS communication.                    | `""`       |
| `--cluster.wait-for-size`        | Minimum cluster size before traffic processing.              | `0`        |
| `--cluster.wait-timeout`         | Timeout for cluster size wait.                               | `0`        |

To enable TLS for peer-to-peer communication, set `--cluster.enable-tls` and configure the related `--cluster.tls-*` flags for the CA certificate, certificate file, key file, and server name.

The `--cluster.rejoin-interval` flag controls how often each node rediscovers peers from `--cluster.join-addresses` and `--cluster.discover-peers` and tries to rejoin them.
This can help recover from split-brain conditions and support dynamic environments.
To discover peers only at startup, set `--cluster.rejoin-interval="0s"`.
After startup, nodes use gossip messages to converge on the cluster state.

The `--cluster.max-join-peers` flag limits how many discovered peers a node tries to connect to when joining or rejoining a cluster.
This is useful for large clusters where connecting to many peers can be expensive.
To disable the limit, set `--cluster.max-join-peers=0`.
If `--cluster.max-join-peers` is higher than the number of discovered peers, {{< param "PRODUCT_NAME" >}} connects to all discovered peers.

The `--cluster.name` flag prevents clusters from accidentally merging.
Nodes only join peers that use the same cluster name.
If a node tries to join a cluster with the wrong `--cluster.name`, it returns a `failed to join memberlist` error.

For `--cluster.join-addresses` DNS prefix syntax and `--cluster.discover-peers` tuple quoting rules, refer to the [`alloy run` reference][run].

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

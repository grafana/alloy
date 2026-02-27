---
canonical: https://grafana.com/docs/alloy/latest/configure/clustering/configure-alloy/
description: Learn how to enable clustering in Grafana Alloy
menuTitle: Configure Alloy
title: Configure Alloy for clustering
weight: 100
---

# Configure {{< param "PRODUCT_NAME" >}} for clustering

You can enable clustering in {{< param "PRODUCT_NAME" >}} using Helm Chart settings or command-line flags.

{{< admonition type="note" >}}
Cluster mode in {{< param "PRODUCT_NAME" >}} allows instances to discover each other and form a cluster.
To distribute workload, you must also [enable clustering in individual components][distribute-workload].
{{< /admonition >}}

## Configure clustering with Helm Chart

You can enable clustering when {{< param "PRODUCT_NAME" >}} is installed on Kubernetes using the [Helm chart][install-helm].

### Before you begin

Ensure that your `values.yaml` file has `controller.type` set to `statefulset`.

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
   helm upgrade <RELEASE_NAME> -f values.yaml
   ```

   Replace the following:

   - _`<RELEASE_NAME>`_: The name of the installation you chose when you installed the Helm chart.

1. Use the {{< param "PRODUCT_NAME" >}} [UI][] to verify the cluster status:

   1. Click **Clustering** in the navigation bar.

   1. Ensure that all expected nodes appear in the resulting table.

## Configure clustering with command-line flags

If you run {{< param "PRODUCT_NAME" >}} outside of Kubernetes or without the Helm chart, use command-line flags to enable clustering.

### Required flags

Pass the following flags to the [`alloy run`][run] command:

| Flag                       | Description                                                 |
| -------------------------- | ----------------------------------------------------------- |
| `--cluster.enabled`        | Enables clustering mode.                                    |
| `--cluster.join-addresses` | Comma-separated list of addresses of cluster nodes to join. |

### Example

```bash
alloy run config.alloy \
  --cluster.enabled \
  --cluster.join-addresses=alloy-1:7946,alloy-2:7946
```

### Optional flags

You can customize clustering behavior with additional flags:

| Flag                             | Description                                     | Default    |
| -------------------------------- | ----------------------------------------------- | ---------- |
| `--cluster.name`                 | Name to prevent mixing clusters.                | `""`       |
| `--cluster.advertise-address`    | Address to advertise to other cluster nodes.    | `""`       |
| `--cluster.advertise-interfaces` | Network interfaces to use for advertisement.    | `eth0,en0` |
| `--cluster.rejoin-interval`      | Interval to rejoin the cluster.                 | `60s`      |
| `--cluster.wait-for-size`        | Minimum cluster size before traffic processing. | `0`        |
| `--cluster.wait-timeout`         | Timeout for cluster size wait.                  | `0`        |

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
[UI]: ../../../troubleshoot/debug/#clustering-page
[run]: ../../../reference/cli/run/#clustering

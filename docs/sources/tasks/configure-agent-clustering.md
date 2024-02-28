---
aliases:
- /docs/grafana-cloud/agent/flow/tasks/configure-agent-clustering/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tasks/configure-agent-clustering/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tasks/configure-agent-clustering/
- /docs/grafana-cloud/send-data/agent/flow/tasks/configure-agent-clustering/
# Previous page aliases for backwards compatibility:
- /docs/grafana-cloud/agent/flow/getting-started/configure-agent-clustering/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/getting-started/configure-agent-clustering/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/getting-started/configure-agent-clustering/
- /docs/grafana-cloud/send-data/agent/flow/getting-started/configure-agent-clustering/
- ../getting-started/configure-agent-clustering/ # /docs/agent/latest/flow/getting-started/configure-agent-clustering/
canonical: https://grafana.com/docs/agent/latest/flow/tasks/configure-agent-clustering/
description: Learn how to configure Grafana Agent clustering in an existing installation
menuTitle: Configure Grafana Agent clustering
title: Configure Grafana Agent clustering in an existing installation
weight: 400
---

# Configure {{% param "PRODUCT_NAME" %}} clustering in an existing installation

You can configure {{< param "PRODUCT_NAME" >}} to run with [clustering][] so that individual {{< param "PRODUCT_ROOT_NAME" >}}s can work together for workload distribution and high availability.

> **Note:** Clustering is a [beta][] feature. Beta features are subject to breaking
> changes and may be replaced with equivalent functionality that covers the same use case.

This topic describes how to add clustering to an existing installation.

## Configure {{% param "PRODUCT_NAME" %}} clustering with Helm Chart

This section guides you through enabling clustering when {{< param "PRODUCT_NAME" >}} is installed on Kubernetes using the {{< param "PRODUCT_ROOT_NAME" >}} [Helm chart][install-helm].

### Before you begin

- Ensure that your `values.yaml` file has `controller.type` set to `statefulset`.

### Steps

To configure clustering:

1. Amend your existing `values.yaml` file to add `clustering.enabled=true` inside the `agent` block.

   ```yaml
   agent:
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

{{% docs/reference %}}
[clustering]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/clustering.md"
[clustering]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/clustering.md"
[beta]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/stability.md#beta"
[beta]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/stability.md#beta"
[install-helm]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/get-started/install/kubernetes.md"
[install-helm]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/get-started/install/kubernetes.md"
[UI]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/tasks/debug.md#component-detail-page"
[UI]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/tasks/debug.md#component-detail-page"
{{% /docs/reference %}}

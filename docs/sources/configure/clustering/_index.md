---
canonical: https://grafana.com/docs/alloy/latest/configure/clustering/
aliases:
  - ../tasks/configure-alloy-clustering/ # /docs/alloy/latest/tasks/configure-alloy-clustering/
description: Learn how to configure Grafana Alloy clustering in an existing installation
menuTitle: Clustering
title: Configure Grafana Alloy clustering in an existing installation
weight: 100
---

# Configure {{% param "PRODUCT_NAME" %}} clustering in an existing installation

You can configure {{< param "PRODUCT_NAME" >}} to run with [clustering][] so that individual {{< param "PRODUCT_NAME" >}}s can work together for workload distribution and high availability.

This topic describes how to add clustering to an existing installation.

## Configure {{% param "PRODUCT_NAME" %}} clustering with Helm Chart

This section guides you through enabling clustering when {{< param "PRODUCT_NAME" >}} is installed on Kubernetes using the {{< param "PRODUCT_NAME" >}} [Helm chart][install-helm].

### Before you begin

- Ensure that your `values.yaml` file has `controller.type` set to `statefulset`.

### Steps

To configure clustering:

1. Amend your existing `values.yaml` file to add `clustering.enabled=true` inside the `alloy` block.

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

[clustering]: ../clustering/
[install-helm]: ../../set-up/install/kubernetes/
[UI]: ../../troubleshoot/debug/#component-detail-page

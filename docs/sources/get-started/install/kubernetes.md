---
canonical: https://grafana.com/docs/alloy/latest/get-started/install/kubernetes/
description: Learn how to deploy Grafana Alloy on Kubernetes
menuTitle: Kubernetes
title: Deploy Grafana Alloy on Kubernetes
weight: 200
---

# Deploy {{% param "PRODUCT_NAME" %}} on Kubernetes

{{< param "PRODUCT_NAME" >}} can be deployed on Kubernetes by using the Helm chart for {{< param "PRODUCT_ROOT_NAME" >}}.

## Before you begin

* Install [Helm][] on your computer.
* Configure a Kubernetes cluster that you can use for {{< param "PRODUCT_NAME" >}}.
* Configure your local Kubernetes context to point at the cluster.

## Deploy

To deploy {{< param "PRODUCT_ROOT_NAME" >}} on Kubernetes using Helm, run the following commands in a terminal window:

1. Add the Grafana Helm chart repository:

   ```shell
   helm repo add grafana https://grafana.github.io/helm-charts
   ```

1. Update the Grafana Helm chart repository:

   ```shell
   helm repo update
   ```

1. Install {{< param "PRODUCT_ROOT_NAME" >}}:

   ```shell
   helm install <RELEASE_NAME> grafana/grafana-alloy
   ```

   Replace the following:

   -  _`<RELEASE_NAME>`_: The name to use for your {{< param "PRODUCT_ROOT_NAME" >}} installation, such as `grafana-alloy`.

For more information on the {{< param "PRODUCT_ROOT_NAME" >}} Helm chart, refer to the Helm chart documentation on [Artifact Hub][].

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

[Helm]: https://helm.sh
[Artifact Hub]: https://artifacthub.io/packages/helm/grafana/grafana-alloy
[Configure]: ../../../tasks/configure/configure-kubernetes/

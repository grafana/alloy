---
canonical: https://grafana.com/docs/alloy/latest/set-up/install/kubernetes/
aliases:
  - ../../get-started/install/kubernetes/ # /docs/alloy/latest/get-started/install/kubernetes/
description: Learn how to deploy Grafana Alloy on Kubernetes
menuTitle: Kubernetes
title: Deploy Grafana Alloy on Kubernetes
weight: 200
---

# Deploy {{% param "FULL_PRODUCT_NAME" %}} on Kubernetes

{{< param "PRODUCT_NAME" >}} can be deployed on Kubernetes by using the Helm chart for {{< param "PRODUCT_NAME" >}}.

## Before you begin

* Install [Helm][] on your computer.
* Configure a Kubernetes cluster that you can use for {{< param "PRODUCT_NAME" >}}.
* Configure your local Kubernetes context to point at the cluster.

## Deploy

To deploy {{< param "PRODUCT_NAME" >}} on Kubernetes using Helm, run the following commands in a terminal window:

1. Add the Grafana Helm chart repository:

   ```shell
   helm repo add grafana https://grafana.github.io/helm-charts
   ```

1. Update the Grafana Helm chart repository:

   ```shell
   helm repo update
   ```

1. Create a namespace for {{< param "PRODUCT_NAME" >}}:

   ```shell
   kubectl create namespace <NAMESPACE>
   ```

   Replace the following:

   - _`<NAMESPACE>`_: The namespace to use for your {{< param "PRODUCT_NAME" >}} installation, such as `alloy`.

1. Install {{< param "PRODUCT_NAME" >}}:

   ```shell
   helm install --namespace <NAMESPACE> <RELEASE_NAME> grafana/alloy
   ```

   Replace the following:

   - _`<NAMESPACE>`_: The namespace created in the previous step.
   - _`<RELEASE_NAME>`_: The name to use for your {{< param "PRODUCT_NAME" >}} installation, such as `alloy`.

1. Verify that the {{< param "PRODUCT_NAME" >}} pods are running:

   ```shell
   kubectl get pods --namespace <NAMESPACE>
   ```

   Replace the following:

   - _`<NAMESPACE>`_: The namespace used in the previous step.

You have successfully deployed {{< param "PRODUCT_NAME" >}} on Kubernetes, using default Helm settings.

## Next steps

- [Configure {{< param "PRODUCT_NAME" >}}][Configure]

<!-- - Refer to the [{{< param "PRODUCT_NAME" >}} Helm chart documentation on Artifact Hub][Artifact Hub] for more information about the Helm chart. -->

[Helm]: https://helm.sh
[Artifact Hub]: https://artifacthub.io/packages/helm/grafana/alloy
[Configure]: ../../../configure/kubernetes/

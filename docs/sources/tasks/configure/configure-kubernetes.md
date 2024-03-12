---
canonical: https://grafana.com/docs/alloy/latest/tasks/configure/configure-kubernetes/
description: Learn how to configure Grafana Alloy on Kubernetes
menuTitle: Kubernetes
title: Configure Grafana Alloy on Kubernetes
weight: 200
---

# Configure {{% param "PRODUCT_NAME" %}} on Kubernetes

To configure {{< param "PRODUCT_NAME" >}} on Kubernetes, perform the following steps:

1. Download a local copy of [values.yaml][] for the Helm chart.

1. Make changes to your copy of `values.yaml` to customize settings for the Helm chart.

   Refer to the inline documentation in the `values.yaml` for more information about each option.

1. Run the following command in a terminal to upgrade your {{< param "PRODUCT_NAME" >}} installation:

   ```shell
   helm upgrade RELEASE_NAME grafana/alloy -f VALUES_PATH
   ```

   1. Replace `RELEASE_NAME` with the name you used for your {{< param "PRODUCT_NAME" >}} installation.

   1. Replace `VALUES_PATH` with the path to your copy of `values.yaml` to use.

## Kustomize considerations

If you are using [Kustomize][] to inflate and install the [Helm chart][], be careful when using a `configMapGenerator` to generate the ConfigMap containing the configuration.
By default, the generator appends a hash to the name and patches the resource mentioning it, triggering a rolling update.

This behavior is undesirable for {{< param "PRODUCT_NAME" >}} because the startup time can be significant depending on the size of the Write-Ahead Log.
You can use the [Helm chart][] sidecar container to watch the ConfigMap and trigger a dynamic reload.

The following is an example snippet of a `kustomization` that disables this behavior:

```yaml
configMapGenerator:
  - name: alloy
    files:
      - config.river
    options:
      disableNameSuffixHash: true
```
[values.yaml]: https://raw.githubusercontent.com/grafana/alloy/main/operations/helm/charts/alloy/values.yaml
[Helm chart]: https://github.com/grafana/alloy/tree/main/operations/helm/charts/alloy
[Kustomize]: https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/

---
aliases:
- /docs/grafana-cloud/agent/flow/tasks/configure/configure-kubernetes/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tasks/configure/configure-kubernetes/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tasks/configure/configure-kubernetes/
- /docs/grafana-cloud/send-data/agent/flow/tasks/configure/configure-kubernetes/
# Previous page aliases for backwards compatibility:
- /docs/grafana-cloud/agent/flow/setup/configure/configure-kubernetes/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/setup/configure/configure-kubernetes/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/setup/configure/configure-kubernetes/
- /docs/grafana-cloud/send-data/agent/flow/setup/configure/configure-kubernetes/
- ../../setup/configure/configure-kubernetes/ # /docs/agent/latest/flow/setup/configure/configure-kubernetes/
canonical: https://grafana.com/docs/agent/latest/flow/tasks/configure/configure-kubernetes/
description: Learn how to configure Grafana Agent Flow on Kubernetes
menuTitle: Kubernetes
title: Configure Grafana Agent Flow on Kubernetes
weight: 200
---

# Configure {{% param "PRODUCT_NAME" %}} on Kubernetes

To configure {{< param "PRODUCT_NAME" >}} on Kubernetes, perform the following steps:

1. Download a local copy of [values.yaml][] for the Helm chart.

1. Make changes to your copy of `values.yaml` to customize settings for the
   Helm chart.

   Refer to the inline documentation in the `values.yaml` for more information about each option.

1. Run the following command in a terminal to upgrade your {{< param "PRODUCT_NAME" >}} installation:

   ```shell
   helm upgrade RELEASE_NAME grafana/grafana-agent -f VALUES_PATH
   ```

   1. Replace `RELEASE_NAME` with the name you used for your {{< param "PRODUCT_NAME" >}} installation.

   1. Replace `VALUES_PATH` with the path to your copy of `values.yaml` to use.

[values.yaml]: https://raw.githubusercontent.com/grafana/agent/main/operations/helm/charts/grafana-agent/values.yaml

## Kustomize considerations

If you are using [Kustomize][] to inflate and install the [Helm chart][], be careful
when using a `configMapGenerator` to generate the ConfigMap containing the
configuration. By default, the generator appends a hash to the name and patches
the resource mentioning it, triggering a rolling update.

This behavior is undesirable for {{< param "PRODUCT_NAME" >}} because the startup time can be significant depending on the size of the Write-Ahead Log.
You can use the [Helm chart][] sidecar container to watch the ConfigMap and trigger a dynamic reload.

The following is an example snippet of a `kustomization` that disables this behavior:

```yaml
configMapGenerator:
  - name: grafana-agent
    files:
      - config.river
    options:
      disableNameSuffixHash: true
```

[Helm chart]: https://github.com/grafana/agent/tree/main/operations/helm/charts/grafana-agent
[Kustomize]: https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/

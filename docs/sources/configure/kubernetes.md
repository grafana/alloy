---
canonical: https://grafana.com/docs/alloy/latest/configure/kubernetes/
aliases:
  - ../tasks/configure/configure-kubernetes/ # /docs/alloy/latest/tasks/configure/configure-kubernetes/
description: Learn how to configure Grafana Alloy on Kubernetes
menuTitle: Kubernetes
title: Configure Grafana Alloy on Kubernetes
weight: 200
---

# Configure {{% param "FULL_PRODUCT_NAME" %}} on Kubernetes

This page describes how to apply a new configuration to {{< param "PRODUCT_NAME" >}} when running on Kubernetes with the Helm chart.
It assumes that:

- You have [installed {{< param "PRODUCT_NAME" >}} on Kubernetes using the Helm chart][k8s-install].
- You already have a new {{< param "PRODUCT_NAME" >}} configuration that you want to apply to your Helm chart installation.

Refer to [Collect and forward data][collect] for information about configuring {{< param "PRODUCT_NAME" >}} to collect and forward data.

[collect]: ../../collect/
[k8s-install]: ../../set-up/install/kubernetes/

## Configure the Helm chart

To modify {{< param "PRODUCT_NAME" >}}'s Helm chart configuration, perform the following steps:

1. Create a local `values.yaml` file with a new Helm chart configuration.

   1. You can use your own copy of the values file or download a copy of the
      default [values.yaml][].

   1. Make changes to your `values.yaml` to customize settings for the
      Helm chart.

      Refer to the inline documentation in the default [values.yaml][] for more
      information about each option.

1. Run the following command in a terminal to upgrade your {{< param "PRODUCT_NAME" >}} installation:

   ```shell
   helm upgrade --namespace <NAMESPACE> <RELEASE_NAME> grafana/alloy -f <VALUES_PATH>
   ```

   Replace the following:
   - _`<NAMESPACE>`_: The namespace you used for your {{< param "PRODUCT_NAME" >}} installation.
   - _`<RELEASE_NAME>`_: The name you used for your {{< param "PRODUCT_NAME" >}} installation.
   - _`<VALUES_PATH>`_: The path to your copy of `values.yaml` to use.

[values.yaml]: https://raw.githubusercontent.com/grafana/alloy/main/operations/helm/charts/alloy/values.yaml

## Kustomize considerations

If you are using [Kustomize][] to inflate and install the [Helm chart][], be careful when using a `configMapGenerator` to generate the ConfigMap containing the configuration.
By default, the generator appends a hash to the name and patches the resource mentioning it, triggering a rolling update.

This behavior is undesirable for {{< param "PRODUCT_NAME" >}} because the startup time can be significant, for example, when your deployment has a large metrics Write-Ahead Log.
You can use the [Helm chart][] sidecar container to watch the ConfigMap and trigger a dynamic reload.

The following is an example snippet of a `kustomization` that disables this behavior:

```yaml
configMapGenerator:
  - name: alloy
    files:
      - config.alloy
    options:
      disableNameSuffixHash: true
```

## Configure the {{< param "PRODUCT_NAME" >}}

This section describes how to modify the {{< param "PRODUCT_NAME" >}} configuration, which is stored in a ConfigMap in the Kubernetes cluster.
There are two methods to perform this task.

### Method 1: Modify the configuration in the values.yaml file

Use this method if you prefer to embed your {{< param "PRODUCT_NAME" >}} configuration in the Helm chart's `values.yaml` file.

1. Modify the configuration file contents directly in the `values.yaml` file:

   ```yaml
   alloy:
     configMap:
       content: |-
         // Write your Alloy config here:
         logging {
           level = "info"
           format = "logfmt"
         }
   ```

1. Run the following command in a terminal to upgrade your {{< param "PRODUCT_NAME" >}} installation:

   ```shell
   helm upgrade --namespace <NAMESPACE> <RELEASE_NAME> grafana/alloy -f <VALUES_PATH>
   ```

   Replace the following:

   - _`<NAMESPACE>`_: The namespace you used for your {{< param "PRODUCT_NAME" >}} installation.
   - _`<RELEASE_NAME>`_: The name you used for your {{< param "PRODUCT_NAME" >}} installation.
   - _`<VALUES_PATH>`_: The path to your copy of `values.yaml` to use.

### Method 2: Create a separate ConfigMap from a file

Use this method if you prefer to write your {{< param "PRODUCT_NAME" >}} configuration in a separate file.

1. Write your configuration to a file, for example, `config.alloy`.

   ```alloy
   // Write your Alloy config here:
   logging {
     level = "info"
     format = "logfmt"
   }
   ```

1. Create a ConfigMap called `alloy-config` from the above file:

   ```shell
   kubectl create configmap --namespace <NAMESPACE> alloy-config "--from-file=config.alloy=./config.alloy"
   ```

   Replace the following:

   - _`<NAMESPACE>`_: The namespace you used for your {{< param "PRODUCT_NAME" >}} installation.

1. Modify Helm Chart's configuration in your `values.yaml` to use the existing ConfigMap:

   ```yaml
   alloy:
     configMap:
       create: false
       name: alloy-config
       key: config.alloy
   ```

1. Run the following command in a terminal to upgrade your {{< param "PRODUCT_NAME" >}} installation:

   ```shell
   helm upgrade --namespace <NAMESPACE> <RELEASE_NAME> grafana/alloy -f <VALUES_PATH>
   ```

   Replace the following:

   - _`<NAMESPACE>`_: The namespace you used for your {{< param "PRODUCT_NAME" >}} installation.
   - _`<RELEASE_NAME>`_: The name you used for your {{< param "PRODUCT_NAME" >}} installation.
   - _`<VALUES_PATH>`_: The path to your copy of `values.yaml` to use.

[values.yaml]: https://raw.githubusercontent.com/grafana/alloy/main/operations/helm/charts/alloy/values.yaml
[Helm chart]: https://github.com/grafana/alloy/tree/main/operations/helm/charts/alloy
[Kustomize]: https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/

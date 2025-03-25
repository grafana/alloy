---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-kubernetes-logs/
description: Learn how to use Grafana Alloy to monitor Kubernetes logs
menuTitle: Monitor Kubernetes logs
title: Monitor Kubernetes logs with Grafana Alloy
weight: 600
---

# Monitor Kubernetes logs with {{% param "FULL_PRODUCT_NAME" %}}

Kubernetes captures logs from each container in a running Pod.
You can use {{< param "PRODUCT_NAME" >}} to collect your Kubernetes logs, forward them to a Grafana stack, and create a Grafana dashboard to monitor your Kubernetes deployment.

The `alloy-scenarios` repository provides series of complete working examples of {{< param "PRODUCT_NAME" >}} deployments.
You can clone the repository and use the example deployments to understand how {{< param "PRODUCT_NAME" >}} can collect, process, and export telemetry signals.

This example scenario uses a Kubernetes Monitoring Helm chart to deploy and monitor Kubernetes logs and installs three Helm charts, Loki, Grafana, and {{< param "PRODUCT_NAME" >}}.
The Helm chart abstracts the need to configure {{< param "PRODUCT_NAME" >}} and deploys best practices for monitoring Kubernetes clusters.

{{< param "PRODUCT_NAME" >}}, installed with `k8s-monitoring-helm`, collects two different log sources, Pod Logs and Kubernetes Events.

## Before you begin

This example requires:

* Docker
* Git
* [Helm](https://helm.sh/docs/intro/install/)
* [kind](https://kind.sigs.k8s.io/docs/user/quick-start/)

## Clone the repository

```shell
git clone https://github.com/grafana/alloy-scenarios.git
```

## Deploy the Grafana stack

1. Change directory to `alloy-scenarios/k8s-logs`.

   ```bash
   cd alloy-scenarios/k8s-logs
   ```

1. Use kind to create a local Kubernetes cluster.
   The `kind.yml` file provides the kind cluster configuration.

   ```shell
   kind create cluster --config kind.yml
   ```

1. Add the Grafana Helm repository.

   ```shell
   helm repo add grafana https://grafana.github.io/helm-charts
   ```

1. Create the `meta` and `prod` namespaces/

   ```shell
   kubectl create namespace meta && \
   kubectl create namespace prod
   ```

1. Deploy Loki in the `meta` namespace. Loki stores the collected logs.
   The `loki-values.yml` file contains the configuration for the Loki Helm chart.

   ```bash
   helm install --values loki-values.yml loki grafana/loki -n meta
   ```

   This Helm chart installs Loki in monolithic mode.
   For more information on Loki modes, see the [Loki documentation](https://grafana.com/docs/loki/latest/get-started/deployment-modes/).

1. Deploy Grafana in the `meta` namespace. You can Grafana to visualize the logs stored in Loki.
   The `grafana-values.yml` file contains the configuration for the Grafana Helm chart.

   ```shell
   helm install --values grafana-values.yml grafana grafana/grafana --namespace meta
   ```

   This Helm chart installs Grafana and sets the `datasources.datasources.yaml` field to the Loki data source configuration.

1. Deploy {{< param "PRODUCT_NAME" >}} in the `meta` namespace.
   The `k8s-monitoring-values.yml` file contains the configuration for the Kubernetes monitoring Helm chart.

   ```shell
   helm install --values ./k8s-monitoring-values.yml k8s grafana/k8s-monitoring -n meta --create-namespace
   ```

   This Helm chart installs {{< param "PRODUCT_NAME" >}} and specifies the log Pod Logs and Kubernetes Events sources that {{< param "PRODUCT_NAME" >}} collects logs from.

## Set up port forwarding

1. Port-forward the Grafana Pod to your local machine.

   1. Get the name of the Grafana Pod.

      ```shell
      export POD_NAME=$(kubectl get pods --namespace meta -l "app.kubernetes.io/name=grafana,app.kubernetes.io/instance=grafana" -o jsonpath="{.items[0].metadata.name}")
      ```

   1. Use `kubectl` to set up the port-forwarding.

      ```shell
      kubectl --namespace meta port-forward $POD_NAME 3000
      ```

1. Port-forward the {{< param "PRODUCT_NAME" >}} Pod to your local machine.

   1. Get the name of the {{< param "PRODUCT_NAME" >}} Pod.

      ```shell
      export POD_NAME=$(kubectl get pods --namespace meta -l "app.kubernetes.io/name=alloy-logs,app.kubernetes.io/instance=k8s" -o jsonpath="{.items[0].metadata.name}")
      ```

   1. Use `kubectl` to set up the port-forwarding.

      ```shell
      kubectl --namespace meta port-forward $POD_NAME 12345
      ```

## Add a demo application to `prod`

Deploy default version of Grafana Tempo as a sample application to the `prod` namespace and generate some logs.
Tempo is a distributed tracing backend that's used to store and query traces.
Normally Tempo would sit next to Loki and Grafana in the meta namespace, but for the purpose of this example, is functions as the primary application generating logs.

```shell
helm install tempo grafana/tempo-distributed -n prod
```

## Visualise your data

To use the Grafana Logs Drilldown, open your browser and navigate to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualise your metrics and logs, open your browser and navigate to [`http://localhost:3000/dashboards`](http://localhost:3000/dashboards).

## Understand the Kubernetes Monitoring Helm chart

The Kubernetes Monitoring Helm chart, `k8s-monitoring-helm`, is used for gathering, scraping, and forwarding Kubernetes telemetry data to a Grafana stack.
This includes the ability to collect metrics, logs, traces, and continuous profiling data.

### Define the cluster

Define the cluster name as `meta-monitoring-tutorial`.
This a static label that's attached to all logs collected by the Kubernetes Monitoring Helm chart.

```yaml
cluster:
  name: meta-monitoring-tutorial
```

### `destinations`

Define a destination named `loki` that's used to forward logs to Loki.
The `url` attribute specifies the URL of the Loki gateway.

```yaml
destinations:
  - name: loki
    type: loki
    url: http://loki-gateway.meta.svc.cluster.local/loki/api/v1/push
```

### `clusterEvents`

Enable the collection of cluster events.

* `collector`: Use the `alloy-logs` collector to collect logs.
* `namespaces`: specifies the meta and prod namespaces to collect logs from.

```yaml
clusterEvents:
  enabled: true
  collector: alloy-logs
  namespaces:
    - meta
    - prod
```

### `nodeLogs`

Disable the collection of node logs.
Collecting node logs requires that you mount `/var/log/journal` and this is out of scope for this example.

```yaml
nodeLogs:
  enabled: false
```

### `podLogs`

Enable the collection of Pod logs.

* `labelsToKeep`: The labels to keep when collecting logs.
  This doesn't drop logs. This is useful when you don't want to apply a high cardinality label.
  This configuration removes `pod` from the labels to keep.
* `structuredMetadata`: The structured metadata to collect.
  This configuration sets the structured metadata `pod` to keep the Pod name for querying.

```yaml
podLogs:
  enabled: true
  gatherMethod: kubernetesApi
  collector: alloy-logs
  labelsToKeep: ["app_kubernetes_io_name","container","instance","job","level","namespace","service_name","service_namespace","deployment_environment","deployment_environment_name"]
  structuredMetadata:
    pod: pod  # Set structured metadata "pod" from label "pod"
  namespaces:
    - meta
    - prod
```

### Define the {{% param "PRODUCT_NAME" %}} role

The Kubernetes Monitoring Helm chart deploys only what you need and nothing more.
In this case, the configuration tells the Helm chart to deploy {{< param "PRODUCT_NAME" >}} with the capability to collect logs.
Metrics, traces, and continuous profiling are disabled.

```yaml
alloy-singleton:
  enabled: false

alloy-metrics:
  enabled: false

alloy-logs:
  enabled: true
  alloy:
    mounts:
      varlog: false
    clustering:
      enabled: true

alloy-profiles:
  enabled: false

alloy-receiver:
  enabled: false
```

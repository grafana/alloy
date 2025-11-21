---
canonical: https://grafana.com/docs/alloy/latest/monitor/monitor-kubernetes-logs/
description: Learn how to use Grafana Alloy to monitor Kubernetes logs
menuTitle: Monitor Kubernetes logs
title: Monitor Kubernetes logs with Grafana Alloy
weight: 600
---

# Monitor Kubernetes logs with {{% param "FULL_PRODUCT_NAME" %}}

Kubernetes captures logs from each container in a running Pod.  
With {{< param "PRODUCT_NAME" >}}, you can collect Kubernetes logs, forward them to a Grafana stack, and create dashboards to monitor your Kubernetes Deployment.

The [`alloy-scenarios`][scenarios] repository contains complete examples of {{< param "PRODUCT_NAME" >}} deployments.  
Clone the repository and use the examples to understand how {{< param "PRODUCT_NAME" >}} collects, processes, and exports telemetry signals.

This example scenario uses a Kubernetes Monitoring Helm chart to deploy and monitor Kubernetes logs.  
It installs three Helm charts: Loki, Grafana, and {{< param "PRODUCT_NAME" >}}.  
The Helm chart simplifies configuration and deploys best practices for monitoring Kubernetes clusters.

{{< param "PRODUCT_NAME" >}}, installed with `k8s-monitoring-helm`, collects two log sources: [Pod Logs][] and [Kubernetes Events][].

[Pod Logs]: https://kubernetes.io/docs/concepts/cluster-administration/logging/#basic-logging-in-kubernetes  
[Kubernetes Events]: https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/event-v1/  
[scenarios]: https://github.com/grafana/alloy-scenarios/

## Before you begin

Ensure you have the following:

* [Docker](https://www.docker.com/)  
* [Git](https://git-scm.com/)  
* [Helm](https://helm.sh/docs/intro/install/)  
* [kind](https://kind.sigs.k8s.io/docs/user/quick-start/)

## Clone and deploy the example

Follow these steps to clone the scenarios repository and deploy the monitoring example:

1. Clone the {{< param "PRODUCT_NAME" >}} scenarios repository:

   ```shell
   git clone https://github.com/grafana/alloy-scenarios.git
   ```

1. Set up the example Kubernetes environment:

   1. Navigate to the `alloy-scenarios/k8s-logs` directory:

      ```shell
      cd alloy-scenarios/k8s-logs
      ```

   1. Create a local Kubernetes cluster using kind.  
      The `kind.yml` file provides the cluster configuration:

      ```shell
      kind create cluster --config kind.yml
      ```

   1. Add the Grafana Helm repository:

      ```shell
      helm repo add grafana https://grafana.github.io/helm-charts
      ```

   1. Create the `meta` and `prod` namespaces:

      ```shell
      kubectl create namespace meta && \
      kubectl create namespace prod
      ```

1. Deploy Loki in the `meta` namespace.  
   Loki stores the collected logs.  
   The `loki-values.yml` file contains the Loki Helm chart configuration:

   ```shell
   helm install --values loki-values.yml loki grafana/loki -n meta
   ```

   This Helm chart installs Loki in monolithic mode.  
   For more details, refer to the [Loki documentation](https://grafana.com/docs/loki/latest/get-started/deployment-modes/).

1. Deploy Grafana in the `meta` namespace.  
   You can use Grafana to visualize the logs stored in Loki.  
   The `grafana-values.yml` file contains the Grafana Helm chart configuration:

   ```shell
   helm install --values grafana-values.yml grafana grafana/grafana --namespace meta
   ```

   This Helm chart installs Grafana and sets the `datasources.datasources.yaml` field to the Loki data source configuration.

1. Deploy {{< param "PRODUCT_NAME" >}} in the `meta` namespace.  
   The `k8s-monitoring-values.yml` file contains the Kubernetes monitoring Helm chart configuration:

   ```shell
   helm install --values ./k8s-monitoring-values.yml k8s grafana/k8s-monitoring -n meta --create-namespace
   ```

   This Helm chart installs {{< param "PRODUCT_NAME" >}} and specifies the Pod logs and Kubernetes Events sources that {{< param "PRODUCT_NAME" >}} collects logs from.

1. Port-forward the Grafana Pod to your local machine:

   1. Get the name of the Grafana Pod:

      ```shell
      export POD_NAME=$(kubectl get pods --namespace meta -l "app.kubernetes.io/name=grafana,app.kubernetes.io/instance=grafana" -o jsonpath="{.items[0].metadata.name}")
      ```

   1. Set up port-forwarding:

      ```shell
      kubectl --namespace meta port-forward $POD_NAME 3000
      ```

1. Port-forward the {{< param "PRODUCT_NAME" >}} Pod to your local machine:

   1. Get the name of the {{< param "PRODUCT_NAME" >}} Pod:

      ```shell
      export POD_NAME=$(kubectl get pods --namespace meta -l "app.kubernetes.io/name=alloy-logs,app.kubernetes.io/instance=k8s-alloy-logs" -o jsonpath="{.items[0].metadata.name}")
      ```

   1. Set up port-forwarding:

      ```shell
      kubectl --namespace meta port-forward $POD_NAME 12345
      ```

1. Deploy Grafana Tempo to the `prod` namespace.  
   Tempo generates logs for this example:

   ```shell
   helm install tempo grafana/tempo-distributed -n prod
   ```

## Monitor and visualize your data

Use Grafana to monitor your deployment's health and visualize your data.

### Monitor the health of your {{% param "PRODUCT_NAME" %}} deployment

To monitor the health of your {{< param "PRODUCT_NAME" >}} deployment, open your browser and go to [http://localhost:12345](http://localhost:12345).

For more information about the {{< param "PRODUCT_NAME" >}} UI, refer to [Debug Grafana Alloy](https://grafana.com/docs/alloy/latest/troubleshoot/debug/).

### Visualize your data

To use the Grafana Logs Drilldown, open your browser and go to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualize your metrics and logs, open your browser and go to [http://localhost:3000/dashboards](http://localhost:3000/dashboards).

## Understand the Kubernetes Monitoring Helm chart

The Kubernetes Monitoring Helm chart, `k8s-monitoring-helm`, collects, scrapes, and forwards Kubernetes telemetry data to a Grafana stack.  
This includes metrics, logs, traces, and continuous profiling data.

### `cluster`

Define the cluster name as `meta-monitoring-tutorial`.  
This is a static label attached to all logs collected by the Kubernetes Monitoring Helm chart.

```yaml
cluster:
  name: meta-monitoring-tutorial
```

### `destinations`

Define a destination named `loki` to forward logs to Loki.  
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
* `namespaces`: Specify the `meta` and `prod` namespaces to collect logs from.

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
Collecting node logs requires mounting `/var/log/journal`, which is out of scope for this example.

```yaml
nodeLogs:
  enabled: false
```

### `podLogs`

Enable the collection of Pod logs.

* `labelsToKeep`: Specify labels to keep when collecting logs.  
  This configuration removes `pod` from the labels to keep.  
* `structuredMetadata`: Specify structured metadata to collect.  
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

The Kubernetes Monitoring Helm chart deploys only what you need.  
In this case, the configuration deploys {{< param "PRODUCT_NAME" >}} with the capability to collect logs.  
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

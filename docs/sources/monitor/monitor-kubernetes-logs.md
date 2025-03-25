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

1. Change directory to your local clone of the Git repository.

   ```bash
   cd alloy-scenarios/k8s-logs
   ```

1. Use kind to create a local Kubernetes cluster.
   The `kind.yml` file provides the kind cluster configuration.

   ```shell
   kind create cluster --config kind.yml
   ```

1. Install the Grafana Helm repository.

   ```shell
   helm repo add grafana https://grafana.github.io/helm-charts
   ```

1. Create the `meta` and `prod` namespaces/

   ```shell
   kubectl create namespace meta && \
   kubectl create namespace prod
   ```

1. Install the Loki Helm chart to install Loki in the `meta` namespace.
   The `loki-values.yml` file contains the configuration for the Loki Helm chart.

   ```bash
   helm install --values loki-values.yml loki grafana/loki -n meta
   ```

   This Helm chart installs Loki in monolithic mode.
   For more information on Loki modes, see the [Loki documentation](https://grafana.com/docs/loki/latest/get-started/deployment-modes/).

1. Install the Grafana Helm chart to install Grafana in the `meta` namespace.
   The `grafana-values.yml` file contains the configuration for the Grafana Helm chart.

   ```shell
   helm install --values grafana-values.yml grafana grafana/grafana --namespace meta
   ```

   This Helm chart installs Grafana and sets the `datasources.datasources.yaml` field to the Loki data source configuration.

1. Install the Kubernetes Monitoring Helm chart to install {{< param "PRODUCT_NAME" >}} in the `meta` namespace.
   The `k8s-monitoring-values.yml` file contains the configuration for the Kubernetes monitoring Helm chart.

   ```shell
   helm install --values ./k8s-monitoring-values.yml k8s grafana/k8s-monitoring -n meta --create-namespace
   ```

   This Helm chart installs {{< param "PRODUCT_NAME" >}} and specifies the log Pod Logs and Kubernetes Events sources that {{< param "PRODUCT_NAME" >}} collects logs from.

1. Port-forward the Grafana Pod to your local machine.

   1. Get the name of the Grafana Pod.

      ```shell
      export POD_NAME=$(kubectl get pods --namespace meta -l "app.kubernetes.io/name=grafana,app.kubernetes.io/instance=grafana" -o jsonpath="{.items[0].metadata.name}")
      ```

   1. Use `kubectl` to set up the port-forwarding.

      ```shell
      kubectl --namespace meta port-forward $POD_NAME 3000
      ```

1. Log in to the Grafana UI.

   1. Open your browser and go to [http://localhost:3000](http://localhost:3000).
   1. Log in to Grafana with the default username `admin` and password `adminadminadmin`.

1. Port-forward the {{< param "PRODUCT_NAME" >}} Pod to your local machine.

   1. Get the name of the {{< param "PRODUCT_NAME" >}} Pod.

      ```shell
      export POD_NAME=$(kubectl get pods --namespace meta -l "app.kubernetes.io/name=alloy-logs,app.kubernetes.io/instance=k8s" -o jsonpath="{.items[0].metadata.name}")
      ```

   1. Use `kubectl` to set up the port-forwarding.

      ```shell
      kubectl --namespace meta port-forward $POD_NAME 12345
      ```

## Visualise your data

To explore metrics, open your browser and navigate to [http://localhost:3000/explore/metrics](http://localhost:3000/explore/metrics).

To use the Grafana Logs Drilldown, open your browser and navigate to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualise your metrics and logs, open your browser and navigate to [`http://localhost:3000/dashboards`](http://localhost:3000/dashboards).

## Add a demo prod app

The Kubernetes monitoring app collects logs from two namespaces: `meta` and `prod`.
To add a demo prod app, run the following command:

```shell
helm install tempo grafana/tempo-distributed -n prod
```

This installs the Tempo distributed tracing system in the `prod` namespace.

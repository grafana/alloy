---
canonical: https://grafana.com/docs/alloy/latest/tasks/migrate/from-operator/
description: Migrate from Grafana Agent Operator to Grafana Alloy
menuTitle: Migrate from Operator
title: Migrate from Grafana Agent Operator to Grafana Alloy
weight: 320
---

# Migrate from Grafana Agent Operator to {{% param "PRODUCT_NAME" %}}

With the release of {{< param "PRODUCT_NAME" >}}, Grafana Agent Operator is no longer the recommended way to deploy {{< param "PRODUCT_ROOT_NAME" >}} in Kubernetes.
Some of the Operator functionality has moved into {{< param "PRODUCT_NAME" >}} itself, and the Helm Chart has replaced the remaining functionality.

- The Monitor types (`PodMonitor`, `ServiceMonitor`, `Probe`, and `PodLogs`) are all supported natively by {{< param "PRODUCT_NAME" >}}.
  You are no longer required to use the Operator to consume those CRDs for dynamic monitoring in your cluster.
- The parts of the Operator that deploy the {{< param "PRODUCT_ROOT_NAME" >}} itself (`GrafanaAgent`, `MetricsInstance`, and `LogsInstance` CRDs) are deprecated.
  Operator users should use the {{< param "PRODUCT_ROOT_NAME" >}} [Helm Chart][] to deploy {{< param "PRODUCT_ROOT_NAME" >}} directly to your clusters.

This guide provides some steps to get started with {{< param "PRODUCT_NAME" >}} for users coming from Grafana Agent Operator.

## Deploy {{% param "PRODUCT_NAME" %}} with Helm

1. Create a `values.yaml` file, which contains options for deploying your {{< param "PRODUCT_ROOT_NAME" >}}.
   You can start with the [default values][] and customize as you see fit, or start with this snippet, which should be a good starting point for what the Operator does.

    ```yaml
    agent:
      mode: 'flow'
      configMap:
        create: true
      clustering:
        enabled: true
    controller:
      type: 'statefulset'
      replicas: 2
    crds:
      create: false
    ```

    This configuration deploys {{< param "PRODUCT_NAME" >}} as a `StatefulSet` using the built-in [clustering][] functionality to allow distributing scrapes across all {{< param "PRODUCT_ROOT_NAME" >}} Pods.

    This is one of many deployment possible modes. For example, you may want to use a `DaemonSet` to collect host-level logs or metrics.
    See the {{< param "PRODUCT_NAME" >}} [deployment guide][] for more details about different topologies.

1. Create a {{< param "PRODUCT_ROOT_NAME" >}} configuration file, `agent.river`.

    In the next step, you add to this configuration as you convert `MetricsInstances`. You can add any additional configuration to this file as you need.

1. Install the Grafana Helm repository:

    ```
    helm repo add grafana https://grafana.github.io/helm-charts
    helm repo update
    ```

1. Create a Helm release. You can name the release anything you like. The following command installs a release called `grafana-agent-metrics` in the `monitoring` namespace.

    ```shell
    helm upgrade grafana-agent-metrics grafana/grafana-agent -i -n monitoring -f values.yaml --set-file agent.configMap.content=agent.river
    ```

    This command uses the `--set-file` flag to pass the configuration file as a Helm value so that you can continue to edit it as a regular River file.

## Convert `MetricsIntances` to {{% param "PRODUCT_NAME" %}} components

A `MetricsInstance` resource primarily defines:

- The remote endpoints {{< param "PRODUCT_NAME" >}} should send metrics to.
- The `PodMonitor`, `ServiceMonitor`, and `Probe` resources this {{< param "PRODUCT_ROOT_NAME" >}} should discover.

You can use these functions in {{< param "PRODUCT_NAME" >}} with the `prometheus.remote_write`, `prometheus.operator.podmonitors`, `prometheus.operator.servicemonitors`, and `prometheus.operator.probes` components respectively.

The following River sample is equivalent to the `MetricsInstance` from the [operator guide][].

```river

// read the credentials secret for remote_write authorization
remote.kubernetes.secret "credentials" {
  namespace = "monitoring"
  name = "primary-credentials-metrics"
}

prometheus.remote_write "primary" {
    endpoint {
        url = "https://<PROMETHEUS_URL>/api/v1/push"
        basic_auth {
            username = nonsensitive(remote.kubernetes.secret.credentials.data["username"])
            password = remote.kubernetes.secret.credentials.data["password"]
        }
    }
}

prometheus.operator.podmonitors "primary" {
    forward_to = [prometheus.remote_write.primary.receiver]
    // leave out selector to find all podmonitors in the entire cluster
    selector {
        match_labels = {instance = "primary"}
    }
}

prometheus.operator.servicemonitors "primary" {
    forward_to = [prometheus.remote_write.primary.receiver]
    // leave out selector to find all servicemonitors in the entire cluster
    selector {
        match_labels = {instance = "primary"}
    }
}

```

Replace the following:

- _`<PROMETHEUS_URL>`_: The endpoint you want to send metrics to.

This configuration discovers all `PodMonitor`, `ServiceMonitor`, and `Probe` resources in your cluster that match the label selector `instance=primary`.
It then scrapes metrics from the targets and forward them to your remote write endpoint.

You may need to customize this configuration further if you use additional features in your `MetricsInstance` resources.
Refer to the documentation for the relevant components for additional information:

- [remote.kubernetes.secret][]
- [prometheus.remote_write][]
- [prometheus.operator.podmonitors][]
- [prometheus.operator.servicemonitors][]
- [prometheus.operator.probes][]
- [prometheus.scrape][]

## Collecting Logs

Our current recommendation is to create an additional DaemonSet deployment of {{< param "PRODUCT_ROOT_NAME" >}}s to scrape logs.

> We have components that can scrape pod logs directly from the Kubernetes API without needing a DaemonSet deployment. These are
> still considered experimental, but if you would like to try them, see the documentation for [loki.source.kubernetes][] and
> [loki.source.podlogs][].

These values are close to what the Operator currently deploys for logs:

```yaml
agent:
  mode: 'flow'
  configMap:
    create: true
  clustering:
    enabled: false
  controller:
    type: 'daemonset'
  mounts:
    # -- Mount /var/log from the host into the container for log collection.
    varlog: true
```

This command will install a release named `grafana-agent-logs` in the `monitoring` namespace:

```
helm upgrade grafana-agent-logs grafana/grafana-agent -i -n monitoring -f values-logs.yaml --set-file agent.configMap.content=agent-logs.river
```

This simple configuration will scrape logs for every pod on each node:

```river
// read the credentials secret for remote_write authorization
remote.kubernetes.secret "credentials" {
  namespace = "monitoring"
  name      = "primary-credentials-logs"
}

discovery.kubernetes "pods" {
  role = "pod"
  // limit to pods on this node to reduce the amount you need to filter
  selectors {
    role  = "pod"
    field = "spec.nodeName=" + env("<HOSTNAME>")
  }
}

discovery.relabel "pod_logs" {
  targets = discovery.kubernetes.pods.targets
  rule {
    source_labels = ["__meta_kubernetes_namespace"]
    target_label  = "namespace"
  }
  rule {
    source_labels = ["__meta_kubernetes_pod_name"]
    target_label  = "pod"
  }
  rule {
    source_labels = ["__meta_kubernetes_pod_container_name"]
    target_label  = "container"
  }
  rule {
    source_labels = ["__meta_kubernetes_namespace", "__meta_kubernetes_pod_name"]
    separator     = "/"
    target_label  = "job"
  }
  rule {
    source_labels = ["__meta_kubernetes_pod_uid", "__meta_kubernetes_pod_container_name"]
    separator     = "/"
    action        = "replace"
    replacement   = "/var/log/pods/*$1/*.log"
    target_label  = "__path__"
  }
  rule {
    action = "replace"
    source_labels = ["__meta_kubernetes_pod_container_id"]
    regex = "^(\\w+):\\/\\/.+$"
    replacement = "$1"
    target_label = "tmp_container_runtime"
  }
}

local.file_match "pod_logs" {
  path_targets = discovery.relabel.pod_logs.output
}

loki.source.file "pod_logs" {
  targets    = local.file_match.pod_logs.targets
  forward_to = [loki.process.pod_logs.receiver]
}

// basic processing to parse the container format. You can add additional processing stages
// to match your application logs.
loki.process "pod_logs" {
  stage.match {
    selector = "{tmp_container_runtime=\"containerd\"}"
    // the cri processing stage extracts the following k/v pairs: log, stream, time, flags
    stage.cri {}
    // Set the extract flags and stream values as labels
    stage.labels {
      values = {
        flags   = "",
        stream  = "",
      }
    }
  }

  // if the label tmp_container_runtime from above is docker parse using docker
  stage.match {
    selector = "{tmp_container_runtime=\"docker\"}"
    // the docker processing stage extracts the following k/v pairs: log, stream, time
    stage.docker {}

    // Set the extract stream value as a label
    stage.labels {
      values = {
        stream  = "",
      }
    }
  }

  // drop the temporary container runtime label as it is no longer needed
  stage.label_drop {
    values = ["tmp_container_runtime"]
  }

  forward_to = [loki.write.loki.receiver]
}

loki.write "loki" {
  endpoint {
    url = "https://<LOKI_URL>/loki/api/v1/push"
    basic_auth {
      username = nonsensitive(remote.kubernetes.secret.credentials.data["username"])
      password = remote.kubernetes.secret.credentials.data["password"]
    }
}
}
```

Replace the following:

- _`<LOKI_URL>`_: The endpoint of your Loki instance.

The logging subsystem is very powerful and has many options for processing logs. For further details, see the [component documentation][].

## Integrations

The `Integration` CRD isn't supported with {{< param "PRODUCT_NAME" >}}.
However, all static mode integrations have an equivalent component in the [`prometheus.exporter`][prometheus.exporter] namespace.
The [reference documentation][component documentation] should help convert those integrations to their {{< param "PRODUCT_NAME" >}} equivalent.

<!-- ToDo: Validate path -->
[default values]: https://github.com/grafana/alloy/blob/main/operations/helm/charts/alloy/values.yaml
[clustering]: ../../../concepts/clustering/
[deployment guide]: ../../../get-started/deploy-alloy
<!-- ToDo: check link syntax -->
[operator guide]: https://grafana.com/docs/agent/latest/operator/deploy-agent-operator-resources/#deploy-a-metricsinstance-resource

[Helm chart]: ../../../get-started/install/kubernetes/
[remote.kubernetes.secret]: ../../../reference/components/remote.kubernetes.secret/
[prometheus.remote_write]: ../../../reference/components/prometheus.remote_write/
[prometheus.operator.podmonitors]: ../../../reference/components/prometheus.operator.podmonitors/
[prometheus.operator.servicemonitors]: ../../../reference/components/prometheus.operator.servicemonitors/
[prometheus.operator.probes]: ../../../reference/components/prometheus.operator.probes/
[prometheus.scrape]: ../../../reference/components/prometheus.scrape/
[loki.source.kubernetes]: ../../../reference/components/loki.source.kubernetes/
[loki.source.podlogs]: ../../../reference/components/loki.source.podlogs/
[component documentation]: ../../../reference/components/
[prometheus.exporter]: ../../../reference/components/

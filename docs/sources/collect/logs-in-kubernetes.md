---
canonical: https://grafana.com/docs/alloy/latest/collect/logs-in-kubernetes/
aliases:
  - ../tasks/collect-logs-in-kubernetes/ # /docs/alloy/latest/tasks/collect-logs-in-kubernetes/
description: Learn how to collect logs on Kubernetes and forward them to Loki
menuTitle: Collect Kubernetes logs
title:  Collect Kubernetes logs and forward them to Loki
weight: 250
---

# Collect Kubernetes logs and forward them to Loki

You can configure {{< param "PRODUCT_NAME" >}} to collect logs and forward them to a [Loki][] database.

This topic describes how to:

* Configure logs delivery.
* Collect logs from Kubernetes Pods.

## Components used in this topic

* [discovery.kubernetes][]
* [discovery.relabel][]
* [local.file_match][]
* [loki.source.file][]
* [loki.source.kubernetes][]
* [loki.source.kubernetes_events][]
* [loki.process][]
* [loki.write][]

## Before you begin

* Ensure that you are familiar with logs labelling when working with Loki.
* Identify where you will write collected logs.
  You can write logs to Loki endpoints such as Grafana Loki, Grafana Cloud, or Grafana Enterprise Logs.
* Be familiar with the concept of [Components][] in {{< param "PRODUCT_NAME" >}}.

## Configure logs delivery

Before components can collect logs, you must have a component responsible for writing those logs somewhere.

The [loki.write][] component delivers logs to a Loki endpoint.
After a `loki.write` component is defined, you can use other {{< param "PRODUCT_NAME" >}} components to forward logs to it.

To configure a `loki.write` component for logs delivery, complete the following steps:

1. Add the following `loki.write` component to your configuration file.

   ```alloy
   loki.write "<LABEL>" {
     endpoint {
       url = "<LOKI_URL>"
     }
   }
   ```

   Replace the following:

   - _`<LABEL>`_: The label for the component, such as `default`.
     The label you use must be unique across all `loki.write` components in the same configuration file.
   - _`<LOKI_URL>`_ : The full URL of the Loki endpoint where logs will be sent, such as `https://logs-us-central1.grafana.net/loki/api/v1/push`.

1. If your endpoint requires basic authentication, paste the following inside the `endpoint` block.

   ```alloy
   basic_auth {
     username = "<USERNAME>"
     password = "<PASSWORD>"
   }
   ```

   Replace the following:

   - _`<USERNAME>`_: The basic authentication username.
   - _`<PASSWORD>`_: The basic authentication password or API key.

  1. If you have more than one endpoint to write logs to, repeat the `endpoint` block for additional endpoints.

The following simple example demonstrates configuring `loki.write` with multiple endpoints, mixed usage of basic authentication, 
and a `loki.source.file` component that collects logs from the filesystem on Alloy's own container.

```alloy
loki.write "default" {
  endpoint {
    url = "http://localhost:3100/loki/api/v1/push"
  }

  endpoint {
    url = "https://logs-us-central1.grafana.net/loki/api/v1/push"

    // Get basic authentication based on environment variables.
    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}

loki.source.file "example" {
  // Collect logs from the default listen address.
  targets = [
    {__path__ = "/tmp/foo.txt", "color" = "pink"},
    {__path__ = "/tmp/bar.txt", "color" = "blue"},
    {__path__ = "/tmp/baz.txt", "color" = "grey"},
  ]

  forward_to = [loki.write.default.receiver]
}
```

Replace the following:

   - _`<USERNAME>`_: The remote write username.
   - _`<PASSWORD>`_: The remote write password.

For more information on configuring logs delivery, refer to [loki.write][].

## Collect logs from Kubernetes

{{< param "PRODUCT_NAME" >}} can be configured to collect all kinds of logs from Kubernetes:

1. System logs
1. Pods logs
1. Kubernetes Events

Thanks to the component architecture, you can follow one or all of the next sections to get those logs. Once you have followed the [Configure logs delivery](#configure-logs-delivery) to ensure collected logs can be written somewhere, jump to the relevant sections.

### System logs

To get the system logs, you should use the following components:
1. [local.file_match][]: Discovers files on the local filesystem.
1. [loki.source.file][]: Reads log entries from files.
1. [loki.write][]: Send logs to the Loki endpoint. You should have configured it in the [Configure logs delivery](#configure-logs-delivery) section.

Here is an example using those stages.

```alloy
// local.file_match discovers files on the local filesystem using glob patterns and the doublestar library. It returns an array of file paths.
local.file_match "node_logs" {
  path_targets = [{
      // Monitor syslog to scrape node-logs
      __path__  = "/var/log/syslog",
      job       = "node/syslog",
      node_name = env("HOSTNAME"),
      cluster   = <CLUSTER_NAME>,
  }]
}

// loki.source.file reads log entries from files and forwards them to other loki.* components.
// You can specify multiple loki.source.file components by giving them different labels.
loki.source.file "node_logs" {
  targets    = local.file_match.node_logs.targets
  forward_to = [loki.write.<WRITE_COMPONENT_NAME>.receiver]
}
```

Replace the following values:

- _`<CLUSTER_NAME>`_: The label for this specific Kubernetes cluster, such as `production` or `us-east-1`.
- _`<WRITE_COMPONENT_NAME>`_: The name of your `loki.write` component, such as `default`.

### Pods logs

{{< admonition type="tip" >}}
You can get pods logs through the log files on each node. In this guide, you will get the logs through the Kubernetes API because it doesn't require system privileges for {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

The following components are needed:

1. [discovery.kubernetes][]: Discover pods information and list them for future components to use.
1. [discovery.relabel][]: Enforce relabelling strategies on the list of pods.
1. [loki.source.kubernetes][]: Tails logs from a list of Kubernetes pods targets.
1. [loki.process][]: Modify the logs before sending them to the next component.
1. [loki.write][]: Send logs to the Loki endpoint. You should have configured it in the [Configure logs delivery](#configure-logs-delivery) section.

Here is an example using those stages:

```alloy
// discovery.kubernetes allows you to find scrape targets from Kubernetes resources.
// It watches cluster state and ensures targets are continually synced with what is currently running in your cluster.
discovery.kubernetes "pod" {
  role = "pod"
}

// discovery.relabel rewrites the label set of the input targets by applying one or more relabeling rules.
// If no rules are defined, then the input targets are exported as-is.
discovery.relabel "pod_logs" {
  targets = discovery.kubernetes.pod.targets

  // Label creation - "namespace" field from "__meta_kubernetes_namespace"
  rule {
    source_labels = ["__meta_kubernetes_namespace"]
    action = "replace"
    target_label = "namespace"
  }

  // Label creation - "pod" field from "__meta_kubernetes_pod_name"
  rule {
    source_labels = ["__meta_kubernetes_pod_name"]
    action = "replace"
    target_label = "pod"
  }

  // Label creation - "container" field from "__meta_kubernetes_pod_container_name"
  rule {
    source_labels = ["__meta_kubernetes_pod_container_name"]
    action = "replace"
    target_label = "container"
  }

  // Label creation -  "app" field from "__meta_kubernetes_pod_label_app_kubernetes_io_name"
  rule {
    source_labels = ["__meta_kubernetes_pod_label_app_kubernetes_io_name"]
    action = "replace"
    target_label = "app"
  }

  // Label creation -  "job" field from "__meta_kubernetes_namespace" and "__meta_kubernetes_pod_container_name"
  // Concatenate values __meta_kubernetes_namespace/__meta_kubernetes_pod_container_name
  rule {
    source_labels = ["__meta_kubernetes_namespace", "__meta_kubernetes_pod_container_name"]
    action = "replace"
    target_label = "job"
    separator = "/"
    replacement = "$1"
  }

  // Label creation - "container" field from "__meta_kubernetes_pod_uid" and "__meta_kubernetes_pod_container_name"
  // Concatenate values __meta_kubernetes_pod_uid/__meta_kubernetes_pod_container_name.log
  rule {
    source_labels = ["__meta_kubernetes_pod_uid", "__meta_kubernetes_pod_container_name"]
    action = "replace"
    target_label = "__path__"
    separator = "/"
    replacement = "/var/log/pods/*$1/*.log"
  }

  // Label creation -  "container_runtime" field from "__meta_kubernetes_pod_container_id"
  rule {
    source_labels = ["__meta_kubernetes_pod_container_id"]
    action = "replace"
    target_label = "container_runtime"
    regex = "^(\\S+):\\/\\/.+$"
    replacement = "$1"
  }
}

// loki.source.kubernetes tails logs from Kubernetes containers using the Kubernetes API.
loki.source.kubernetes "pod_logs" {
  targets    = discovery.relabel.pod_logs.output
  forward_to = [loki.process.pod_logs.receiver]
}

// loki.process receives log entries from other Loki components, applies one or more processing stages,
// and forwards the results to the list of receivers in the component’s arguments.
loki.process "pod_logs" {
  stage.static_labels {
      values = {
        cluster = "<CLUSTER_NAME>",
      }
  }

  forward_to = [loki.write.<WRITE_COMPONENT_NAME>.receiver]
}
```

Replace the following values:

- _`<CLUSTER_NAME>`_: The label for this specific Kubernetes cluster, such as `production` or `us-east-1`.
- _`<WRITE_COMPONENT_NAME>`_: The name of your `loki.write` component, such as `default`.

### Kubernetes Cluster Events

The following components are needed:

1. [loki.source.kubernetes_events][]: Tails events from Kubernetes API.
1. [loki.process][]: Modify the logs before sending them to the next component.
1. [loki.write][]: Send logs to the Loki endpoint. You should have configured it in the [Configure logs delivery](#configure-logs-delivery) section.

Here is an example using those stages:

```alloy
// loki.source.kubernetes_events tails events from the Kubernetes API and converts them
// into log lines to forward to other Loki components.
loki.source.kubernetes_events "cluster_events" {
  job_name   = "integrations/kubernetes/eventhandler"
  log_format = "logfmt"
  forward_to = [
    loki.process.cluster_events.receiver,
  ]
}

// loki.process receives log entries from other loki components, applies one or more processing stages,
// and forwards the results to the list of receivers in the component’s arguments.
loki.process "cluster_events" {
  forward_to = [loki.write.<WRITE_COMPONENT_NAME>.receiver]

  stage.static_labels {
    values = {
      cluster = "<CLUSTER_NAME>",
    }
  }

  stage.labels {
    values = {
      kubernetes_cluster_events = "job",
    }
  }
}
```

Replace the following values:

- _`<CLUSTER_NAME>`_: The label for this specific Kubernetes cluster, such as `production` or `us-east-1`.
- _`<WRITE_COMPONENT_NAME>`_: The name of your `loki.write` component, such as `default`.

[Loki]: https://grafana.com/oss/loki/
[Field Selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
[Labels and Selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#set-based-requirement
[Field Selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
[Labels and Selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#set-based-requirement
[Configure logs delivery]: #configure-logs-delivery
[discovery.kubernetes]: ../../reference/components/discovery/discovery.kubernetes/
[loki.write]: ../../reference/components/loki/loki.write/
[local.file_match]: ../../reference/components/local/local.file_match/
[loki.source.file]: ../../reference/components/loki/loki.source.file/
[discovery.relabel]: ../../reference/components/discovery/discovery.relabel/
[loki.source.kubernetes]: ../../reference/components/loki/loki.source.kubernetes/
[loki.process]: ../../reference/components/loki/loki.process/
[loki.source.kubernetes_events]: ../../reference/components/loki/loki.source.kubernetes_events/
[Components]: ../../get-started/components/
[Objects]: ../../concepts/configuration-syntax/expressions/types_and_values/#objects
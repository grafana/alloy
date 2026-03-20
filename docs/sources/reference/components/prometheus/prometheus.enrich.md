---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.enrich/
description: The prometheus.enrich component enriches metrics with labels from service discovery
labels:
  stage: experimental
  products:
    - oss
title: prometheus.enrich
---

# `prometheus.enrich`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `prometheus.enrich` component enriches metrics with additional labels from service discovery targets.
It matches labels from incoming metrics against labels from discovered targets, and copies specified labels from the
matched target to the metric sample. If no match occurs, the metrics are passed through unchanged.

The component supports two matching modes:

- **Single-label matching (legacy):** Match on a single label pair using `target_match_label` and optionally `metrics_match_label`.
- **Multi-label matching:** Match on multiple labels simultaneously using `target_to_metric_match`. The map keys are target label names and the values are the corresponding metric label names. All labels in the map must match for enrichment to occur.

## Usage

### Single-label matching

```alloy
prometheus.enrich "<LABEL>" {
  targets = <DISCOVERY_COMPONENT>.targets

  target_match_label = "<LABEL>"

  forward_to = [<RECEIVER_LIST>]
}
```

### Multi-label matching

```alloy
prometheus.enrich "<LABEL>" {
  targets = <DISCOVERY_COMPONENT>.targets

  target_to_metric_match = {
    "<TARGET_LABEL_1>" = "<METRIC_LABEL_1>",
    "<TARGET_LABEL_2>" = "<METRIC_LABEL_2>",
  }

  forward_to = [<RECEIVER_LIST>]
}
```

## Arguments

You can use the following arguments with `prometheus.enrich`:

| Name                                | Type                           | Description                                                                                        | Default | Required |
|-------------------------------------|--------------------------------|----------------------------------------------------------------------------------------------------|---------|----------|
| `forward_to`                        | `list(MetricsReceiver)`        | Where the metrics should be forwarded to, after enrichment.                                        |         | yes      |
| `targets`                           | `list(map(string))`            | List of targets from a discovery component.                                                        |         | yes      |
| `target_to_metric_match`            | `map(string)`                  | Map of target label name to metric label name. All entries must match for enrichment.              |         | no       |
| `target_match_label`                | `string`                       | The label from discovered targets to match against, for example, `"__inventory_consul_service"`.   |         | no       |
| `metrics_match_label`               | `string`                       | The label from incoming metrics to match against discovered targets, for example `"service_name"`. |         | no       |
| `labels_to_copy`                    | `list(string)`                 | List of labels to copy from discovered targets to metrics. If empty, all labels are copied.        |         | no       |



You must specify exactly one of the following:

- `target_match_label` (with optional `metrics_match_label`) for single-label matching
- `target_to_metric_match` for multi-label matching.

These two modes are mutually exclusive. If you don't set the `metrics_match_label`, it defaults to the value of `target_match_label`.

## Blocks

The `prometheus.enrich` component doesn't support any blocks. You can configure this component with arguments.

## Exports

The following values are exported:

| Name       | Type              | Description                                               |
|------------|-------------------|-----------------------------------------------------------|
| `receiver` | `MetricsReceiver` | The input receiver where samples are sent to be enriched. |

## Debug information

`prometheus.enrich` doesn't expose any component-specific debug information.

## Debug metrics

* `prometheus_fanout_latency` (histogram): Write latency for sending to direct and indirect components.
* `prometheus_forwarded_samples_total` (counter): Total number of samples sent to downstream components.
* `prometheus_target_cache_size` (gauge): Total number of cached target entries.

## Examples

### Enrich metrics from `prometheus.scrape`

The following example shows how the `prometheus.enrich` enriches incoming metrics from
`prometheus.scrape.default`, using HTTP discovery, and forwards the results to
`prometheus.remote_write.default` component:

```alloy
discovery.http "default" {
    url = "http://network-inventory.example.com/prometheus_sd"
}

prometheus.scrape "default" {
  targets = [
    {"__address__" = "example-app:9001"},
  ]

  forward_to = [prometheus.enrich.default.receiver]
}

prometheus.enrich "default" {
	targets = discovery.http.default.targets

	target_match_label = "hostname"

	forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}
```

### Enrich metrics from `prometheus.receive_http`

The following example enriches cadvisor metrics with Kubernetes Pod metadata, matching on namespace, Pod, and container labels simultaneously.

```alloy
discovery.file "network_devices" {
   files = ["/etc/alloy/devices.json"]
}

prometheus.receive_http "default" {
  http {
    listen_address = "0.0.0.0"
    listen_port = 9999
  }

  forward_to = [prometheus.enrich.default.receiver]
}

prometheus.enrich "default" {
    targets = discovery.file.network_devices.targets

    target_match_label = "hostname"

    forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}
```

### Multi-label matching with Kubernetes metadata

The following example enriches cadvisor metrics with Kubernetes pod metadata,
matching on namespace, pod, and container labels simultaneously.

```alloy
discovery.kubernetes "pods" {
  role = "pod"
}

prometheus.scrape "cadvisor" {
  targets = [
    {"__address__" = "localhost:10250", "__metrics_path__" = "/metrics/cadvisor"},
  ]
  scheme = "https"

  forward_to = [prometheus.enrich.k8s_meta.receiver]
}

prometheus.enrich "k8s_meta" {
    targets = discovery.kubernetes.pods.targets

    target_to_metric_match = {
        "__meta_kubernetes_namespace"          = "namespace",
        "__meta_kubernetes_pod_name"           = "pod",
        "__meta_kubernetes_pod_container_name" = "container",
    }

    labels_to_copy = ["__meta_kubernetes_pod_node_name", "__meta_kubernetes_pod_label_app"]

    forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.enrich` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)

`prometheus.enrich` has exports that can be consumed by the following components:

- Components that consume [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

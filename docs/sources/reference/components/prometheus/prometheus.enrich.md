---
aliases:
- /docs/alloy/latest/reference/components/prometheus/prometheus.enrich/
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.enrich/
title: prometheus.enrich
labels:
  stage: experimental
  products:
    - oss
description: The prometheus.enrich component enriches logs with labels from service discovery.
---

# `prometheus.enrich`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `prometheus.enrich` component enriches metrics with additional labels from service discovery targets.
It matches a label from incoming metrics against a label from discovered targets, and copies specified labels from the matched target to the metric sample.

## Usage

```alloy
prometheus.enrich "<LABEL>" {
  // List of targets from a discovery component
  targets = <DISCOVERY_COMPONENT>.targets
  
  // Which label from discovered targets to match against
  target_match_label = "<LABEL>"
  
  // Which label from incoming metrics to match against
  metrics_match_label = "<LABEL>"
  
  // List of labels to copy from discovered targets to metrics
  labels_to_copy = ["<LABEL>", ...]
  
  // Where to send enriched metrics
  forward_to = [<RECEIVER_LIST>]
}
```

## Arguments

You can use the following arguments with `prometheus.enrich`:

| Name                  | Type                           | Description                                                                                        | Default                | Required |
|-----------------------|--------------------------------|----------------------------------------------------------------------------------------------------| ---------------------- | -------- |
| `forward_to`          | `[]prometheus.MetricsReceiver` | Where the metrics should be forwarded to, after enrichment.                                        |                        | yes      |
| `target_match_label`  | `string`                       | The label from discovered targets to match against, for example, `"__inventory_consul_service"`.   |                        | yes      |
| `targets`             | `[]discovery.Target`           | List of targets from a discovery component.                                                        |                        | yes      |
| `labels_to_copy`      | `[]string`                     | List of labels to copy from discovered targets to metrics. If empty, all labels will be copied.    |                        | no       |
| `metrics_match_label` | `string`                       | The label from incoming metrics to match against discovered targets, for example `"service_name"`. |                        | no       |

If not provided, the `metrics_match_label` attribute will default to the value of `target_match_label`.

## Blocks

The `prometheus.enrich` component doesn't support any blocks. You can configure this component with arguments.

## Exports

The following values are exported:

| Name       | Type                         | Description                                               |
| ---------- |------------------------------|-----------------------------------------------------------|
| `receiver` | `prometheus.MetricsReceiver` | The input receiver where samples are sent to be enriched. |

## Example

```alloy
// Configure HTTP discovery
discovery.http "default" {
    url = "http://network-inventory.example.com/prometheus_sd"
}

prometheus.scrape "scrape_prom_metrics" {
  targets = [
    {"__address__" = "example-app:9001"},
  ]

  forward_to = [prometheus.enrich.enrich_prom_metrics.receiver]
}

prometheus.enrich "enrich_prom_metrics" {
	targets = discovery.http.default.targets

	target_match_label = "hostname"

	forward_to = [prometheus.remote_write.enrich_prom_metrics.receiver]
}

prometheus.remote_write "enrich_prom_metrics" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}
```

## Component Behavior

The component matches metric samples to discovered targets and enriches them with additional labels.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.enrich` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

`prometheus.enrich` has exports that can be consumed by the following components:

- Components that consume [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.mapping/
description: Learn about prometheus.mapping
title: prometheus.mapping
---
<span class="badge docs-labels__stage docs-labels__item">Public preview</span>

# prometheus.mapping

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

Prometheus metrics follow the [OpenMetrics](https://openmetrics.io/) format.
Each time series is uniquely identified by its metric name, plus optional
key-value pairs called labels. Each sample represents a datapoint in the
time series and contains a value and an optional timestamp.

```text
<metric name>{<label_1>=<label_val_1>, <label_2>=<label_val_2> ...} <value> [timestamp]
```

The `prometheus.mapping` component create new labels on each metric passed
along to the exported receiver by applying a mapping table to a label value.

The most common use of `prometheus.mapping` is to create new labels with a high
cardinality source label value (>1k) when a large set of regular expressions are
inefficient.

You can specify multiple `prometheus.mapping` components by giving them
different labels.

## Usage

```alloy
prometheus.mapping "LABEL" {
  forward_to = RECEIVER_LIST

  source_label = "labelA"

  mapping = {
    "from" = {"labelB" = "to"},
    ...
  }
}
```

## Arguments

The following arguments are supported:

Name           | Type                      | Description                                                         | Default | Required
---------------|---------------------------|---------------------------------------------------------------------|---------|---------
`forward_to`   | `list(MetricsReceiver)`   | The receiver the metrics are forwarded to after they are relabeled. |         | yes
`source_label` | `string`                  | Name of the source label to use for mapping.                        |         | yes
`mapping`      | `map(string,map(string))` | Mapping from source label value to target labels name/value.        |         | yes

## Exported fields

The following fields are exported and can be referenced by other components:

Name       | Type              | Description
-----------|-------------------|-----------------------------------------------------------
`receiver` | `MetricsReceiver` | The input receiver where samples are sent to be relabeled.

## Component health

`prometheus.mapping` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields are kept at their last healthy values.

## Debug information

`prometheus.mapping` doesn't expose any component-specific debug information.

## Debug metrics

* `prometheus_mapping_metrics_processed` (counter): Total number of metrics processed.
* `prometheus_mapping_metrics_written` (counter): Total number of metrics written.

## Example

Create an instance of  a `prometheus.mapping` component.

```alloy
prometheus.mapping "keep_backend_only" {
  forward_to = [prometheus.remote_write.onprem.receiver]

  source_label = "app"

  mapping = {
    "frontend" = {"team" = "teamA"}
    "backend"  = {"team" = "teamB"}
    "database" = {"team" = "teamC"}
  }
}
```

Use the following metrics.

```text
metric_a{__address__ = "localhost", instance = "development", app = "frontend"} 10
metric_a{__address__ = "localhost", instance = "development", app = "backend"}  2
metric_a{__address__ = "cluster_a", instance = "production",  app = "frontend"} 7
metric_a{__address__ = "cluster_a", instance = "production",  app = "backend"}  9
metric_a{__address__ = "cluster_b", instance = "production",  app = "database"} 4
```

After applying the mapping a new `team` label is created based on mapping table and `app` label value.

```text
metric_a{team = "teamA", __address__ = "localhost", instance = "development", app = "frontend"} 10
metric_a{team = "teamB", __address__ = "localhost", instance = "development", app = "backend"}  2
metric_a{team = "teamA",  __address__ = "cluster_a", instance = "production",  app = "frontend"} 7
metric_a{team = "teamA",  __address__ = "cluster_a", instance = "production",  app = "backend"}  9
metric_a{team = "teamC",  __address__ = "cluster_a", instance = "production",  app = "database"} 4
```

The resulting metrics are propagated to each receiver defined in the `forward_to` argument.
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.mapping` can accept arguments from the following components:

- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)

`prometheus.mapping` has exports that can be consumed by the following components:

- Components that consume [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

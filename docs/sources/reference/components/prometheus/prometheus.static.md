---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.static/
description: The prometheus.static component sends a fixed set of user-defined metrics
labels:
  stage: experimental
  products:
    - oss
title: prometheus.static
---

# `prometheus.static`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `prometheus.static` component sends a fixed set of user-defined metrics to downstream Prometheus-compatible components.
Use it to report static information such as build metadata, environment details, or feature flags as metrics.

Each metric defined in a `metric` block is emitted with the value you configure and the labels you attach.
The metrics are re-emitted on a regular interval so they remain fresh in downstream storage.

## Usage

```alloy
prometheus.static "<LABEL>" {
  metric {
    name = "<METRIC_NAME>"
  }

  forward_to = [<RECEIVER_LIST>]
}
```

## Arguments

You can use the following arguments with `prometheus.static`:

| Name              | Type                    | Description                                                          | Default | Required |
|-------------------|-------------------------|---------------------------------------------------------------------|---------|----------|
| `forward_to`      | `list(MetricsReceiver)` | Where the metrics are forwarded to.                                 |         | yes      |
| `prefix`          | `string`                | Prefix added to every metric name, joined with an underscore.  | `""`    | no       |
| `scrape_interval` | `duration`              | The interval at which metrics are sent to `forward_to`.      | `"1m"`  | no       |

When `prefix` is set, the emitted metric name is `<prefix>_<name>`.
For example, a `prefix` of `"self_report"` and a metric `name` of `"build_info"` produces the metric `self_report_build_info`.

`prometheus.static` doesn't scrape any target.
The `scrape_interval` argument is the interval at which the configured metrics are emitted to the `forward_to` receivers.
The metrics are re-emitted on each interval so they remain fresh in downstream storage.

## Blocks

You can use the following blocks with `prometheus.static`:

| Block                | Description                                     | Required |
|----------------------|-------------------------------------------------|----------|
| [`metric`][metric]   | Defines a single metric to emit.                | yes      |
| [`labels`][labels]   | Labels attached to every emitted metric.        | no       |

You must define at least one `metric` block.

[metric]: #metric
[labels]: #labels

### `metric`

The `metric` block defines a single metric to emit. You can define multiple `metric` blocks.

| Name    | Type     | Description                                     | Default     | Required |
|---------|----------|-------------------------------------------------|-------------|----------|
| `name`  | `string` | The metric name, before `prefix` is applied.    |             | yes      |
| `value` | `number` | The value emitted for the metric.               | `1`         | no       |
| `type`  | `string` | The metric type reported as metadata.           | `"unknown"` | no       |
| `help`  | `string` | A description reported as metadata.             | `""`        | no       |

The default `value` of `1` matches the convention for info-style metrics such as `build_info`.

The `type` attribute controls the metric type metadata (`# TYPE`) forwarded to downstream components such as `prometheus.remote_write`.
It doesn't change the sample value.
The supported types are `gauge`, `counter`, `info`, and `unknown`.
Types that require multiple values, such as `histogram` and `summary`, aren't supported.

The `metric` block supports a nested [`labels`](#labels) block for labels that apply only to that metric.
When a metric-level label and a component-level label share the same name, the metric-level label takes precedence.

### `labels`

The `labels` block is a set of key-value pairs attached to metrics.
Unlike most blocks, the attribute names inside a `labels` block are the label names, and the values are the label values:

```alloy
labels {
  version = "1.2.3"
  color  = "green"
}
```

When you place a `labels` block at the top level of `prometheus.static`, those labels apply to every metric.
When you place a `labels` block inside a `metric` block, those labels apply only to that metric.

## Exports

`prometheus.static` doesn't export any fields.

## Debug information

`prometheus.static` doesn't expose any component-specific debug information.

## Debug metrics

* `alloy_prometheus_static_metrics_emitted_total` (counter): Total number of static metrics sent to downstream components.
* `prometheus_fanout_latency` (histogram): Write latency for sending to direct and indirect components.
* `prometheus_forwarded_samples_total` (counter): Total number of samples sent to downstream components.

## Examples

### Send a single metric

The following example emits a single `heartbeat` metric with a value of `1` and forwards it to `prometheus.remote_write`:

```alloy
prometheus.static "heartbeat" {
  metric {
    name = "heartbeat"
  }

  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}
```

This produces the following series:

```text
heartbeat 1
```

### Send multiple metrics with a prefix and shared labels

The following example uses a `prefix`, defines two metrics with their own labels, and attaches a `region` label to every metric:

```alloy
prometheus.static "self_report" {
  prefix = "self_report"

  metric {
    name = "build_info"
    labels {
      version = "1.2.3"
    }
  }

  metric {
    name  = "feature_flag_enabled"
    value = 0
    labels {
      flag = "new_ui"
    }
  }

  labels {
    region = "us-east-1"
  }

  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}
```

This produces the following series:

```text
self_report_build_info{region="us-east-1", version="1.2.3"} 1
self_report_feature_flag_enabled{flag="new_ui", region="us-east-1"} 0
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.static` can accept arguments from the following components:

- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

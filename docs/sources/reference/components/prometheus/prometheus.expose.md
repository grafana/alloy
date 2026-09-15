---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.expose/
description: Learn about prometheus.expose
labels:
  stage: experimental
  products:
    - oss
title: prometheus.expose
---

# `prometheus.expose`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `prometheus.expose` component receives Prometheus metrics and re-exposes them on Alloy's own `/metrics` endpoint.
This is useful when you want scraped or forwarded metrics to be visible as part of Alloy's self-monitoring metrics.

## Usage

```alloy
prometheus.expose "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.expose`:

| Name        | Type                | Description                                      | Default | Required |
| ----------- | ------------------- | ------------------------------------------------ | ------- | -------- |
| `namespace` | `string`            | Prefix added to all metric names as `namespace_`.| `""`    | no       |
| `subsystem` | `string`            | Prefix added to all metric names as `subsystem_`.| `""`    | no       |
| `labels`    | `map(string)`       | Extra labels added to every metric.              | `{}`    | no       |

When both `namespace` and `subsystem` are set, the resulting metric name is `namespace_subsystem_<original_name>`.

Global labels defined in `labels` are only added when the incoming metric does not already carry a label with the same name.

## Blocks

The `prometheus.expose` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type              | Description                                               |
| ---------- | ----------------- | --------------------------------------------------------- |
| `receiver` | `MetricsReceiver` | A value that other components can use to send metrics to. |

## Component health

`prometheus.expose` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.expose` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.expose` doesn't expose any component-specific debug metrics.

## Example

Expose scraped application metrics on Alloy's own `/metrics` endpoint:

```alloy
prometheus.scrape "app" {
  targets = [{"__address__" = "localhost:8080"}]
  forward_to = [prometheus.expose.local.receiver]
}

prometheus.expose "local" {}
```

### With namespace and extra labels

```alloy
prometheus.scrape "app" {
  targets = [{"__address__" = "localhost:8080"}]
  forward_to = [prometheus.expose.local.receiver]
}

prometheus.expose "local" {
  namespace = "myapp"
  labels    = { env = "production" }
}
```

In this example all metrics received will be renamed from `<name>` to `myapp_<name>` and will carry an additional `env="production"` label.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.expose` can accept metrics from the following components:

- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

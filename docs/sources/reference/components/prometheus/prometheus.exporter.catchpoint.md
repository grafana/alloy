---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.catchpoint/
aliases:
  - ../prometheus.exporter.catchpoint/ # /docs/alloy/latest/reference/components/prometheus.exporter.catchpoint/
description: Learn about prometheus.exporter.catchpoint
title: prometheus.exporter.catchpoint
---

# prometheus.exporter.catchpoint

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `prometheus.exporter.catchpoint` component uses the [catchpoint_exporter](https://github.com/grafana/catchpoint-prometheus-exporter) for collecting statistics from a Catchpoint account.

## Usage

```alloy
prometheus.exporter.catchpoint "<LABEL>" {
    port              = PORT
    verbose_logging   = <VERBOSE_LOGGING>
    webhook_path      = <WEBHOOK_PATH>
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
Omitted fields take their default values.

| Name              | Type     | Description                                                                     | Default                 | Required |
| ----------------- | -------- | ------------------------------------------------------------------------------- | ----------------------- | -------- |
| `port`            | `string` | Sets the port on which the exporter will run.                                   | `"9090"`                | no       |
| `verbose_logging` | `bool`   | Enables verbose logging to provide more detailed output for debugging purposes. | `false`                 | no       |
| `webhook_path`    | `string` | Defines the path where the exporter will receive webhook data from Catchpoint   | `"/catchpoint-webhook"` | no       |

## Blocks

The `prometheus.exporter.catchpoint` component does not support any blocks, and is configured
fully through arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.catchpoint` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.catchpoint` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.catchpoint` does not expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.catchpoint`:

```alloy
prometheus.exporter.catchpoint "example" {
  port             = "9090"
  verbose_logging  = false
  webhook_path     = "/catchpoint-webhook"
}

// Configure a prometheus.scrape component to collect catchpoint metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.catchpoint.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = <PROMETHEUS_REMOTE_WRITE_URL>

    basic_auth {
      username = <USERNAME>
      password = <PASSWORD>
    }
  }
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the remote_write API.
- _`<PASSWORD>`_: The password to use for authentication to the remote_write API.

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.catchpoint` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

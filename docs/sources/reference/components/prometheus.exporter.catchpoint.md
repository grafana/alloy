---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus.exporter.catchpoint/
description: Learn about prometheus.exporter.catchpoint
title: prometheus.exporter.catchpoint
---

# prometheus.exporter.catchpoint

The `prometheus.exporter.catchpoint` component embeds
[catchpoint_exporter](https://github.com/grafana/catchpoint-prometheus-exporter) for collecting statistics from a Catchpoint account.

## Usage

```alloy
prometheus.exporter.catchpoint "LABEL" {
    port              = PORT
    verbosity_logging = VERBOSITY_LOGGING
    webhook_path      = WEBHOOK_PATH
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
Omitted fields take their default values.

| Name                | Type     | Description                                           | Default                 | Required |
| ------------------- | -------- | ----------------------------------------------------- | ----------------------- | -------- |
| `port`              | `string` | The account to collect metrics for.                   | `"9090"`                | yes      |
| `verbosity_logging` | `bool`   | The username for the user used when querying metrics. | `false`                 | yes      |
| `webhook_path`      | `string` | The password for the user used when querying metrics. | `"/catchpoint-webhook"` | yes      |

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
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

Replace the following:

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the remote_write API.
- `PASSWORD`: The password to use for authentication to the remote_write API.

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.catchpoint` has exports that can be consumed by the following components:

- Components that consume [Targets](../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

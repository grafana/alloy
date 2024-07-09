---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.self/
aliases:
  - ../prometheus.exporter.self/ # /docs/alloy/latest/reference/components/prometheus.exporter.self/
description: Learn about prometheus.exporter.self
title: prometheus.exporter.self
---

# prometheus.exporter.self

The `prometheus.exporter.self` component collects and exposes metrics about {{< param "PRODUCT_NAME" >}} itself.

## Usage

```alloy
prometheus.exporter.self "alloy" {
}
```

## Arguments

`prometheus.exporter.self` accepts no arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.self` is only reported as unhealthy if given
an invalid configuration.

## Debug information

`prometheus.exporter.self` doesn't expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.self` doesn't expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.self`:

```alloy
prometheus.exporter.self "example" {}

// Configure a prometheus.scrape component to collect Alloy metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.self.example.targets
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

`prometheus.exporter.self` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

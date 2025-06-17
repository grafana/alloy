---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.self/
aliases:
  - ../prometheus.exporter.self/ # /docs/alloy/latest/reference/components/prometheus.exporter.self/
description: Learn about prometheus.exporter.self
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.self
---

# `prometheus.exporter.self`

The `prometheus.exporter.self` component collects and exposes metrics about {{< param "PRODUCT_NAME" >}} itself.

## Usage

```alloy
prometheus.exporter.self "<LABEL>" {
}
```

{{< admonition type="note" >}}
When using `prometheus.exporter.self` within a [cluster](../../../../get-started/clustering/) of Alloy instances,
it is recommended to use a dedicated `prometheus.scrape` component that is used to scrape `prometheus.exporter.self`
and does not have clustering enabled.

This is because clustering uses consistent hashing to distribute targets across instances,
and the instance label (which defaults to the hostname) must be same across all cluster instances.
{{< /admonition >}}

## Arguments

The `prometheus.exporter.self` component doesn't support any arguments.

## Blocks

The `prometheus.exporter.self` component doesn't support any blocks.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.self` is only reported as unhealthy if given an invalid configuration.

## Debug information

`prometheus.exporter.self` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.self` doesn't expose any component-specific debug metrics.

## Example

The following example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.self`:

```alloy
prometheus.exporter.self "example" {}

// Configure a prometheus.scrape component to collect Alloy metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.self.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

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

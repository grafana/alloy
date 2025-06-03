---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.squid/
aliases:
  - ../prometheus.exporter.squid/ # /docs/alloy/latest/reference/components/prometheus.exporter.squid/
description: Learn about prometheus.exporter.squid
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.squid
---

# `prometheus.exporter.squid`

The `prometheus.exporter.squid` component embeds the [`squid_exporter`](https://github.com/boynux/squid-exporter) for collecting metrics from a squid instance.

## Usage

```alloy
prometheus.exporter.squid "<LABEL>" {
    address = "<SQUID_ADDRESS>"
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.squid`:

| Name       | Type     | Description                                           | Default | Required |
| ---------- | -------- | ----------------------------------------------------- | ------- | -------- |
| `address`  | `string` | The squid address to collect metrics from.            |         | yes      |
| `password` | `secret` | The password for the user used when querying metrics. |         | no       |
| `username` | `string` | The username for the user used when querying metrics. |         | no       |

## Blocks

The `prometheus.exporter.squid` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.squid` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.squid` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.squid` doesn't expose any component-specific debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.squid`:

```alloy
prometheus.exporter.squid "example" {
  address = "localhost:3128"
}

// Configure a prometheus.scrape component to collect squid metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.squid.example.targets
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

`prometheus.exporter.squid` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

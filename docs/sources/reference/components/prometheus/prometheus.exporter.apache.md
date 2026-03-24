---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.apache/
aliases:
  - ../prometheus.exporter.apache/ # /docs/alloy/latest/reference/components/prometheus.exporter.apache/
description: Learn about prometheus.exporter.apache
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.apache
---

# `prometheus.exporter.apache`

The `prometheus.exporter.apache` component embeds [`apache_exporter`](https://github.com/Lusitaniae/apache_exporter) for collecting `mod_status` statistics from an Apache server.

## Usage

```alloy
prometheus.exporter.apache "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.apache`.

| Name            | Type     | Description                               | Default                                 | Required |
| --------------- | -------- | ----------------------------------------- | --------------------------------------- | -------- |
| `host_override` | `string` | Override for HTTP Host header.            |                                         | no       |
| `insecure`      | `bool`   | Ignore server certificate if using HTTPS. | `false`                                 | no       |
| `scrape_uri`    | `string` | URI to Apache stub status page.           | `"http://localhost/server-status?auto"` | no       |

## Blocks

The `prometheus.exporter.apache` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.apache` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.apache` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.apache` doesn't expose any component-specific debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.apache`:

```alloy
prometheus.exporter.apache "example" {
  scrape_uri = "http://web.example.com/server-status?auto"
}

// Configure a prometheus.scrape component to collect apache metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.apache.example.targets
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

`prometheus.exporter.apache` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

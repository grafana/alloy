---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.static/
aliases:
  - ../prometheus.exporter.static/ # /docs/alloy/latest/reference/components/prometheus.exporter.static/
description: Learn about prometheus.exporter.static
labels:
  stage: experimental
  products:
    - oss
title: prometheus.exporter.static
---

# `prometheus.exporter.static`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`prometheus.exporter.static` loads metrics from text specified in [`Prometheus exposition format`](https://prometheus.io/docs/instrumenting/exposition_formats/) and expose them for scraping.

You can specify multiple `prometheus.exporter.static` components by giving them different labels.

## Usage

```alloy
prometheus.exporter.static "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.static`:

| Name   | Type     | Description                           | Default | Required |
| ------ | -------- | ------------------------------------- | ------- | -------- |
| `text` | `string` | Text in prometheus exposition format. | `""`    | no       |

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.static` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.static` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.static` doesn't expose any component-specific debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.static`:

```alloy
prometheus.exporter.static "demo" {
    text = `
# HELP http_requests_total The total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="post",code="200"} 1027
http_requests_total{method="post",code="400"}    3

# HELP http_request_duration_seconds A histogram of the request duration.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.05"} 24054
http_request_duration_seconds_bucket{le="0.1"} 33444
http_request_duration_seconds_bucket{le="0.2"} 100392
http_request_duration_seconds_bucket{le="0.5"} 129389
http_request_duration_seconds_bucket{le="1"} 133988
http_request_duration_seconds_bucket{le="+Inf"} 144320
http_request_duration_seconds_sum 53423
http_request_duration_seconds_count 144320
    `
}

// Configure a prometheus.scrape component to collect staic metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.static.demo.targets
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

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

[formats]: https://prometheus.io/docs/instrumenting/exposition_formats/
[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.static` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

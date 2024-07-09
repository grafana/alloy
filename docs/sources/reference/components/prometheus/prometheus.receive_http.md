---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.receive_http/
aliases:
  - ../prometheus.receive_http/ # /docs/alloy/latest/reference/components/prometheus.receive_http/
description: Learn about prometheus.receive_http
title: prometheus.receive_http
---

# prometheus.receive_http

`prometheus.receive_http` listens for HTTP requests containing Prometheus metric samples and forwards them to other components capable of receiving metrics.

The HTTP API exposed is compatible with [Prometheus `remote_write` API][prometheus-remote-write-docs]. This means that other [`prometheus.remote_write`][prometheus.remote_write] components can be used as a client and send requests to `prometheus.receive_http` which enables using {{< param "PRODUCT_NAME" >}} as a proxy for Prometheus metrics.

[prometheus.remote_write]: ../prometheus.remote_write/
[prometheus-remote-write-docs]: https://prometheus.io/docs/prometheus/2.45/querying/api/#remote-write-receiver

## Usage

```alloy
prometheus.receive_http "LABEL" {
  http {
    listen_address = "LISTEN_ADDRESS"
    listen_port = PORT
  }
  forward_to = RECEIVER_LIST
}
```

The component will start an HTTP server supporting the following endpoint:

- `POST /api/v1/metrics/write` - send metrics to the component, which in turn will be forwarded to the receivers as configured in `forward_to` argument. The request format must match that of [Prometheus `remote_write` API][prometheus-remote-write-docs]. One way to send valid requests to this component is to use another {{< param "PRODUCT_NAME" >}} with a [`prometheus.remote_write`][prometheus.remote_write] component.

## Arguments

`prometheus.receive_http` supports the following arguments:

Name         | Type             | Description                           | Default | Required
-------------|------------------|---------------------------------------|---------|---------
`forward_to` | `list(MetricsReceiver)` | List of receivers to send metrics to. |         | yes

## Blocks

The following blocks are supported inside the definition of `prometheus.receive_http`:

Hierarchy | Name     | Description                                        | Required
----------|----------|----------------------------------------------------|---------
`http`    | [http][] | Configures the HTTP server that receives requests. | no

[http]: #http

### http

{{< docs/shared lookup="reference/components/loki-server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`prometheus.receive_http` does not export any fields.

## Component health

`prometheus.receive_http` is reported as unhealthy if it is given an invalid configuration.

## Debug metrics

The following are some of the metrics that are exposed when this component is used. Note that the metrics include labels such as `status_code` where relevant, which can be used to measure request success rates.

* `prometheus_receive_http_request_duration_seconds` (histogram): Time (in seconds) spent serving HTTP requests.
* `prometheus_receive_http_request_message_bytes` (histogram): Size (in bytes) of messages received in the request.
* `prometheus_receive_http_response_message_bytes` (histogram): Size (in bytes) of messages sent in response.
* `prometheus_receive_http_tcp_connections` (gauge): Current number of accepted TCP connections.
* `prometheus_fanout_latency` (histogram): Write latency for sending metrics to other components.
* `prometheus_forwarded_samples_total` (counter): Total number of samples sent to downstream components.

## Example

### Receiving metrics over HTTP

This example creates a `prometheus.receive_http` component which starts an HTTP server listening on `0.0.0.0` and port `9999`. The server receives metrics and forwards them to a `prometheus.remote_write` component which writes these metrics to the specified HTTP endpoint.

```alloy
// Receives metrics over HTTP
prometheus.receive_http "api" {
  http {
    listen_address = "0.0.0.0"
    listen_port = 9999
  }
  forward_to = [prometheus.remote_write.local.receiver]
}

// Send metrics to a locally running Mimir.
prometheus.remote_write "local" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"

    basic_auth {
      username = "example-user"
      password = "example-password"
    }
  }
}
```

### Proxying metrics

In order to send metrics to the `prometheus.receive_http` component defined in the previous example, another {{< param "PRODUCT_NAME" >}} can run with the following configuration:

```alloy
// Collects metrics of localhost:12345
prometheus.scrape "self" {
  targets = [
    {"__address__" = "localhost:12345", "job" = "alloy"},
  ]
  forward_to = [prometheus.remote_write.local.receiver]
}

// Writes metrics to localhost:9999/api/v1/metrics/write - e.g. served by
// the prometheus.receive_http component from the example above.
prometheus.remote_write "local" {
  endpoint {
    url = "http://localhost:9999/api/v1/metrics/write"
  }
}
```

## Technical details

`prometheus.receive_http` uses [snappy](https://en.wikipedia.org/wiki/Snappy_(compression)) for compression.
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.receive_http` can accept arguments from the following components:

- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

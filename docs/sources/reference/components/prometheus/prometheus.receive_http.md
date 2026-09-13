---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.receive_http/
aliases:
  - ../prometheus.receive_http/ # /docs/alloy/latest/reference/components/prometheus.receive_http/
description: Learn about prometheus.receive_http
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.receive_http
---

# `prometheus.receive_http`

`prometheus.receive_http` listens for HTTP requests containing Prometheus metric samples and forwards them to other components capable of receiving metrics.

The component exposes two HTTP APIs:

* The [Prometheus `remote_write` API][prometheus-remote-write-docs], so that other [`prometheus.remote_write`][prometheus.remote_write] components can be used as a client and send requests to `prometheus.receive_http`.
  This enables using {{< param "PRODUCT_NAME" >}} as a proxy for Prometheus metrics.
* The [Pushgateway push API][pushgateway-docs], so that short-lived jobs can push metrics with the Prometheus client library or tooling they already use.

[prometheus.remote_write]: ../prometheus.remote_write/
[prometheus-remote-write-docs]: https://prometheus.io/docs/prometheus/latest/querying/api/#remote-write-receiver
[pushgateway-docs]: https://github.com/prometheus/pushgateway#api

## Usage

```alloy
prometheus.receive_http "<LABEL>" {
  http {
    listen_address = "<LISTEN_ADDRESS>"
    listen_port = <PORT>
  }
  forward_to = <RECEIVER_LIST>
}
```

The component starts an HTTP server supporting the following endpoints.
Both endpoints forward the metrics they receive to the receivers configured in the `forward_to` argument.

* `POST /api/v1/metrics/write`: Sends metrics to the component.
  The request format must be compatible with the [Prometheus remote_write API][prometheus-remote-write-docs] and can use either the v1 or v2 format.
  One way to send valid requests to this component is to use another {{< param "PRODUCT_NAME" >}} with a [`prometheus.remote_write`][prometheus.remote_write] component.
* `POST|PUT /metrics/job/<JOB_NAME>{/<LABEL_NAME>/<LABEL_VALUE>}`: Pushes metrics to the component.
  The request body must be in the Prometheus text, OpenMetrics, or protobuf exposition format, selected with the `Content-Type` header.
  {{< param "PRODUCT_NAME" >}} assumes the Prometheus text format when the header is missing.
  Refer to [Push metrics](#push-metrics) for details.

## Arguments

You can use the following arguments with `prometheus.receive_http`:

| Name                                      | Type                    | Description                                                           | Default                       | Required |
| ----------------------------------------- | ----------------------- | --------------------------------------------------------------------- | ----------------------------- | -------- |
| `forward_to`                              | `list(MetricsReceiver)` | List of receivers to send metrics to.                                 |                               | yes      |
| `accepted_remote_write_protobuf_messages` | `list(string)`          | Accepted remote write protobuf message types.                         | `["prometheus.WriteRequest"]` | no       |
| `append_metadata`                         | `bool`                  | Pass metric metadata to downstream components.                        | `false`                       | no       |
| `enable_type_and_unit_labels`             | `bool`                  | Add the metric type and unit as labels to the metric.                 | `false`                       | no       |

> **EXPERIMENTAL**: The `append_metadata`, `enable_type_and_unit_labels`, and using `"io.prometheus.write.v2.Request"` in `accepted_remote_write_protobuf_messages` are [experimental][] features.
>
> The `append_metadata` and `enable_type_and_unit_labels` arguments only apply to pushed exposition payloads and remote write v2 payloads, and only when metadata is included in those payloads.
> Enabling support for remote write v2 payloads requires that `"io.prometheus.write.v2.Request"` is included in `accepted_remote_write_protobuf_messages`.
> Remote write v1 payloads (`accepted_remote_write_protobuf_messages = ["prometheus.WriteRequest"]`) cannot support these features.
>
> Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.
> To enable and use an experimental feature, you must set the `stability.level` [flag][] to `experimental`.

[experimental]: https://grafana.com/docs/release-life-cycle/
[flag]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/cli/run/

## Blocks

You can use the following blocks with `prometheus.receive_http`:

{{< docs/alloy-config >}}

| Name                  | Description                                        | Required |
| --------------------- | -------------------------------------------------- | -------- |
| [`http`][http]        | Configures the HTTP server that receives requests. | no       |
| `http` > [`tls`][tls] | Configures TLS for the HTTP server.                | no       |

[http]: #http
[tls]: #tls

{{< /docs/alloy-config >}}

### `http`

{{< docs/shared lookup="reference/components/server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS for the HTTP server.

{{< docs/shared lookup="reference/components/server-tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`prometheus.receive_http` doesn't export any fields.

## Component health

`prometheus.receive_http` is reported as unhealthy if it's given an invalid configuration.

## Debug metrics

The following are some of the metrics that are exposed when this component is used.
The metrics include labels such as `status_code` where relevant, which can be used to measure request success rates.

* `prometheus_fanout_latency` (histogram): Write latency for sending metrics to other components.
* `prometheus_forwarded_samples_total` (counter): Total number of samples sent to downstream components.
* `prometheus_receive_http_request_duration_seconds` (histogram): Time (in seconds) spent serving HTTP requests.
* `prometheus_receive_http_request_message_bytes` (histogram): Size (in bytes) of messages received in the request.
* `prometheus_receive_http_response_message_bytes` (histogram): Size (in bytes) of messages sent in response.
* `prometheus_receive_http_tcp_connections` (gauge): Current number of accepted TCP connections.

## Push metrics

The `POST|PUT /metrics/job/<JOB_NAME>{/<LABEL_NAME>/<LABEL_VALUE>}` endpoint accepts the same requests as [Pushgateway][pushgateway-docs].
Short-lived jobs that can't be scraped can push their metrics to it with the Prometheus client library or the tooling they already use.

The following example pushes a metric to a component listening on port `9999`:

```shell
echo 'job_last_success_timestamp 1700000000' | \
  curl --data-binary @- http://localhost:9999/metrics/job/demo_batch/instance/worker-1
```

The path following `/metrics/` sets the grouping labels {{< param "PRODUCT_NAME" >}} adds to every series in the request body.
The `job` label is required and must come first.
These labels replace any label of the same name in the request body.
Add a `@base64` suffix to a label name to encode its value with the [base64url][] alphabet, which is how you push a value that contains a `/`.

Unlike Pushgateway, {{< param "PRODUCT_NAME" >}} doesn't store the metrics it receives.
It forwards them to the receivers in `forward_to` the same way it forwards a remote write request, which has the following consequences:

* {{< param "PRODUCT_NAME" >}} doesn't expose the pushed metrics for scraping, and doesn't repeat a sample until the job pushes it again.
  Push at the interval you want the samples to appear at.
* {{< param "PRODUCT_NAME" >}} keeps the timestamp of the samples that carry one, and stamps the other samples with the time the request arrived.
* {{< param "PRODUCT_NAME" >}} doesn't group the metrics, so `POST` and `PUT` do the same thing, and the `DELETE` method of the Pushgateway API isn't supported and responds with `405 Method Not Allowed`.

[base64url]: https://datatracker.ietf.org/doc/html/rfc4648#section-5

## Example

### Receive metrics over HTTP

The following example creates a `prometheus.receive_http` component which starts an HTTP server listening on port `9999` on all network interfaces.
The server receives metrics and forwards them to a `prometheus.remote_write` component which writes these metrics to the specified HTTP endpoint.

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

### Receive metrics from a batch job

The following example creates a `prometheus.receive_http` component that a batch job can push to.

```alloy
prometheus.receive_http "pushes" {
  http {
    listen_address = "0.0.0.0"
    listen_port = 9999
  }
  forward_to = [prometheus.remote_write.local.receiver]
}

prometheus.remote_write "local" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}
```

The job pushes the timestamp of its last successful run when it finishes:

```shell
printf 'job_last_success_timestamp %s\n' "$(date +%s)" \
  | curl --data-binary @- http://alloy:9999/metrics/job/demo_batch/instance/worker-1
```

### Proxy metrics

To send metrics to the `prometheus.receive_http` component defined in the previous example, another {{< param "PRODUCT_NAME" >}} can run with the following configuration:

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

`prometheus.receive_http` uses [snappy](<https://en.wikipedia.org/wiki/Snappy_(compression)>) for compression of remote write requests.
Requests to the push endpoint aren't compressed.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.receive_http` can accept arguments from the following components:

- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/sigil/sigil.receive/
description: Learn about sigil.receive
labels:
  stage: experimental
  products:
    - oss
title: sigil.receive
---

# `sigil.receive`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`sigil.receive` receives [Grafana AI Observability](https://grafana.com/docs/grafana-cloud/machine-learning/ai-observability/) generation export requests over HTTP and forwards them to `sigil.write` components or any other component that exposes a Sigil `GenerationsReceiver`.
Clients such as the [Grafana AI Observability SDK](https://github.com/grafana/sigil-sdk) send these requests to record AI generations.

The component starts an HTTP server that accepts `POST /api/v1/generations:export` requests with a JSON (`application/json`) body. It decodes the body into a typed Sigil `ExportGenerationsRequest` before forwarding so downstream components can inspect and modify it. Each downstream receiver in `forward_to` receives an independent clone of the request, which lets downstream components mutate the request safely without affecting sibling branches.

`sigil.receive` returns `400 Bad Request` when it cannot decode the request body, and `415 Unsupported Media Type` when the `Content-Type` header is set to anything other than `application/json`. An empty `Content-Type` is accepted and the body is parsed as JSON.

## Usage

```alloy
sigil.receive "<LABEL>" {
  forward_to = <RECEIVER_LIST>
}
```

## Arguments

You can use the following arguments with `sigil.receive`:

| Name            | Type                         | Description                                          | Default  | Required |
| --------------- | ---------------------------- | ---------------------------------------------------- | -------- | -------- |
| `forward_to`    | `list(GenerationsReceiver)`  | List of receivers to send generations to.            |          | yes      |
| `max_request_body_size` | `string`               | Maximum allowed request body size (for example, `"20MiB"`).  | `"20MiB"`| no       |

## Blocks

You can use the following blocks with `sigil.receive`:

{{< docs/alloy-config >}}

| Block                  | Description                                        | Required |
| ---------------------- | -------------------------------------------------- | -------- |
| [`http`][http]         | Configures the HTTP server that receives requests. | no       |
| `http` > [`tls`][tls] | Configures TLS for the HTTP server.                | no       |

[http]: #http
[tls]: #tls

{{< /docs/alloy-config >}}

The `>` symbol indicates deeper levels of nesting.
For example, `http` > `tls` refers to a `tls` block defined inside an `http` block.

### `http`

{{< docs/shared lookup="reference/components/server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS for the HTTP server.

{{< docs/shared lookup="reference/components/server-tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`sigil.receive` doesn't export any fields.

## Component health

`sigil.receive` is reported as unhealthy if it's given an invalid configuration.

## Debug information

`sigil.receive` doesn't expose any component-specific debug information.

## Debug metrics

| Metric                                              | Type      | Description                                                                       |
| --------------------------------------------------- | --------- | --------------------------------------------------------------------------------- |
| `sigil_receive_http_request_duration_seconds`       | Histogram | Time spent serving HTTP requests, labeled by `method`, `route`, and `status_code`. |
| `sigil_receive_http_tcp_connections`                | Gauge     | Current number of accepted TCP connections.                                       |
| `sigil_receive_http_tcp_connections_limit`          | Gauge     | Maximum number of TCP connections that the server can accept. 0 means no limit.   |
| `sigil_receive_fanout_partial_failures_total`       | Counter   | Times fan-out had at least one failed downstream branch but at least one success. |

The HTTP server emits additional `sigil_receive_http_*` metrics for request size, in-flight requests, and connection limits.

## Example

This example creates a `sigil.receive` that listens on the default address and forwards generation records to a `sigil.write` component.

```alloy
sigil.receive "default" {
  forward_to = [sigil.write.default.receiver]
}

sigil.write "default" {
  endpoint {
    url = "https://sigil.grafana.net"

    basic_auth {
      username = env("SIGIL_USER")
      password = env("SIGIL_API_KEY")
    }

    tenant_id = env("SIGIL_TENANT_ID")
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`sigil.receive` can accept arguments from the following components:

- Components that export [Sigil `GenerationsReceiver`](../../../compatibility/#sigil-generationsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

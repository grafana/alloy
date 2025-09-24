---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.receive_http/
description: Learn about pyroscope.receive_http
labels:
  stage: general-availability
  products:
    - oss
title: pyroscope.receive_http
---

# `pyroscope.receive_http`

`pyroscope.receive_http` receives profiles over HTTP and forwards them to `pyroscope.*` components capable of receiving profiles.

The HTTP API exposed is compatible with both the Pyroscope [HTTP ingest API](https://grafana.com/docs/pyroscope/latest/reference-server-api/) and the [pushv1.PusherService](https://github.com/grafana/pyroscope/blob/main/api/push/v1/push.proto) Connect API.
This allows `pyroscope.receive_http` to act as a proxy for Pyroscope profiles, enabling flexible routing and distribution of profile data.

## Usage

```alloy
pyroscope.receive_http "<LABEL>" {
  http {
    listen_address = "<LISTEN_ADDRESS>"
    listen_port = "<PORT>"
  }
  forward_to = <RECEIVER_LIST>
}
```

The component starts an HTTP server supporting the following endpoints:

* `POST /ingest`: Send profiles to the component, which forwards them to the receivers configured in the `forward_to` argument.
  The request format must match the format of the Pyroscope ingest API.
* `POST /push.v1.PusherService/Push`: Send profiles to the component, which forwards them to the receivers configured in the `forward_to` argument.
  The request format must match the format of the Pyroscope pushv1.PusherService Connect API.

## Arguments

You can use the following argument with `pyroscope.receive_http`:

| Name         | Type                     | Description                            | Default | Required |
| ------------ | ------------------------ | -------------------------------------- | ------- | -------- |
| `forward_to` | `list(ProfilesReceiver)` | List of receivers to send profiles to. |         | yes      |

## Blocks

You can use the following blocks with `pyroscope.receive_http`:

| Name                  | Description                                        | Required |
| --------------------- | -------------------------------------------------- | -------- |
| [`http`][http]        | Configures the HTTP server that receives requests. | no       |
| `http` > [`tls`][tls] | Configures TLS for the HTTP server.                | no       |

The > symbol indicates deeper levels of nesting.
For example, `http` > `tls` refers to a `tls` block defined inside an `http` block.

[http]: #http

### `http`

{{< docs/shared lookup="reference/components/server-http.md" source="alloy" version="<ALLOY_VERSION>" >}}

[tls]: #tls

### `tls`

The `tls` block configures TLS for the HTTP server.

{{< docs/shared lookup="reference/components/server-tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`pyroscope.receive_http` doesn't export any fields.

## Component health

`pyroscope.receive_http` is reported as unhealthy if it's given an invalid configuration.

## Debug metrics

`pyroscope_receive_http_tcp_connections` (gauge): Current number of accepted TCP connections.
`pyroscope_receive_http_tcp_connections_limit` (gauge): The maximum number of TCP connections that the component can accept. A value of 0 means no limit.

## Troubleshoot

{{< docs/shared lookup="reference/components/pyroscope-troubleshooting.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Example

This example creates a `pyroscope.receive_http` component, which starts an HTTP server listening on `0.0.0.0` and port `9999`.
The server receives profiles and forwards them to multiple `pyroscope.write` components, which write these profiles to different HTTP endpoints.

```alloy
// Receives profiles over HTTP
pyroscope.receive_http "default" {
  http {
    listen_address = "0.0.0.0"
    listen_port = 9999
  }
  forward_to = [pyroscope.write.staging.receiver, pyroscope.write.production.receiver]
}

// Send profiles to a staging Pyroscope instance
pyroscope.write "staging" {
  endpoint {
    url = "http://pyroscope-staging:4040"
  }
}

// Send profiles to a production Pyroscope instance
pyroscope.write "production" {
  endpoint {
    url = "http://pyroscope-production:4040"
  }
}
```

{{< admonition type="note" >}}
This example demonstrates forwarding to multiple `pyroscope.write` components.
This configuration duplicates the received profiles and sends a copy to each configured `pyroscope.write` component.
{{< /admonition >}}

You can also create multiple `pyroscope.receive_http` components with different configurations to listen on different addresses or ports as needed.
This flexibility allows you to design a setup that best fits your infrastructure and profile routing requirements.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`pyroscope.receive_http` can accept arguments from the following components:

- Components that export [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

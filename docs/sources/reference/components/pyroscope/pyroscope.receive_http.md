---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.receive_http/
description: Learn about pyroscope.receive_http
labels:
  stage: public-preview
title: pyroscope.receive_http
---

# `pyroscope.receive_http`

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

`pyroscope.receive_http` receives profiles over HTTP and forwards them to `pyroscope.*` components capable of receiving profiles.

The HTTP API exposed is compatible with the Pyroscope [HTTP ingest API](https://grafana.com/docs/pyroscope/latest/configure-server/about-server-api/).
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

The component starts an HTTP server supporting the following endpoint.

- `POST /ingest` - send profiles to the component, which will be forwarded to the receivers as configured in the `forward_to argument`. The request format must match the format of the Pyroscope ingest API.

## Arguments

You can use the following arguments with `pyroscope.receive_http`:

| Name         | Type                     | Description                            | Default | Required |
| ------------ | ------------------------ | -------------------------------------- | ------- | -------- |
| `forward_to` | `list(ProfilesReceiver)` | List of receivers to send profiles to. |         | yes      |

## Blocks

You can use the following blocks `pyroscope.receive_http`:

| Name           | Description                                        | Required |
| -------------- | -------------------------------------------------- | -------- |
| [`http`][http] | Configures the HTTP server that receives requests. | no       |

[http]: #http

### `http`

The `http` block configures the HTTP server.

You can use the following arguments to configure the `http` block. Any omitted fields take their default values.

| Name                   | Type       | Description                                                                                                      | Default  | Required |
| ---------------------- | ---------- | ---------------------------------------------------------------------------------------------------------------- | -------- | -------- |
| `conn_limit`           | `int`      | Maximum number of simultaneous HTTP connections. Defaults to 100.                                                | `0`      | no       |
| `listen_address`       | `string`   | Network address on which the server listens for new connections. Defaults to accepting all incoming connections. | `""`     | no       |
| `listen_port`          | `int`      | Port number on which the server listens for new connections.                                                     | `8080`   | no       |
| `server_idle_timeout`  | `duration` | Idle timeout for the HTTP server.                                                                                | `"120s"` | no       |
| `server_read_timeout`  | `duration` | Read timeout for the HTTP server.                                                                                | `"30s"`  | no       |
| `server_write_timeout` | `duration` | Write timeout for the HTTP server.                                                                               | `"30s"`  | no       |

## Exported fields

`pyroscope.receive_http` doesn't export any fields.

## Component health

`pyroscope.receive_http` is reported as unhealthy if it's given an invalid configuration.

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
This configuration duplicates the received profiles and send a copy to each configured `pyroscope.write` component.
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

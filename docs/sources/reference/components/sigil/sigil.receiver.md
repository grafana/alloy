---
canonical: https://grafana.com/docs/alloy/latest/reference/components/sigil/sigil.receiver/
description: Learn about sigil.receiver
labels:
  stage: experimental
  products:
    - oss
title: sigil.receiver
---

# `sigil.receiver`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`sigil.receiver` receives Sigil AI generation export requests over HTTP and forwards them to `sigil.write` components or other components capable of receiving generations.

The component starts an HTTP server that accepts `POST /api/v1/generations:export` requests.
Request and response bodies are forwarded as opaque bytes without deserialization.

## Usage

```alloy
sigil.receiver "<LABEL>" {
  forward_to = <RECEIVER_LIST>
}
```

## Arguments

You can use the following arguments with `sigil.receiver`:

| Name            | Type                         | Description                                          | Default  | Required |
| --------------- | ---------------------------- | ---------------------------------------------------- | -------- | -------- |
| `forward_to`    | `list(GenerationsReceiver)`  | List of receivers to send generations to.            |          | yes      |
| `max_request_body_size` | `string`               | Maximum allowed request body size (e.g. `"20MiB"`).  | `"20MiB"`| no       |

## Blocks

You can use the following blocks with `sigil.receiver`:

{{< docs/alloy-config >}}

| Block                  | Description                                        | Required |
| ---------------------- | -------------------------------------------------- | -------- |
| [`http`][http]         | Configures the HTTP server that receives requests. | no       |
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

`sigil.receiver` doesn't export any fields.

## Component health

`sigil.receiver` is reported as unhealthy if it's given an invalid configuration.

## Debug metrics

| Metric                                          | Type      | Description                                          |
| ----------------------------------------------- | --------- | ---------------------------------------------------- |
| `sigil_receiver_requests_total`                 | Counter   | Total number of generation export requests received. |
| `sigil_receiver_request_body_bytes_total`       | Counter   | Total bytes received in generation export requests.  |
| `sigil_receiver_request_duration_seconds`       | Histogram | Duration of generation export request handling.      |

## Example

This example creates a `sigil.receiver` that listens on the default address and forwards generation records to a `sigil.write` component.

```alloy
sigil.receiver "default" {
  forward_to = [sigil.write.default.receiver]
}

sigil.write "default" {
  endpoint {
    url = "https://sigil.grafana.net"

    basic_auth {
      username = env("SIGIL_USER")
      password = env("SIGIL_API_KEY")
    }

    headers = {
      "X-Scope-OrgID" = env("SIGIL_TENANT_ID"),
    }
  }
}
```

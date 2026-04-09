---
canonical: https://grafana.com/docs/alloy/latest/reference/components/sigil/sigil.write/
description: Learn about sigil.write
labels:
  stage: experimental
  products:
    - oss
title: sigil.write
---

# `sigil.write`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`sigil.write` receives Sigil AI generation records from other components and forwards them to remote Sigil ingest endpoints.

Request and response bodies are forwarded as opaque bytes without deserialization.
When multiple `endpoint` blocks are provided, generation records are concurrently forwarded to all configured locations.

You can specify multiple `sigil.write` components by giving them different labels.

## Usage

```alloy
sigil.write "<LABEL>" {
  endpoint {
    url = "<SIGIL_URL>"
  }
}
```

## Arguments

`sigil.write` has no top-level arguments.

## Blocks

You can use the following blocks with `sigil.write`:

{{< docs/alloy-config >}}

| Block                                              | Description                                                | Required |
| -------------------------------------------------- | ---------------------------------------------------------- | -------- |
| [`endpoint`][endpoint]                             | Location to send generations to.                           | no       |
| `endpoint` > [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| `endpoint` > [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint. | no       |
| `endpoint` > [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `endpoint` > `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.     | no       |
| `endpoint` > [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.     | no       |

[endpoint]: #endpoint
[authorization]: #authorization
[basic_auth]: #basic_auth
[oauth2]: #oauth2
[tls_config]: #tls_config

{{< /docs/alloy-config >}}

### `endpoint`

The `endpoint` block describes a single location to send generation records to.
Multiple `endpoint` blocks can be provided to fan out to multiple locations.

The following arguments are supported:

| Name                     | Type                | Description                                                                                      | Default   | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | --------- | -------- |
| `url`                    | `string`            | Full URL of the Sigil ingest endpoint.                                                           |           | yes      |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |           | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |           | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`    | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`    | no       |
| `headers`                | `map(string)`       | Extra headers to deliver with the request.                                                       |           | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |           | no       |
| `max_backoff_period`     | `duration`          | Maximum backoff time between retries.                                                            | `"5m"`    | no       |
| `max_backoff_retries`    | `int`               | Maximum number of retries. 0 to retry infinitely.                                                | `10`      | no       |
| `min_backoff_period`     | `duration`          | Initial backoff time between retries.                                                            | `"500ms"` | no       |
| `name`                   | `string`            | Optional name to identify the endpoint in metrics.                                               |           | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |           | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |           | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`   | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |           | no       |
| `remote_timeout`         | `duration`          | Timeout for requests made to the URL.                                                            | `"10s"`   | no       |
| `tenant_id`              | `string`            | Tenant ID to use for `X-Scope-OrgID` header. Overrides the value from the incoming request.      |           | no       |

At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][endpoint] argument
* [`bearer_token`][endpoint] argument
* [`oauth2`][oauth2] block

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

Requests are retried on HTTP 429, 408, and 5xx responses with exponential backoff. Requests that receive other 4xx responses are not retried.

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type       | Description                                                     |
| ---------- | ---------- | --------------------------------------------------------------- |
| `receiver` | `receiver` | A value that other components can use to send generations to.   |

## Component health

`sigil.write` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields are kept at their last healthy values.

## Debug metrics

| Metric                                | Type      | Description                                                    |
| ------------------------------------- | --------- | -------------------------------------------------------------- |
| `sigil_write_sent_bytes_total`        | Counter   | Total number of bytes sent to Sigil endpoints.                 |
| `sigil_write_dropped_bytes_total`     | Counter   | Total number of bytes dropped by Sigil write.                  |
| `sigil_write_requests_total`          | Counter   | Total number of requests sent to Sigil endpoints.              |
| `sigil_write_retries_total`           | Counter   | Total number of retries to Sigil endpoints.                    |
| `sigil_write_latency`                 | Histogram | Write latency for sending generations to Sigil endpoints.      |

All metrics include an `endpoint` label identifying the specific endpoint URL.

## Example

This example forwards generation records to two Sigil endpoints with different tenant IDs.

```alloy
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

  endpoint {
    url = "https://sigil-staging.internal"
    tenant_id = "staging"
  }
}

sigil.receiver "default" {
  forward_to = [sigil.write.default.receiver]
}
```

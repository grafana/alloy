---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/remotecfg/
description: Learn about the remotecfg configuration block
labels:
  stage: general-availability
  products:
    - oss
title: remotecfg
---

# `remotecfg`

`remotecfg` is an optional configuration block that enables {{< param "PRODUCT_NAME" >}} to fetch and load the configuration from a remote endpoint.
You specify `remotecfg` without a label and can only include it once per configuration file.

The [API definition][] for managing and fetching configuration that the `remotecfg` block uses is available under the Apache 2.0 license.

{{< admonition type="note" >}}
The `remotecfg` block requires a compatible remote configuration management server that implements the [alloy-remote-config API][API definition].
The server dynamically decides which configuration to serve based on the collector's `id` and `attributes`.

If you want to load a static configuration file from an HTTP server, use [import.http][] instead.
Refer to [Load configuration from remote sources][load-remote] for more information.

[import.http]: ../import.http/
[load-remote]: ../../configure/load-remote-configuration/
{{< /admonition >}}

## Usage

```alloy
remotecfg {

}
```

## Arguments

You can use the following arguments with `remotecfg`:

| Name                     | Type                | Description                                                                                      | Default   | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | --------- | -------- |
| `attributes`             | `map(string)`       | A set of self-reported attributes.                                                               | `{}`      | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |           | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |           | no       |
| `enable_http2`           | `bool`              | Whether to enable HTTP2 for requests.                                                            | `true`    | no       |
| `follow_redirects`       | `bool`              | Whether to follow redirects returned by the server.                                              | `true`    | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to send with each request. The map key is the header name.                   |           | no       |
| `id`                     | `string`            | A self-reported ID.                                                                              | see below | no       |
| `name`                   | `string`            | A human-readable name for the collector.                                                         | `""`      | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. | `""`      | no       |
| `poll_frequency`         | `duration`          | How often to poll the API for configuration updates.                                             | `"1m"`    | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |           | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`   | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             | `""`      | no       |
| `url`                    | `string`            | The address of the API to poll for configuration.                                                | `""`      | no       |

If you don't set the `url`, the `remotecfg` block has no effect.

If you don't set `id`, {{< param "PRODUCT_NAME" >}} generates a random, anonymous unique ID (UUID) and stores it in an `alloy_seed.json` file in the {{< param "PRODUCT_NAME" >}} storage path.
This allows the ID to persist across restarts.
You can use the `name` field to set a human-friendly identifier for the {{< param "PRODUCT_NAME" >}} instance.

{{< param "PRODUCT_NAME" >}} includes the `id` and `attributes` fields in periodic requests to the remote endpoint so the API can decide what configuration to serve.

The `attributes` map keys can include any custom value except the reserved prefix `collector.`.
The reserved label prefix is for automatic system attributes.
You can't override this prefix.

- `collector.os`: The operating system where {{< param "PRODUCT_NAME" >}} is running.
- `collector.version`: The version of {{< param "PRODUCT_NAME" >}}.

You must set `poll_frequency` to at least `"10s"`.

You can provide at most one of the following:

- [`authorization`][authorization] block
- [`basic_auth`][basic_auth] block
- [`bearer_token_file`][arguments] argument
- [`bearer_token`][arguments] argument
- [`oauth2`][oauth2] block

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `remotecfg`:

| Block                                 | Description                                                | Required |
| ------------------------------------- | ---------------------------------------------------------- | -------- |
| [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint. | no       |
| [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.     | no       |
| [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.     | no       |

The > symbol indicates deeper levels of nesting.
For example, `oauth2` > `tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Example

```alloy
remotecfg {
    url = "<SERVICE_URL>"
    basic_auth {
        username      = "<USERNAME>"
        password_file = "<PASSWORD_FILE>"
    }

    id             = constants.hostname
    attributes     = {"cluster" = "dev", "namespace" = "otlp-dev"}
    poll_frequency = "5m"
}
```

## Troubleshooting

If {{< param "PRODUCT_NAME" >}} fails to load configuration using `remotecfg`, check the following:

- `401` or `403` errors: Verify that authentication settings are correct, such as `basic_auth`, `authorization`, OAuth2, or bearer token.
- `404` errors: Confirm that the configured `url` points to a server implementing the alloy-remote-config API.
  Static HTTP servers can't serve configuration for `remotecfg`.
- `415 Unsupported Media Type` errors: Ensure the server implements the alloy-remote-config API and returns the expected response format.
- Connection timeouts: Check network connectivity, proxy settings, and firewall rules between the collector and the remote server.

If you only want to load a static configuration file from an HTTP server, use [`import.http`][import.http] instead.

[API definition]: https://github.com/grafana/alloy-remote-config
[arguments]: #arguments
[authorization]: #authorization
[basic_auth]: #basic_auth
[import.http]: ../import.http/
[oauth2]: #oauth2
[tls_config]: #tls_config

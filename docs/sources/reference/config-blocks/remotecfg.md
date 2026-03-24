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
`remotecfg` is specified without a label and can only be provided once per configuration file.

The [API definition][] for managing and fetching configuration that the `remotecfg` block uses is available under the Apache 2.0 license.

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
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`    | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`    | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |           | no       |
| `id`                     | `string`            | A self-reported ID.                                                                              | see below | no       |
| `name`                   | `string`            | A human-readable name for the collector.                                                         | `""`      | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. | `""`      | no       |
| `poll_frequency`         | `duration`          | How often to poll the API for new configuration.                                                 | `"1m"`    | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |           | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`   | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             | `""`      | no       |
| `url`                    | `string`            | The address of the API to poll for configuration.                                                | `""`      | no       |

If the `url` isn't set, then the service block is a no-op.

If not set, the self-reported `id` that {{< param "PRODUCT_NAME" >}} uses is a randomly generated, anonymous unique ID (UUID) that is stored as an `alloy_seed.json` file in the {{< param "PRODUCT_NAME" >}} storage path so that it can persist across restarts.
You can use the `name` field to set another human-friendly identifier for the specific {{< param "PRODUCT_NAME" >}} instance.

The `id` and `attributes` fields are used in the periodic request sent to the remote endpoint so that the API can decide what configuration to serve.

The `attribute` map keys can include any custom value except the reserved prefix `collector.`.
The reserved label prefix is for automatic system attributes.
You can't override this prefix.

* `collector.os`: The operating system where {{< param "PRODUCT_NAME" >}} is running.
* `collector.version`: The version of {{< param "PRODUCT_NAME" >}}.

The `poll_frequency` must be set to at least `"10s"`.

At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments] argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

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

[API definition]: https://github.com/grafana/alloy-remote-config
[arguments]: #arguments
[basic_auth]: #basic_auth
[authorization]: #authorization
[oauth2]: #oauth2
[tls_config]: #tls_config

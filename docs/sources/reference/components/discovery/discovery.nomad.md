---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.nomad/
aliases:
  - ../discovery.nomad/ # /docs/alloy/latest/reference/components/discovery.nomad/
description: Learn about discovery.nomad
labels:
  stage: general-availability
  products:
    - oss
title: discovery.nomad
---

# `discovery.nomad`

`discovery.nomad` allows you to retrieve scrape targets from [Nomad's](https://www.nomadproject.io/) Service API.

## Usage

```alloy
discovery.nomad "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `discovery.nomad`:

| Name                     | Type                | Description                                                                                      | Default                   | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------------------------- | -------- |
| `allow_stale`            | `bool`              | Allow reading from non-leader nomad instances.                                                   | `true`                    | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |                           | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |                           | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`                    | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`                    | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |                           | no       |
| `namespace`              | `string`            | Nomad namespace to use.                                                                          | `default`                 | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |                           | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |                           | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`                   | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |                           | no       |
| `refresh_interval`       | `duration`          | Frequency to refresh list of containers.                                                         | `"30s"`                   | no       |
| `region`                 | `string`            | Nomad region to use.                                                                             | `global`                  | no       |
| `server`                 | `string`            | Address of nomad server.                                                                         | `"http://localhost:4646"` | no       |
| `tag_separator`          | `string`            | Separator to join nomad tags into Prometheus labels.                                             | `","`                     | no       |

 At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments] argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

[arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `discovery.nomad`:

| Block                                 | Description                                                | Required |
| ------------------------------------- | ---------------------------------------------------------- | -------- |
| [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint. | no       |
| [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.     | no       |
| [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.     | no       |

The > symbol indicates deeper levels of nesting.
For example, `oauth2` > `tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

[authorization]: #authorization
[basic_auth]: #basic_auth
[oauth2]: #oauth2
[tls_config]: #tls_config

### `authorization`

The `authorization` block configures generic authorization to the endpoint.

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

The `basic_auth` block configures basic authentication to the endpoint.

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

The `oauth` block configures OAuth 2.0 authentication to the endpoint.

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

The `tls_config` block configures TLS settings for connecting to the endpoint.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                                          |
| --------- | ------------------- | ---------------------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from the nomad server. |

Each target includes the following labels:

* `__meta_nomad_address`: The service address of the target.
* `__meta_nomad_dc`: The data center name for the target.
* `__meta_nomad_namespace`: The namespace of the target.
* `__meta_nomad_node_id`: The node name defined for the target.
* `__meta_nomad_service_address`: The service address of the target.
* `__meta_nomad_service_id`: The service ID of the target.
* `__meta_nomad_service_port`: The service port of the target.
* `__meta_nomad_service`: The name of the service the target belongs to.
* `__meta_nomad_tags`: The list of tags of the target joined by the tag separator.

## Component health

`discovery.nomad` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.nomad` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.nomad` doesn't expose any component-specific debug metrics.

## Example

This example discovers targets from a Nomad server:

```alloy
discovery.nomad "example" {
}

prometheus.scrape "demo" {
  targets    = discovery.nomad.example.targets
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

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.nomad` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

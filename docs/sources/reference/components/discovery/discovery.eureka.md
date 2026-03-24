---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.eureka/
aliases:
  - ../discovery.eureka/ # /docs/alloy/latest/reference/components/discovery.eureka/
description: Learn about discovery.eureka
labels:
  stage: general-availability
  products:
    - oss
title: discovery.eureka
---

# `discovery.eureka`

`discovery.eureka` discovers instances in a [Eureka][] Registry and exposes them as targets.

[Eureka]: https://github.com/Netflix/eureka/

## Usage

```alloy
discovery.eureka "<LABEL>" {
    server = "<SERVER>"
}
```

## Arguments

You can use the following arguments with `discovery.eureka`:

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `server`                 | `string`            | Eureka server URL.                                                                               |         | yes      |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |
| `refresh_interval`       | `duration`          | Interval at which to refresh the list of targets.                                                | `"30s"` | no       |

At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments] argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

[arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `discovery.eureka`:

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

The `oauth2` block configures OAuth 2.0 authentication to the endpoint.

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

The `tls_config` block configures TLS settings for connecting to the endpoint.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                                        |
| --------- | ------------------- | -------------------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from the Eureka API. |

Each target includes the following labels:

* `__meta_eureka_app_instance_country_id`
* `__meta_eureka_app_instance_datacenterinfo_metadata_`
* `__meta_eureka_app_instance_datacenterinfo_name`
* `__meta_eureka_app_instance_healthcheck_url`
* `__meta_eureka_app_instance_homepage_url`
* `__meta_eureka_app_instance_hostname`
* `__meta_eureka_app_instance_id`
* `__meta_eureka_app_instance_ip_addr`
* `__meta_eureka_app_instance_metadata_`
* `__meta_eureka_app_instance_port_enabled`
* `__meta_eureka_app_instance_port`
* `__meta_eureka_app_instance_secure_port_enabled`
* `__meta_eureka_app_instance_secure_port`
* `__meta_eureka_app_instance_secure_vip_address`
* `__meta_eureka_app_instance_status`
* `__meta_eureka_app_instance_statuspage_url`
* `__meta_eureka_app_instance_vip_address`
* `__meta_eureka_app_name`

## Component health

`discovery.eureka` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.eureka` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.eureka` doesn't expose any component-specific debug metrics.

## Example

```alloy
discovery.eureka "example" {
    server = "https://eureka.example.com/eureka/v1"
}

prometheus.scrape "demo" {
  targets    = discovery.eureka.example.targets
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

`discovery.eureka` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

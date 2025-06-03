---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.lightsail/
aliases:
  - ../discovery.lightsail/ # /docs/alloy/latest/reference/components/discovery.lightsail/
description: Learn about discovery.lightsail
labels:
  stage: general-availability
  products:
    - oss
title: discovery.lightsail
---

# `discovery.lightsail`

`discovery.lightsail` allows retrieving scrape targets from Amazon Lightsail instances. The private IP address is used by default, but may be changed to the public IP address with relabeling.

## Usage

```alloy
discovery.lightsail "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `discovery.lightsail`:

| Name                     | Type                | Description                                                                                                             | Default | Required |
| ------------------------ | ------------------- | ----------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `access_key`             | `string`            | The AWS API key ID. If blank, the environment variable `AWS_ACCESS_KEY_ID` is used.                                     |         | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                                                    |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                                                      |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                                                | `true`  | no       |
| `endpoint`               | `string`            | Custom endpoint to be used.                                                                                             |         | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                                            | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.                                 |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying.                        |         | no       |
| `port`                   | `int`               | The port to scrape metrics from. If using the public IP address, this must instead be specified in the relabeling rule. | `80`    | no       |
| `profile`                | `string`            | Named AWS profile used to connect to the API.                                                                           |         | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                                           |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                                                   | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                                                    |         | no       |
| `refresh_interval`       | `duration`          | Refresh interval to re-read the instance list.                                                                          | `"60s"` | no       |
| `region`                 | `string`            | The AWS region. If blank, the region from the instance metadata is used.                                                |         | no       |
| `role_arn`               | `string`            | AWS Role ARN, an alternative to using AWS API keys.                                                                     |         | no       |
| `secret_key`             | `string`            | The AWS API key secret. If blank, the environment variable `AWS_SECRET_ACCESS_KEY` is used.                             |         | no       |

At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments] argument
* [`bearer_token][arguments] argument
* [`oauth2`][oauth2] block

 [arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `discovery.lightsail`:

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

| Name      | Type                | Description                              |
| --------- | ------------------- | ---------------------------------------- |
| `targets` | `list(map(string))` | The set of discovered Lightsail targets. |

Each target includes the following labels:

* `__meta_lightsail_availability_zone`: The availability zone in which the instance is running.
* `__meta_lightsail_blueprint_id`: The Lightsail blueprint ID.
* `__meta_lightsail_bundle_id`: The Lightsail bundle ID.
* `__meta_lightsail_instance_name`: The name of the Lightsail instance.
* `__meta_lightsail_instance_state`: The state of the Lightsail instance.
* `__meta_lightsail_instance_support_code`: The support code of the Lightsail instance.
* `__meta_lightsail_ipv6_addresses`: Comma-separated list of IPv6 addresses assigned to the instance's network interfaces, if present.
* `__meta_lightsail_private_ip`: The private IP address of the instance.
* `__meta_lightsail_public_ip`: The public IP address of the instance, if available.
* `__meta_lightsail_region`: The region of the instance.
* `__meta_lightsail_tag_<tagkey>`: Each tag value of the instance.

## Component health

`discovery.lightsail` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.lightsail` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.lightsail` doesn't expose any component-specific debug metrics.

## Example

```alloy
discovery.lightsail "lightsail" {
  region = "us-east-1"
}

prometheus.scrape "demo" {
  targets    = discovery.lightsail.lightsail.targets
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

`discovery.lightsail` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

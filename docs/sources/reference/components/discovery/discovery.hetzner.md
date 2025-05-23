---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.hetzner/
aliases:
  - ../discovery.hetzner/ # /docs/alloy/latest/reference/components/discovery.hetzner/
description: Learn about discovery.hetzner
labels:
  stage: general-availability
  products:
    - oss
title: discovery.hetzner
---

# `discovery.hetzner`

`discovery.hetzner` allows retrieving scrape targets from [Hetzner Cloud API][] and [Robot API][].
This service discovery uses the public IPv4 address by default, but that can be changed with relabeling.

[Hetzner Cloud API]: https://www.hetzner.com/
[Robot API]: https://docs.hetzner.com/robot/

## Usage

```alloy
discovery.hetzner "<LABEL>" {
  role = "<HETZNER_ROLE>"
}
```

## Arguments

You can use the following arguments with `discovery.hetzner`:

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `role`                   | `string`            | Hetzner role of entities that should be discovered.                                              |         | yes      |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `port`                   | `int`               | The port to scrape metrics from.                                                                 | `80`    | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |
| `refresh_interval`       | `duration`          | The time after which the servers are refreshed.                                                  | `"60s"` | no       |

`role` must be one of `robot` or `hcloud`.

 At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments] argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

[arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `discovery.hetzner`:

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

| Name      | Type                | Description                                                 |
| --------- | ------------------- | ----------------------------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from the Hetzner catalog API. |

Each target includes the following labels:

* `__meta_hetzner_datacenter`: The data center of the server
* `__meta_hetzner_public_ipv4`: The public IPv4 address of the server.
* `__meta_hetzner_public_ipv6_network`: The public IPv6 network (/64) of the server.
* `__meta_hetzner_server_id`: The ID of the server.
* `__meta_hetzner_server_name`: The name of the server.
* `__meta_hetzner_server_status`: The status of the server.

### `hcloud`

The labels below are only available for targets with `role` set to `hcloud`:

* `__meta_hetzner_hcloud_cpu_cores`: The CPU cores count of the server.
* `__meta_hetzner_hcloud_cpu_type`: The CPU type of the server (shared or dedicated).
* `__meta_hetzner_hcloud_datacenter_location_network_zone`: The network zone of the server.
* `__meta_hetzner_hcloud_datacenter_location`: The location of the server.
* `__meta_hetzner_hcloud_disk_size_gb`: The disk size of the server (in GB).
* `__meta_hetzner_hcloud_image_description`: The description of the server image.
* `__meta_hetzner_hcloud_image_name`: The image name of the server.
* `__meta_hetzner_hcloud_image_os_flavor`: The OS flavor of the server image.
* `__meta_hetzner_hcloud_image_os_version`: The OS version of the server image.
* `__meta_hetzner_hcloud_label_<labelname>`: Each label of the server.
* `__meta_hetzner_hcloud_labelpresent_<labelname>`: `true` for each label of the server.
* `__meta_hetzner_hcloud_memory_size_gb`: The amount of memory of the server (in GB).
* `__meta_hetzner_hcloud_private_ipv4_<networkname>`: The private IPv4 address of the server within a given network.
* `__meta_hetzner_hcloud_server_type`: The type of the server.

### `robot`

The labels below are only available for targets with `role` set to `robot`:

* `__meta_hetzner_robot_cancelled`: The server cancellation status.
* `__meta_hetzner_robot_product`: The product of the server.

## Component health

`discovery.hetzner` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.hetzner` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.hetzner` doesn't expose any component-specific debug metrics.

## Example

This example discovers targets from Hetzner:

```alloy
discovery.hetzner "example" {
  role = "<HETZNER_ROLE>"
}

prometheus.scrape "demo" {
  targets    = discovery.hetzner.example.targets
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

* _`<HETZNER_ROLE>`_: The role of the entities that should be discovered.
* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.hetzner` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

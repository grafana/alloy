---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.linode/
aliases:
  - ../discovery.linode/ # /docs/alloy/latest/reference/components/discovery.linode/
description: Learn about discovery.linode
labels:
  stage: general-availability
  products:
    - oss
title: discovery.linode
---

# `discovery.linode`

`discovery.linode` allows you to retrieve scrape targets from [Linode's][] Linode APIv4.
This service discovery uses the public IPv4 address by default, but that can be changed with relabeling.

[Linode's]: https://www.linode.com/

## Usage

```alloy
discovery.linode "<LABEL>" {
    bearer_token = "<LINODE_API_TOKEN>"
}
```

{{< admonition type="note" >}}
You must create the Linode APIv4 Token with the scopes: `linodes:read_only`, `ips:read_only`, and `events:read_only`.
{{< /admonition >}}

## Arguments

You can use the following arguments with `discovery.linode`:

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `port`                   | `int`               | Port that metrics are scraped from.                                                              | `80`    | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |
| `refresh_interval`       | `duration`          | The time to wait between polling update requests.                                                | `"60s"` | no       |
| `region`                 | `string`            | A region to filter on.                                                                           |         | no       |
| `tag_separator`          | `string`            | The string by which Linode Instance tags are joined into the tag label.                          | `","`   | no       |

 At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments] argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

[arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `discovery.linode`:

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

| Name      | Type                | Description                                        |
| --------- | ------------------- | -------------------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from the Linode API. |

The following meta labels are available on targets and can be used by the discovery.relabel component:

* `__meta_linode_backups`: The backup service status of the Linode instance.
* `__meta_linode_extra_ips`: A list of all extra IPv4 addresses assigned to the Linode instance joined by the tag separator.
* `__meta_linode_group`: The display group a Linode instance is a member of.
* `__meta_linode_gpus`: The number of GPUs of the Linode instance.
* `__meta_linode_hypervisor`: The virtualization software powering the Linode instance.
* `__meta_linode_image`: The slug of the Linode instance's image.
* `__meta_linode_instance_id`: The ID of the Linode instance.
* `__meta_linode_instance_label`: The label of the Linode instance.
* `__meta_linode_private_ipv4`: The private IPv4 of the Linode instance.
* `__meta_linode_private_ipv4_rdns`: The reverse DNS for the first private IPv4 of the Linode instance.
* `__meta_linode_public_ipv4`: The public IPv4 of the Linode instance.
* `__meta_linode_public_ipv4_rdns`: The reverse DNS for the first public IPv4 of the Linode instance.
* `__meta_linode_public_ipv6`: The public IPv6 of the Linode instance.
* `__meta_linode_public_ipv6_rdns`: The reverse DNS for the first public IPv6 of the Linode instance.
* `__meta_linode_region`: The region of the Linode instance.
* `__meta_linode_specs_disk_bytes`: The amount of storage space the Linode instance has access to.
* `__meta_linode_specs_memory_bytes`: The amount of RAM the Linode instance has access to.
* `__meta_linode_specs_transfer_bytes`: The amount of network transfer the Linode instance is allotted each month.
* `__meta_linode_specs_vcpus`: The number of VCPUS this Linode has access to.
* `__meta_linode_status`: The status of the Linode instance.
* `__meta_linode_tags`: A list of tags of the Linode instance joined by the tag separator.
* `__meta_linode_type`: The type of the Linode instance.
* `__meta_linode_ipv6_ranges`: A list of IPv6 ranges with mask assigned to the Linode instance joined by the tag separator.

## Component health

`discovery.linode` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.linode` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.linode` doesn't expose any component-specific debug metrics.

## Example

```alloy
discovery.linode "example" {
    bearer_token = sys.env("LINODE_TOKEN")
    port = 8876
}
prometheus.scrape "demo" {
    targets    = discovery.linode.example.targets
    forward_to = [prometheus.remote_write.demo.receiver]
}
prometheus.remote_write "demo" {
    endpoint {
        url = <PROMETHEUS_REMOTE_WRITE_URL>
        basic_auth {
            username = <USERNAME>
            password = <PASSWORD>
        }
    }
}
```

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Use a private IP address

```alloy
discovery.linode "example" {
    bearer_token = sys.env("LINODE_TOKEN")
    port = 8876
}
discovery.relabel "private_ips" {
    targets = discovery.linode.example.targets
    rule {
        source_labels = ["__meta_linode_private_ipv4"]
        replacement     = "[$1]:8876"
        target_label  = "__address__"
    }
}
prometheus.scrape "demo" {
    targets    = discovery.relabel.private_ips.targets
    forward_to = [prometheus.remote_write.demo.receiver]
}
prometheus.remote_write "demo" {
    endpoint {
        url = <PROMETHEUS_REMOTE_WRITE_URL>
        basic_auth {
            username = <USERNAME>
            password = <PASSWORD>
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

`discovery.linode` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

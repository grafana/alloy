---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.hetzner/
aliases:
  - ../discovery.hetzner/ # /docs/alloy/latest/reference/components/discovery.hetzner/
description: Learn about discovery.hetzner
title: discovery.hetzner
---

# discovery.hetzner

`discovery.hetzner` allows retrieving scrape targets from [Hetzner Cloud API][] and [Robot API][].
This service discovery uses the public IPv4 address by default, but that can be changed with relabeling.

[Hetzner Cloud API]: https://www.hetzner.com/
[Robot API]: https://docs.hetzner.com/robot/

## Usage

```alloy
discovery.hetzner "LABEL" {
  role = HETZNER_ROLE
}
```

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                                                      | Default | Required
-------------------------|---------------------|--------------------------------------------------------------------------------------------------|---------|---------
`role`                   | `string`            | Hetzner role of entities that should be discovered.                                              |         | yes
`port`                   | `int`               | The port to scrape metrics from.                                                                 | `80`    | no
`refresh_interval`       | `duration`          | The time after which the servers are refreshed.                                                  | `"60s"` | no
`bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no
`bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no

`role` must be one of `robot` or `hcloud`.

 At most, one of the following can be provided:
 - [`bearer_token` argument](#arguments).
 - [`bearer_token_file` argument](#arguments).
 - [`basic_auth` block][basic_auth].
 - [`authorization` block][authorization].
 - [`oauth2` block][oauth2].

[arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

The following blocks are supported inside the definition of
`discovery.hetzner`:

Hierarchy           | Block             | Description                                              | Required
--------------------|-------------------|----------------------------------------------------------|---------
basic_auth          | [basic_auth][]    | Configure basic_auth for authenticating to the endpoint. | no
authorization       | [authorization][] | Configure generic authorization to the endpoint.         | no
oauth2              | [oauth2][]        | Configure OAuth2 for authenticating to the endpoint.     | no
oauth2 > tls_config | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no
tls_config          | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no

The `>` symbol indicates deeper levels of nesting.
For example, `oauth2 > tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block

### basic_auth block

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### authorization block

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### oauth2 block

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                | Description
----------|---------------------|------------------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the Hetzner catalog API.

Each target includes the following labels:

* `__meta_hetzner_server_id`: the ID of the server
* `__meta_hetzner_server_name`: the name of the server
* `__meta_hetzner_server_status`: the status of the server
* `__meta_hetzner_public_ipv4`: the public ipv4 address of the server
* `__meta_hetzner_public_ipv6_network`: the public ipv6 network (/64) of the server
* `__meta_hetzner_datacenter`: the datacenter of the server

### `hcloud`

The labels below are only available for targets with `role` set to `hcloud`:

* `__meta_hetzner_hcloud_image_name`: the image name of the server
* `__meta_hetzner_hcloud_image_description`: the description of the server image
* `__meta_hetzner_hcloud_image_os_flavor`: the OS flavor of the server image
* `__meta_hetzner_hcloud_image_os_version`: the OS version of the server image
* `__meta_hetzner_hcloud_datacenter_location`: the location of the server
* `__meta_hetzner_hcloud_datacenter_location_network_zone`: the network zone of the server
* `__meta_hetzner_hcloud_server_type`: the type of the server
* `__meta_hetzner_hcloud_cpu_cores`: the CPU cores count of the server
* `__meta_hetzner_hcloud_cpu_type`: the CPU type of the server (shared or dedicated)
* `__meta_hetzner_hcloud_memory_size_gb`: the amount of memory of the server (in GB)
* `__meta_hetzner_hcloud_disk_size_gb`: the disk size of the server (in GB)
* `__meta_hetzner_hcloud_private_ipv4_<networkname>`: the private ipv4 address of the server within a given network
* `__meta_hetzner_hcloud_label_<labelname>`: each label of the server
* `__meta_hetzner_hcloud_labelpresent_<labelname>`: `true` for each label of the server

### `robot`

The labels below are only available for targets with `role` set to `robot`:

* `__meta_hetzner_robot_product`: the product of the server
* `__meta_hetzner_robot_cancelled`: the server cancellation status


## Component health

`discovery.hetzner` is only reported as unhealthy when given an invalid
configuration. In those cases, exported fields retain their last healthy
values.

## Debug information

`discovery.hetzner` does not expose any component-specific debug information.

## Debug metrics

`discovery.hetzner` does not expose any component-specific debug metrics.

## Example

This example discovers targets from Hetzner:

```alloy
discovery.hetzner "example" {
  role = HETZNER_ROLE
}

prometheus.scrape "demo" {
  targets    = discovery.hetzner.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```
Replace the following:
  - `HETZNER_ROLE`: The role of the entities that should be discovered.
  - `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
  - `USERNAME`: The username to use for authentication to the remote_write API.
  - `PASSWORD`: The password to use for authentication to the remote_write API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.hetzner` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

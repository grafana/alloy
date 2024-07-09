---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.ionos/
aliases:
  - ../discovery.ionos/ # /docs/alloy/latest/reference/components/discovery.ionos/
description: Learn about discovery.ionos
title: discovery.ionos
---

# discovery.ionos

`discovery.ionos` allows you to retrieve scrape targets from [IONOS Cloud][] API.

[IONOS Cloud]: https://cloud.ionos.com/

## Usage

```alloy
discovery.ionos "LABEL" {
    datacenter_id = DATACENTER_ID
}
```

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                                                      | Default | Required
-------------------------|---------------------|--------------------------------------------------------------------------------------------------|---------|---------
`datacenter_id`          | `string`            | The unique ID of the data center.                                                                |         | yes
`refresh_interval`       | `duration`          | The time after which the servers are refreshed.                                                  | `60s`   | no
`port`                   | `int`               | The port to scrape metrics from.                                                                 | 80      | no
`bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no
`bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no

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
`discovery.ionos`:

| Hierarchy           | Block             | Description                                              | Required |
| ------------------- | ----------------- | -------------------------------------------------------- | -------- |
| basic_auth          | [basic_auth][]    | Configure basic_auth for authenticating to the endpoint. | no       |
| authorization       | [authorization][] | Configure generic authorization to the endpoint.         | no       |
| oauth2              | [oauth2][]        | Configure OAuth2 for authenticating to the endpoint.     | no       |
| oauth2 > tls_config | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no       |
| tls_config          | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no       |

The `>` symbol indicates deeper levels of nesting. For example,
`oauth2 > tls_config` refers to a `tls_config` block defined inside
an `oauth2` block.

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

| Name      | Type                | Description                                             |
| --------- | ------------------- | ------------------------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from the IONOS Cloud API. |

Each target includes the following labels:

- `__meta_ionos_server_availability_zone`: the availability zone of the server.
- `__meta_ionos_server_boot_cdrom_id`: the ID of the CD-ROM the server is booted from.
- `__meta_ionos_server_boot_image_id`: the ID of the boot image or snapshot the server is booted from.
- `__meta_ionos_server_boot_volume_id`: the ID of the boot volume.
- `__meta_ionos_server_cpu_family`: the CPU family of the server to.
- `__meta_ionos_server_id`: the ID of the server.
- `__meta_ionos_server_ip`: comma separated list of all IPs assigned to the server.
- `__meta_ionos_server_lifecycle`: the lifecycle state of the server resource.
- `__meta_ionos_server_name`: the name of the server.
- `__meta_ionos_server_nic_ip_<nic_name>`: comma separated list of IPs, grouped by the name of each NIC attached to the server.
- `__meta_ionos_server_servers_id`: the ID of the servers the server belongs to.
- `__meta_ionos_server_state`: the execution state of the server.
- `__meta_ionos_server_type`: the type of the server.

## Component health

`discovery.ionos` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.ionos` does not expose any component-specific debug information.

## Debug metrics

`discovery.ionos` does not expose any component-specific debug metrics.

## Example

```alloy
discovery.ionos "example" {
    datacenter_id = "15f67991-0f51-4efc-a8ad-ef1fb31a480c"
}

prometheus.scrape "demo" {
  targets    = discovery.ionos.example.targets
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

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the remote_write API.
- `PASSWORD`: The password to use for authentication to the remote_write API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.ionos` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

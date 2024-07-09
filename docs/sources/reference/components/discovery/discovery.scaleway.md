---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.scaleway/
aliases:
  - ../discovery.scaleway/ # /docs/alloy/latest/reference/components/discovery.scaleway/
description: Learn about discovery.scaleway
title: discovery.scaleway
---

# discovery.scaleway

`discovery.scaleway` discovers targets from [Scaleway instances][instance] and [baremetal services][baremetal].

[instance]: https://www.scaleway.com/en/virtual-instances/
[baremetal]: https://www.scaleway.com/en/bare-metal-servers/

## Usage

```alloy
discovery.scaleway "LABEL" {
    project_id = "SCALEWAY_PROJECT_ID"
    role       = "SCALEWAY_PROJECT_ROLE"
    access_key = "SCALEWAY_ACCESS_KEY"
    secret_key = "SCALEWAY_SECRET_KEY"
}
```

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                                                      | Default                      | Required
-------------------------|---------------------|--------------------------------------------------------------------------------------------------|------------------------------|------------
`project_id`             | `string`            | Scaleway project ID of targets.                                                                  |                              | yes
`role`                   | `string`            | Role of targets to retrieve.                                                                     |                              | yes
`api_url`                | `string`            | Scaleway API URL.                                                                                | `"https://api.scaleway.com"` | no
`zone`                   | `string`            | Availability zone of targets.                                                                    | `"fr-par-1"`                 | no
`access_key`             | `string`            | Access key for the Scaleway API.                                                                 |                              | yes
`secret_key`             | `secret`            | Secret key for the Scaleway API.                                                                 |                              | conditional
`secret_key_file`        | `string`            | Path to file containing secret key for the Scaleway API.                                         |                              | conditional
`name_filter`            | `string`            | Name filter to apply against the listing request.                                                |                              | no
`tags_filter`            | `list(string)`      | List of tags to search for.                                                                      |                              | no
`refresh_interval`       | `duration`          | Frequency to rediscover targets.                                                                 | `"60s"`                      | no
`port`                   | `number`            | Default port on servers to associate with generated targets.                                     | `80`                         | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |                              | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |                              | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`                      | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |                              | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`                       | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`                       | no

The `role` argument determines what type of Scaleway machines to discover.
It must be set to one of the following:

* `"baremetal"`: Discover [baremetal][] Scaleway machines.
* `"instance"`: Discover virtual Scaleway [instances][instance].

The `name_filter` and `tags_filter` arguments can be used to filter the set of discovered servers.
`name_filter` returns machines matching a specific name, while `tags_filter` returns machines who contain _all_ the tags listed in the `tags_filter` argument.

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

The following blocks are supported inside the definition of
`discovery.scaleway`:

Hierarchy  | Block          | Description                                            | Required
-----------|----------------|--------------------------------------------------------|---------
tls_config | [tls_config][] | Configure TLS settings for connecting to the endpoint. | no

The `>` symbol indicates deeper levels of nesting.
For example, `oauth2 > tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

[tls_config]: #tls_config-block

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                | Description
----------|---------------------|-----------------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the Consul catalog API.

When `role` is `baremetal`, discovered targets include the following labels:

* `__meta_scaleway_baremetal_id`: ID of the server.
* `__meta_scaleway_baremetal_public_ipv4`: Public IPv4 address of the server.
* `__meta_scaleway_baremetal_public_ipv6`: Public IPv6 address of the server.
* `__meta_scaleway_baremetal_name`: Name of the server.
* `__meta_scaleway_baremetal_os_name`: Operating system name of the server.
* `__meta_scaleway_baremetal_os_version`: Operation system version of the server.
* `__meta_scaleway_baremetal_project_id`: Project ID the server belongs to.
* `__meta_scaleway_baremetal_status`: Current status of the server.
* `__meta_scaleway_baremetal_tags`: The list of tags associated with the server concatenated with a `,`.
* `__meta_scaleway_baremetal_type`: Commercial type of the server.
* `__meta_scaleway_baremetal_zone`: Availability zone of the server.

When `role` is `instance`, discovered targets include the following labels:

* `__meta_scaleway_instance_boot_type`: Boot type of the server.
* `__meta_scaleway_instance_hostname`: Hostname of the server.
* `__meta_scaleway_instance_id`: ID of the server.
* `__meta_scaleway_instance_image_arch`: Architecture of the image the server is running.
* `__meta_scaleway_instance_image_id`: ID of the image the server is running.
* `__meta_scaleway_instance_image_name`: Name of the image the server is running.
* `__meta_scaleway_instance_location_cluster_id`: ID of the cluster for the server's location.
* `__meta_scaleway_instance_location_hypervisor_id`: Hypervisor ID for the server's location.
* `__meta_scaleway_instance_location_node_id`: Node ID for the server's location.
* `__meta_scaleway_instance_name`: Name of the server.
* `__meta_scaleway_instance_organization_id`: Organization ID that the server belongs to.
* `__meta_scaleway_instance_private_ipv4`: Private IPv4 address of the server.
* `__meta_scaleway_instance_project_id`: Project ID the server belongs to.
* `__meta_scaleway_instance_public_ipv4`: Public IPv4 address of the server.
* `__meta_scaleway_instance_public_ipv6`: Public IPv6 address of the server.
* `__meta_scaleway_instance_region`: Region of the server.
* `__meta_scaleway_instance_security_group_id`: ID of the security group the server is assigned to.
* `__meta_scaleway_instance_security_group_name`: Name of the security group the server is assigned to.
* `__meta_scaleway_instance_status`: Current status of the server.
* `__meta_scaleway_instance_tags`: The list of tags associated with the server concatenated with a `,`.
* `__meta_scaleway_instance_type`: Commercial type of the server.
* `__meta_scaleway_instance_zone`: Availability zone of the server.

## Component health

`discovery.scaleway` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.scaleway` does not expose any component-specific debug information.

## Debug metrics

`discovery.scaleway` does not expose any component-specific debug metrics.

## Example

```alloy
discovery.scaleway "example" {
    project_id = "SCALEWAY_PROJECT_ID"
    role       = "SCALEWAY_PROJECT_ROLE"
    access_key = "SCALEWAY_ACCESS_KEY"
    secret_key = "SCALEWAY_SECRET_KEY"
}

prometheus.scrape "demo" {
    targets    = discovery.scaleway.example.targets
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

* `SCALEWAY_PROJECT_ID`: The project ID of your Scaleway machines.
* `SCALEWAY_PROJECT_ROLE`: Set to `baremetal` to discover [baremetal][] machines or `instance` to discover [virtual instances][instance].
* `SCALEWAY_ACCESS_KEY`: Your Scaleway API access key.
* `SCALEWAY_SECRET_KEY`: Your Scaleway API secret key.
* `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
* `USERNAME`: The username to use for authentication to the remote_write API.
* `PASSWORD`: The password to use for authentication to the remote_write API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.scaleway` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

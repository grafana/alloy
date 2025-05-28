---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.scaleway/
aliases:
  - ../discovery.scaleway/ # /docs/alloy/latest/reference/components/discovery.scaleway/
description: Learn about discovery.scaleway
labels:
  stage: general-availability
  products:
    - oss
title: discovery.scaleway
---

# `discovery.scaleway`

`discovery.scaleway` discovers targets from [Scaleway instances][instance] and [bare metal services][bare metal].

[instance]: https://www.scaleway.com/en/virtual-instances/
[bare metal]: https://www.scaleway.com/en/bare-metal-servers/

## Usage

```alloy
discovery.scaleway "<LABEL>" {
    project_id = "<SCALEWAY_PROJECT_ID>"
    role       = "<SCALEWAY_PROJECT_ROLE>"
    access_key = "<SCALEWAY_ACCESS_KEY>"
    secret_key = "<SCALEWAY_SECRET_KEY>"
}
```

## Arguments

You can use the following arguments with `discovery.scaleway`:

| Name                     | Type                | Description                                                                                      | Default                      | Required    |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------- | ----------- |
| `access_key`             | `string`            | Access key for the Scaleway API.                                                                 |                              | yes         |
| `project_id`             | `string`            | Scaleway project ID of targets.                                                                  |                              | yes         |
| `role`                   | `string`            | Role of targets to retrieve.                                                                     |                              | yes         |
| `secret_key_file`        | `string`            | Path to file containing secret key for the Scaleway API.                                         |                              | conditional |
| `secret_key`             | `secret`            | Secret key for the Scaleway API.                                                                 |                              | conditional |
| `api_url`                | `string`            | Scaleway API URL.                                                                                | `"https://api.scaleway.com"` | no          |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`                       | no          |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`                       | no          |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |                              | no          |
| `name_filter`            | `string`            | Name filter to apply against the listing request.                                                |                              | no          |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |                              | no          |
| `port`                   | `number`            | Default port on servers to associate with generated targets.                                     | `80`                         | no          |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |                              | no          |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`                      | no          |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |                              | no          |
| `refresh_interval`       | `duration`          | Frequency to rediscover targets.                                                                 | `"60s"`                      | no          |
| `tags_filter`            | `list(string)`      | List of tags to search for.                                                                      |                              | no          |
| `zone`                   | `string`            | Availability zone of targets.                                                                    | `"fr-par-1"`                 | no          |

The `role` argument determines what type of Scaleway machines to discover.
It must be set to one of the following:

* `"baremetal"`: Discover [bare metal][] Scaleway machines.
* `"instance"`: Discover virtual Scaleway [instances][instance].

The `name_filter` and `tags_filter` arguments can be used to filter the set of discovered servers.
`name_filter` returns machines matching a specific name, while `tags_filter` returns machines who contain _all_ the tags listed in the `tags_filter` argument.

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `discovery.scaleway`:

| Block                      | Description                                            | Required |
| -------------------------- | ------------------------------------------------------ | -------- |
| [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint. | no       |

[tls_config]: #tls_config

### `tls_config`

The `tls_config` block configures TLS settings for connecting to the endpoint.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                                                |
| --------- | ------------------- | ---------------------------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from the Consul catalog API. |

When `role` is `baremetal`, discovered targets include the following labels:

* `__meta_scaleway_baremetal_id`: The ID of the server.
* `__meta_scaleway_baremetal_name`: The name of the server.
* `__meta_scaleway_baremetal_os_name`: The operating system name of the server.
* `__meta_scaleway_baremetal_os_version`: The operating system version of the server.
* `__meta_scaleway_baremetal_project_id`: The project ID the server belongs to.
* `__meta_scaleway_baremetal_public_ipv4`: The public IPv4 address of the server.
* `__meta_scaleway_baremetal_public_ipv6`: The public IPv6 address of the server.
* `__meta_scaleway_baremetal_status`: The current status of the server.
* `__meta_scaleway_baremetal_tags`: The list of tags associated with the server concatenated with a `,`.
* `__meta_scaleway_baremetal_type`: The commercial type of the server.
* `__meta_scaleway_baremetal_zone`: The availability zone of the server.

When `role` is `instance`, discovered targets include the following labels:

* `__meta_scaleway_instance_boot_type`: The boot type of the server.
* `__meta_scaleway_instance_hostname`: The hostname of the server.
* `__meta_scaleway_instance_id`: The ID of the server.
* `__meta_scaleway_instance_image_arch`: The architecture of the image the server is running.
* `__meta_scaleway_instance_image_id`: The ID of the image the server is running.
* `__meta_scaleway_instance_image_name`: The name of the image the server is running.
* `__meta_scaleway_instance_location_cluster_id`: The ID of the cluster for the server's location.
* `__meta_scaleway_instance_location_hypervisor_id`: The hypervisor ID for the server's location.
* `__meta_scaleway_instance_location_node_id`: The node ID for the server's location.
* `__meta_scaleway_instance_name`: The name of the server.
* `__meta_scaleway_instance_organization_id`: The organization ID that the server belongs to.
* `__meta_scaleway_instance_private_ipv4`: The private IPv4 address of the server.
* `__meta_scaleway_instance_project_id`: The project ID the server belongs to.
* `__meta_scaleway_instance_public_ipv4`: The public IPv4 address of the server.
* `__meta_scaleway_instance_public_ipv6`: The public IPv6 address of the server.
* `__meta_scaleway_instance_region`: The region of the server.
* `__meta_scaleway_instance_security_group_id`: The ID of the security group the server is assigned to.
* `__meta_scaleway_instance_security_group_name`: The name of the security group the server is assigned to.
* `__meta_scaleway_instance_status`: The current status of the server.
* `__meta_scaleway_instance_tags`: The list of tags associated with the server concatenated with a `,`.
* `__meta_scaleway_instance_type`: The commercial type of the server.
* `__meta_scaleway_instance_zone`: The availability zone of the server.

## Component health

`discovery.scaleway` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.scaleway` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.scaleway` doesn't expose any component-specific debug metrics.

## Example

```alloy
discovery.scaleway "example" {
    project_id = "<SCALEWAY_PROJECT_ID>"
    role       = "<SCALEWAY_PROJECT_ROLE>"
    access_key = "<SCALEWAY_ACCESS_KEY>"
    secret_key = "<SCALEWAY_SECRET_KEY>"
}

prometheus.scrape "demo" {
    targets    = discovery.scaleway.example.targets
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

* _`<SCALEWAY_PROJECT_ID>`_: The project ID of your Scaleway machines.
* _`<SCALEWAY_PROJECT_ROLE>`_: Set to `baremetal` to discover [bare metal][] machines or `instance` to discover [virtual instances][instance].
* _`<SCALEWAY_ACCESS_KEY>`_: Your Scaleway API access key.
* _`<SCALEWAY_SECRET_KEY>`_: Your Scaleway API secret key.
* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.scaleway` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

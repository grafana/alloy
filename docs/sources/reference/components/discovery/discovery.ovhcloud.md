---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.ovhcloud/
aliases:
  - ../discovery.ovhcloud/ # /docs/alloy/latest/reference/components/discovery.ovhcloud/
description: Learn about discovery.ovhcloud
labels:
  stage: general-availability
  products:
    - oss
title: discovery.ovhcloud
---

# `discovery.ovhcloud`

`discovery.ovhcloud` discovers scrape targets from the OVHcloud [dedicated servers][] and [VPS][] using their [API][].
{{< param "PRODUCT_NAME" >}} periodically checks the REST endpoint and create a target for every discovered server.
The public IPv4 address is used by default. If there's no IPv4 address, the IPv6 address is used.
This may be changed via relabeling with `discovery.relabel`.
For the OVHcloud [public cloud][] instances you can use `discovery.openstack`.

[API]: https://api.ovh.com/
[public cloud]: https://www.ovhcloud.com/en/public-cloud/
[VPS]: https://www.ovhcloud.com/en/vps/
[Dedicated servers]: https://www.ovhcloud.com/en/bare-metal/

## Usage

```alloy
discovery.ovhcloud "<LABEL>" {
    application_key    = "<APPLICATION_KEY>"
    application_secret = "<APPLICATION_SECRET>"
    consumer_key       = "<CONSUMER_KEY>"
    service            = "<SERVICE>"
}
```

## Arguments

You can use the following arguments with `discovery.ovhcloud`:

| Name                 | Type       | Description                                     | Default    | Required |
| -------------------- | ---------- | ----------------------------------------------- | ---------- | -------- |
| `application_key`    | `string`   | [API][] application key.                        |            | yes      |
| `application_secret` | `secret`   | [API][] application secret.                     |            | yes      |
| `consumer_key`       | `secret`   | [API][] consumer key.                           |            | yes      |
| `service`            | `string`   | Service of the targets to retrieve.             |            | yes      |
| `endpoint`           | `string`   | [API][] endpoint.                               | `"ovh-eu"` | no       |
| `refresh_interval`   | `duration` | Refresh interval to re-read the resources list. | `"60s"`    | no       |

`service` must be either `vps` or `dedicated_server`.

`endpoint` must be one of the [supported API endpoints][supported-apis].

[supported-apis]: https://github.com/ovh/go-ovh#supported-apis

## Blocks

The `discovery.ovhcloud` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                                          |
| --------- | ------------------- | ---------------------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from the OVHcloud API. |

Multiple meta labels are available on `targets` and can be used by the `discovery.relabel` component.

[VPS][] meta labels:

* `__meta_ovhcloud_vps_cluster`: The cluster of the server.
* `__meta_ovhcloud_vps_datacenter`: The data center of the server.
* `__meta_ovhcloud_vps_disk`: The disk of the server.
* `__meta_ovhcloud_vps_display_name`: The display name of the server.
* `__meta_ovhcloud_vps_ipv4`: The IPv4 of the server.
* `__meta_ovhcloud_vps_ipv6`: The IPv6 of the server.
* `__meta_ovhcloud_vps_keymap`: The KVM keyboard layout of the server.
* `__meta_ovhcloud_vps_maximum_additional_ip`: The maximum additional IP addresses of the server.
* `__meta_ovhcloud_vps_memory_limit`: The memory limit of the server.
* `__meta_ovhcloud_vps_memory`: The memory of the server.
* `__meta_ovhcloud_vps_monitoring_ip_blocks`: The monitoring IP blocks of the server.
* `__meta_ovhcloud_vps_name`: The name of the server.
* `__meta_ovhcloud_vps_netboot_mode`: The netboot mode of the server.
* `__meta_ovhcloud_vps_offer_type`: The offer type of the server.
* `__meta_ovhcloud_vps_offer`: The offer of the server.
* `__meta_ovhcloud_vps_state`: The state of the server.
* `__meta_ovhcloud_vps_vcore`: The number of virtual cores of the server.
* `__meta_ovhcloud_vps_version`: The version of the server.
* `__meta_ovhcloud_vps_zone`: The zone of the server.

[Dedicated servers][] meta labels:

* `__meta_ovhcloud_dedicated_server_commercial_range`: The commercial range of the server.
* `__meta_ovhcloud_dedicated_server_datacenter`: The data center of the server.
* `__meta_ovhcloud_dedicated_server_ipv4`: The IPv4 of the server.
* `__meta_ovhcloud_dedicated_server_ipv6`: The IPv6 of the server.
* `__meta_ovhcloud_dedicated_server_link_speed`: The link speed of the server.
* `__meta_ovhcloud_dedicated_server_name`: The name of the server.
* `__meta_ovhcloud_dedicated_server_no_intervention`: Whether datacenter intervention is disabled for the server.
* `__meta_ovhcloud_dedicated_server_os`: The operating system of the server.
* `__meta_ovhcloud_dedicated_server_rack`: The rack of the server.
* `__meta_ovhcloud_dedicated_server_reverse`: The reverse DNS name of the server.
* `__meta_ovhcloud_dedicated_server_server_id`: The ID of the server.
* `__meta_ovhcloud_dedicated_server_state`: The state of the server.
* `__meta_ovhcloud_dedicated_server_support_level`: The support level of the server.

## Component health

`discovery.ovhcloud` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.ovhcloud` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.ovhcloud` doesn't expose any component-specific debug metrics.

## Example

```alloy
discovery.ovhcloud "example" {
    application_key    = "<APPLICATION_KEY>"
    application_secret = "<APPLICATION_SECRET>"
    consumer_key       = "<CONSUMER_KEY>"
    service            = "<SERVICE>"
}

prometheus.scrape "demo" {
    targets    = discovery.ovhcloud.example.targets
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

* _`<APPLICATION_KEY>`_: The OVHcloud [API][] application key.
* _`<APPLICATION_SECRET>`_: The OVHcloud [API][] application secret.
* _`<CONSUMER_KEY>`_: The OVHcloud [API][] consumer key.
* _`<SERVICE>`_: The OVHcloud service of the targets to retrieve.
* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.ovhcloud` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

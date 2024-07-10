---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.ovhcloud/
aliases:
  - ../discovery.ovhcloud/ # /docs/alloy/latest/reference/components/discovery.ovhcloud/
description: Learn about discovery.ovhcloud
title: discovery.ovhcloud
---

# discovery.ovhcloud

`discovery.ovhcloud` discovers scrape targets from OVHcloud's [dedicated servers][] and [VPS][] using their [API][].
{{< param "PRODUCT_NAME" >}} will periodically check the REST endpoint and create a target for every discovered server.
The public IPv4 address will be used by default - if there's none, the IPv6 address will be used.
This may be changed via relabeling with `discovery.relabel`.
For OVHcloud's [public cloud][] instances you can use `discovery.openstack`.

[API]: https://api.ovh.com/
[public cloud]: https://www.ovhcloud.com/en/public-cloud/
[VPS]: https://www.ovhcloud.com/en/vps/
[Dedicated servers]: https://www.ovhcloud.com/en/bare-metal/

## Usage

```alloy
discovery.ovhcloud "LABEL" {
    application_key    = APPLICATION_KEY
    application_secret = APPLICATION_SECRET
    consumer_key       = CONSUMER_KEY
    service            = SERVICE
}
```

## Arguments

The following arguments are supported:

Name               | Type           | Description                                                    | Default       | Required
------------------ | -------------- | -------------------------------------------------------------- | ------------- | --------
application_key    | `string`       | [API][] application key.                                       |               | yes
application_secret | `secret`       | [API][] application secret.                                    |               | yes
consumer_key       | `secret`       | [API][] consumer key.                                          |               | yes
endpoint           | `string`       | [API][] endpoint.                                              | "ovh-eu"      | no
refresh_interval   | `duration`     | Refresh interval to re-read the resources list.                | "60s"         | no
service            | `string`       | Service of the targets to retrieve.                            |               | yes

`endpoint` must be one of the [supported API endpoints][supported-apis].

`service` must be either `vps` or `dedicated_server`.

[supported-apis]: https://github.com/ovh/go-ovh#supported-apis

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                | Description
----------|---------------------|-----------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the OVHcloud API.

Multiple meta labels are available on `targets` and can be used by the `discovery.relabel` component.

[VPS][] meta labels:
* `__meta_ovhcloud_vps_cluster`: the cluster of the server.
* `__meta_ovhcloud_vps_datacenter`: the datacenter of the server.
* `__meta_ovhcloud_vps_disk`: the disk of the server.
* `__meta_ovhcloud_vps_display_name`: the display name of the server.
* `__meta_ovhcloud_vps_ipv4`: the IPv4 of the server.
* `__meta_ovhcloud_vps_ipv6`: the IPv6 of the server.
* `__meta_ovhcloud_vps_keymap`: the KVM keyboard layout of the server.
* `__meta_ovhcloud_vps_maximum_additional_ip`: the maximum additional IPs of the server.
* `__meta_ovhcloud_vps_memory_limit`: the memory limit of the server.
* `__meta_ovhcloud_vps_memory`: the memory of the server.
* `__meta_ovhcloud_vps_monitoring_ip_blocks`: the monitoring IP blocks of the server.
* `__meta_ovhcloud_vps_name`: the name of the server.
* `__meta_ovhcloud_vps_netboot_mode`: the netboot mode of the server.
* `__meta_ovhcloud_vps_offer_type`: the offer type of the server.
* `__meta_ovhcloud_vps_offer`: the offer of the server.
* `__meta_ovhcloud_vps_state`: the state of the server.
* `__meta_ovhcloud_vps_vcore`: the number of virtual cores of the server.
* `__meta_ovhcloud_vps_version`: the version of the server.
* `__meta_ovhcloud_vps_zone`: the zone of the server.

[Dedicated servers][] meta labels:
* `__meta_ovhcloud_dedicated_server_commercial_range`: the commercial range of the server.
* `__meta_ovhcloud_dedicated_server_datacenter`: the datacenter of the server.
* `__meta_ovhcloud_dedicated_server_ipv4`: the IPv4 of the server.
* `__meta_ovhcloud_dedicated_server_ipv6`: the IPv6 of the server.
* `__meta_ovhcloud_dedicated_server_link_speed`: the link speed of the server.
* `__meta_ovhcloud_dedicated_server_name`: the name of the server.
* `__meta_ovhcloud_dedicated_server_os`: the operating system of the server.
* `__meta_ovhcloud_dedicated_server_rack`: the rack of the server.
* `__meta_ovhcloud_dedicated_server_reverse`: the reverse DNS name of the server.
* `__meta_ovhcloud_dedicated_server_server_id`: the ID of the server.
* `__meta_ovhcloud_dedicated_server_state`: the state of the server.
* `__meta_ovhcloud_dedicated_server_support_level`: the support level of the server.

## Component health

`discovery.ovhcloud` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.ovhcloud` does not expose any component-specific debug information.

## Debug metrics

`discovery.ovhcloud` does not expose any component-specific debug metrics.

## Example

```alloy
discovery.ovhcloud "example" {
	application_key    = APPLICATION_KEY
	application_secret = APPLICATION_SECRET
	consumer_key       = CONSUMER_KEY
	service            = SERVICE
}

prometheus.scrape "demo" {
	targets    = discovery.ovhcloud.example.targets
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
  - `APPLICATION_KEY`: The OVHcloud [API][] application key.
  - `APPLICATION_SECRET`: The OVHcloud [API][] application secret.
  - `CONSUMER_KEY`: The OVHcloud [API][] consumer key.
  - `SERVICE`: The OVHcloud service of the targets to retrieve.
  - `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
  - `USERNAME`: The username to use for authentication to the remote_write API.
  - `PASSWORD`: The password to use for authentication to the remote_write API.


<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.ovhcloud` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

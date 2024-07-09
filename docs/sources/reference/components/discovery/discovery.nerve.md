---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.nerve/
aliases:
  - ../discovery.nerve/ # /docs/alloy/latest/reference/components/discovery.nerve/
description: Learn about discovery.nerve
title: discovery.nerve
---

# discovery.nerve

`discovery.nerve` discovers [airbnb/nerve][] targets stored in Zookeeper.

[airbnb/nerve]: https://github.com/airbnb/nerve

## Usage

```alloy
discovery.nerve "LABEL" {
	servers = [SERVER_1, SERVER_2]
	paths   = [PATH_1, PATH_2]
}
```

## Arguments

The following arguments are supported:

Name               | Type           | Description                          | Default       | Required
------------------ | -------------- | ------------------------------------ | ------------- | --------
`servers`          | `list(string)` | The Zookeeper servers.               |               | yes
`paths`            | `list(string)` | The paths to look for targets at.    |               | yes
`timeout`          | `duration`     | The timeout to use.                  | `"10s"`       | no


Each element in the `path` list can either point to a single service, or to the root of a tree of services.

## Blocks

The `discovery.nerve` component does not support any blocks, and is configured fully through arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                | Description
--------- | ------------------- | -----------
`targets` | `list(map(string))` | The set of targets discovered from Nerve's API.

The following meta labels are available on targets and can be used by the discovery.relabel component
* `__meta_nerve_path`: the full path to the endpoint node in Zookeeper
* `__meta_nerve_endpoint_host`: the host of the endpoint
* `__meta_nerve_endpoint_port`: the port of the endpoint
* `__meta_nerve_endpoint_name`: the name of the endpoint

## Component health

`discovery.nerve` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.nerve` does not expose any component-specific debug information.

## Debug metrics

`discovery.nerve` does not expose any component-specific debug metrics.

## Example

```alloy
discovery.nerve "example" {
	servers = ["localhost"]
	paths   = ["/monitoring"]
	timeout = "1m"
}
prometheus.scrape "demo" {
	targets    = discovery.nerve.example.targets
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

`discovery.nerve` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.triton/
aliases:
  - ../discovery.triton/ # /docs/alloy/latest/reference/components/discovery.triton/
description: Learn about discovery.triton
title: discovery.triton
---

# discovery.triton

`discovery.triton` discovers [Triton][] Container Monitors and exposes them as targets.

[Triton]: https://www.tritondatacenter.com

## Usage

```alloy
discovery.triton "LABEL" {
	account    = ACCOUNT
	dns_suffix = DNS_SUFFIX
	endpoint   = ENDPOINT
}
```

## Arguments

The following arguments are supported:

Name               | Type           | Description                                         | Default       | Required
------------------ | -------------- | --------------------------------------------------- | ------------- | --------
`account`          | `string`       | The account to use for discovering new targets.     |               | yes
`role`             | `string`       | The type of targets to discover.                    | `"container"` | no
`dns_suffix`       | `string`       | The DNS suffix that is applied to the target.       |               | yes
`endpoint`         | `string`       | The Triton discovery endpoint. 					  |               | yes
`groups`           | `list(string)` | A list of groups to retrieve targets from.          |               | no
`port`             | `int`          | The port to use for discovery and metrics scraping. | `9163`        | no
`refresh_interval` | `duration`     | The refresh interval for the list of targets.       | `60s`         | no
`version`          | `int`          | The Triton discovery API version.                   | `1`           | no

`role` can be set to:
* `"container"` to discover virtual machines (SmartOS zones, lx/KVM/bhyve branded zones) running on Triton
* `"cn"` to discover compute nodes (servers/global zones) making up the Triton infrastructure

`groups` is only supported when `role` is set to `"container"`.
If omitted all containers owned by the requesting account are scraped.

## Blocks
The following blocks are supported inside the definition of
`discovery.triton`:

Hierarchy  | Block          | Description                                       | Required
-----------|----------------|---------------------------------------------------|---------
tls_config | [tls_config][] | TLS configuration for requests to the Triton API. | no

[tls_config]: #tls_config-block

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                | Description
----------|---------------------|---------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the Triton API.

When `role` is set to `"container"`, each target includes the following labels:

* `__meta_triton_groups`: The list of groups belonging to the target joined by a comma separator.
* `__meta_triton_machine_alias`: The alias of the target container.
* `__meta_triton_machine_brand`: The brand of the target container.
* `__meta_triton_machine_id`: The UUID of the target container.
* `__meta_triton_machine_image`: The target container's image type.
* `__meta_triton_server_id`: The server UUID the target container is running on.

When `role` is set to `"cn"` each target includes the following labels:

* `__meta_triton_machine_alias`: The hostname of the target (requires triton-cmon 1.7.0 or newer).
* `__meta_triton_machine_id`: The UUID of the target.

## Component health

`discovery.triton` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.triton` does not expose any component-specific debug information.

## Debug metrics

`discovery.triton` does not expose any component-specific debug metrics.

## Example

```alloy
discovery.triton "example" {
	account    = TRITON_ACCOUNT
	dns_suffix = TRITON_DNS_SUFFIX
	endpoint   = TRITON_ENDPOINT
}

prometheus.scrape "demo" {
	targets    = discovery.triton.example.targets
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
  - `TRITON_ACCOUNT`: Your Triton account.
  - `TRITON_DNS_SUFFIX`: Your Triton DNS suffix.
  - `TRITON_ENDPOINT`: Your Triton endpoint.
  - `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
  - `USERNAME`: The username to use for authentication to the remote_write API.
  - `PASSWORD`: The password to use for authentication to the remote_write API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.triton` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

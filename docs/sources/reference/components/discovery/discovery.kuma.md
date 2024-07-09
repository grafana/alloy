---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.kuma/
aliases:
  - ../discovery.kuma/ # /docs/alloy/latest/reference/components/discovery.kuma/
description: Learn about discovery.kuma
title: discovery.kuma
---

# discovery.kuma

`discovery.kuma` discovers scrape target from the [Kuma][] control plane.

[Kuma]: https://kuma.io/

## Usage

```alloy
discovery.kuma "LABEL" {
    server = SERVER
}
```

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                                                      | Default | Required
-------------------------|---------------------|--------------------------------------------------------------------------------------------------|---------|---------
`server`                 | `string`            | Address of the Kuma Control Plane's MADS xDS server.                                             |         | yes
`refresh_interval`       | `duration`          | The time to wait between polling update requests.                                                | `"30s"` | no
`fetch_timeout`          | `duration`          | The time after which the monitoring assignments are refreshed.                                   | `"2m"`  | no
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

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

The following blocks are supported inside the definition of
`discovery.kuma`:

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
----------|---------------------|-------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the Kuma API.

The following meta labels are available on targets and can be used by the discovery.relabel component:
* `__meta_kuma_mesh`: the name of the proxy's Mesh
* `__meta_kuma_dataplane`: the name of the proxy
* `__meta_kuma_service`: the name of the proxy's associated Service
* `__meta_kuma_label_<tagname>`: each tag of the proxy

## Component health

`discovery.kuma` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.kuma` does not expose any component-specific debug information.

## Debug metrics

`discovery.kuma` does not expose any component-specific debug metrics.

## Example

```alloy
discovery.kuma "example" {
    server     = "http://kuma-control-plane.kuma-system.svc:5676"
}
prometheus.scrape "demo" {
	targets    = discovery.kuma.example.targets
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

`discovery.kuma` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.puppetdb/
aliases:
  - ../discovery.puppetdb/ # /docs/alloy/latest/reference/components/discovery.puppetdb/
description: Learn about discovery.puppetdb
title: discovery.puppetdb
---

# discovery.puppetdb

`discovery.puppetdb` allows you to retrieve scrape targets from [PuppetDB](https://www.puppet.com/docs/puppetdb/7/overview.html) resources.

This SD discovers resources and will create a target for each resource returned by the API.

The resource address is the `certname` of the resource, and can be changed during relabeling.

The queries for this component are expected to be valid [PQL (Puppet Query Language)](https://puppet.com/docs/puppetdb/latest/api/query/v4/pql.html).

## Usage

```alloy
discovery.puppetdb "LABEL" {
  url = PUPPET_SERVER
}
```

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                   | Default | Required
------------------------ | ------------------- | ------------------------------------------------------------- | ------- | --------
`url`                    | `string`            | The URL of the PuppetDB root query endpoint.                  |         | yes
`query`                  | `string`            | Puppet Query Language (PQL) query. Only resources are supported. |      | yes
`include_parameters`     | `bool`              | Whether to include the parameters as meta labels. Due to the differences between parameter types and Prometheus labels, some parameters might not be rendered. The format of the parameters might also change in future releases. Make sure that you don't have secrets exposed as parameters if you enable this. | `false` | no
`port`                   | `int`               | The port to scrape metrics from.                              | `80`    | no
`refresh_interval`       | `duration`          | Frequency to refresh targets.                                 | `"30s"` | no
`bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.          |         | no
`bearer_token`           | `secret`            | Bearer token to authenticate with.                            |         | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                      | `true`  | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.  | `true`  | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                          |         | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. | | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.         | `false` | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests. |         | no

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
`discovery.puppetdb`:

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
----------|---------------------|---------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from puppetdb.

Each target includes the following labels:

* `__meta_puppetdb_query`: the Puppet Query Language (PQL) query.
* `__meta_puppetdb_certname`: the name of the node associated with the resourcet.
* `__meta_puppetdb_resource`: a SHA-1 hash of the resource’s type, title, and parameters, for identification.
* `__meta_puppetdb_type`: the resource type.
* `__meta_puppetdb_title`: the resource title.
* `__meta_puppetdb_exported`: whether the resource is exported ("true" or "false").
* `__meta_puppetdb_tags`: comma separated list of resource tags.
* `__meta_puppetdb_file`: the manifest file in which the resource was declared.
* `__meta_puppetdb_environment`: the environment of the node associated with the resource.
* `__meta_puppetdb_parameter_<parametername>`: the parameters of the resource.

## Component health

`discovery.puppetdb` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.puppetdb` does not expose any component-specific debug information.

## Debug metrics

`discovery.puppetdb` does not expose any component-specific debug metrics.

## Example

This example discovers targets from puppetdb for all the servers that have a specific package defined:

```alloy
discovery.puppetdb "example" {
	url   = "http://puppetdb.local:8080"
	query = "resources { type = \"Package\" and title = \"node_exporter\" }"
	port  = 9100
}

prometheus.scrape "demo" {
	targets    = discovery.puppetdb.example.targets
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

`discovery.puppetdb` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

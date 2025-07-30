---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.puppetdb/
aliases:
  - ../discovery.puppetdb/ # /docs/alloy/latest/reference/components/discovery.puppetdb/
description: Learn about discovery.puppetdb
labels:
  stage: general-availability
  products:
    - oss
title: discovery.puppetdb
---

# `discovery.puppetdb`

`discovery.puppetdb` allows you to retrieve scrape targets from [PuppetDB](https://www.puppet.com/docs/puppetdb/7/overview.html) resources.

This SD discovers resources and creates a target for each resource returned by the API.

The resource address is the `certname` of the resource, and can be changed during relabeling.

The queries for this component are expected to be valid [PQL (Puppet Query Language)](https://puppet.com/docs/puppetdb/latest/api/query/v4/pql.html).

## Usage

```alloy
discovery.puppetdb "<LABEL>" {
  url = "<PUPPET_SERVER>"
}
```

## Arguments

You can use the following arguments with `discovery.puppetdb`:

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `query`                  | `string`            | Puppet Query Language (PQL) query. Only resources are supported.                                 |         | yes      |
| `url`                    | `string`            | The URL of the PuppetDB root query endpoint.                                                     |         | yes      |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |         | no       |
| `include_parameters`     | `bool`              | Whether to include the parameters as meta labels. Due to the differences between parameter types and Prometheus labels, some parameters might not be rendered. The format of the parameters might also change in future releases. Make sure that you don't have secrets exposed as parameters if you enable this.  | `false` | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `port`                   | `int`               | The port to scrape metrics from.                                                                 | `80`    | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |
| `refresh_interval`       | `duration`          | Frequency to refresh targets.                                                                    | `"30s"` | no       |

 At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments] argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

[arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `discovery.puppetdb`:

| Block                                 | Description                                                | Required |
| ------------------------------------- | ---------------------------------------------------------- | -------- |
| [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint. | no       |
| [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.     | no       |
| [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.     | no       |

The > symbol indicates deeper levels of nesting.
For example, `oauth2` > `tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

[authorization]: #authorization
[basic_auth]: #basic_auth
[oauth2]: #oauth2
[tls_config]: #tls_config

### `authorization`

The `authorization` block configures generic authorization to the endpoint.

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

The `basic_auth` block configures basic authentication to the endpoint.

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

The `oauth2` block configures OAuth 2.0 authentication to the endpoint.

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

The `tls_config` block configures TLS settings for connecting to the endpoint.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                                  |
| --------- | ------------------- | -------------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from PuppetDB. |

Each target includes the following labels:

* `__meta_puppetdb_certname`: The name of the node associated with the resource.
* `__meta_puppetdb_environment`: The environment of the node associated with the resource.
* `__meta_puppetdb_exported`: Whether the resource is exported, either `true` or `false`.
* `__meta_puppetdb_file`: The manifest file in which the resource was declared.
* `__meta_puppetdb_parameter_<parametername>`: The parameters of the resource.
* `__meta_puppetdb_query`: The Puppet Query Language (PQL) query.
* `__meta_puppetdb_resource`: A SHA-1 hash of the resource's type, title, and parameters, for identification.
* `__meta_puppetdb_tags`: A comma separated list of resource tags.
* `__meta_puppetdb_title`: The resource title.
* `__meta_puppetdb_type`: The resource type.

## Component health

`discovery.puppetdb` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.puppetdb` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.puppetdb` doesn't expose any component-specific debug metrics.

## Example

This example discovers targets from PuppetDB for all the servers that have a specific package defined:

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
        url = "<PROMETHEUS_REMOTE_WRITE_URL>"

        basic_auth {
            username = "<USERNAME>"
            password = "<PASSWORD>"
        }
    }
}
```

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.puppetdb` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

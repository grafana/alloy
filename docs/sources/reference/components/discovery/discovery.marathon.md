---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.marathon/
aliases:
  - ../discovery.marathon/ # /docs/alloy/latest/reference/components/discovery.marathon/
description: Learn about discovery.marathon
labels:
  stage: general-availability
  products:
    - oss
title: discovery.marathon
---

# `discovery.marathon`

`discovery.marathon` allows you to retrieve scrape targets from [Marathon's](https://mesosphere.github.io/marathon/) Service API.

## Usage

```alloy
discovery.marathon "<LABEL>" {
  servers = ["<MARATHON_SERVER1>", "<MARATHON_SERVER2>"...]
}
```

## Arguments

You can use the following arguments with `discovery.marathon`:

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `servers`                | `list(string)`      | List of Marathon servers.                                                                        |         | yes      |
| `auth_token_file`        | `string`            | File containing an auth token to authenticate with.                                              |         | no       |
| `auth_token`             | `secret`            | Auth token to authenticate with.                                                                 |         | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |         | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |         | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |         | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |         | no       |
| `refresh_interval`       | `duration`          | Interval at which to refresh the list of targets.                                                | `"30s"` | no       |

 At most, one of the following can be provided:

* [`auth_token_file`][arguments] argument
* [`auth_token`][arguments] argument
* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][arguments] argument
* [`bearer_token`][arguments] argument
* [`oauth2`][oauth2] block

[arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `discovery.marathon`:

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

The `oauth` block configures OAuth 2.0 authentication to the endpoint.

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

The `tls_config` block configures TLS settings for connecting to the endpoint.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                | Description                                              |
| --------- | ------------------- | -------------------------------------------------------- |
| `targets` | `list(map(string))` | The set of targets discovered from the Marathon servers. |

Each target includes the following labels:

* `__meta_marathon_app_label_<labelname>`: Any Marathon labels attached to the app.
* `__meta_marathon_app`: The name of the app, with slashes replaced by dashes.
* `__meta_marathon_image`: The name of the Docker image used, if available.
* `__meta_marathon_port_definition_label_<labelname>`: The port definition labels.
* `__meta_marathon_port_index`: The port index number, for example 1 for PORT1.
* `__meta_marathon_port_mapping_label_<labelname>`: The port mapping labels.
* `__meta_marathon_task`: The ID of the Apache Mesos task.

## Component health

`discovery.marathon` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.marathon` doesn't expose any component-specific debug information.

## Debug metrics

`discovery.marathon` doesn't expose any component-specific debug metrics.

## Example

This example discovers targets from a Marathon server:

```alloy
discovery.marathon "example" {
  servers = ["localhost:8500"]
}

prometheus.scrape "demo" {
  targets    = discovery.marathon.example.targets
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

`discovery.marathon` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

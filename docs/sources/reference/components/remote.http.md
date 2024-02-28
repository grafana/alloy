---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/remote.http/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/remote.http/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/remote.http/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/remote.http/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/remote.http/
description: Learn about remote.http
title: remote.http
---

# remote.http

`remote.http` exposes the response body of a URL to other components. The URL
is polled for changes so that the most recent content is always available.

The most common use of `remote.http` is to load discovery targets from an HTTP
server.

Multiple `remote.http` components can be specified by giving them different
labels.

## Usage

```river
remote.http "LABEL" {
  url = "URL_TO_POLL"
}
```

## Arguments

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`url` | `string` | URL to poll. | | yes
`method` | `string` | Define HTTP method for the request | `"GET"` | no
`headers` | `map(string)` | Custom headers for the request. | `{}` | no
`body`    | `string`      | The request body. | `""` | no
`poll_frequency` | `duration` | Frequency to poll the URL. | `"1m"` | no
`poll_timeout` | `duration` | Timeout when polling the URL. | `"10s"` | no
`is_secret` | `bool` | Whether the response body should be treated as a secret. | false | no

When `remote.http` performs a poll operation, an HTTP `GET` request is made
against the URL specified by the `url` argument. A poll is triggered by the
following:

* When the component first loads.
* Every time the component's arguments get re-evaluated.
* At the frequency specified by the `poll_frequency` argument.

The poll is successful if the URL returns a `200 OK` response code. All other
response codes are treated as errors and mark the component as unhealthy. After
a successful poll, the response body from the URL is exported.

[secret]: {{< relref "../../concepts/config-language/expressions/types_and_values.md#secrets" >}}

## Blocks

The following blocks are supported inside the definition of `remote.http`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
client | [client][] | HTTP client settings when connecting to the endpoint. | no
client > basic_auth | [basic_auth][] | Configure basic_auth for authenticating to the endpoint. | no
client > authorization | [authorization][] | Configure generic authorization to the endpoint. | no
client > oauth2 | [oauth2][] | Configure OAuth2 for authenticating to the endpoint. | no
client > oauth2 > tls_config | [tls_config][] | Configure TLS settings for connecting to the endpoint. | no
client > tls_config | [tls_config][] | Configure TLS settings for connecting to the endpoint. | no

The `>` symbol indicates deeper levels of nesting. For example, `client >
basic_auth` refers to an `basic_auth` block defined inside a `client` block.

[client]: #client-block
[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block

### client block

The `client` block configures settings used to connect to the HTTP
server.

{{< docs/shared lookup="flow/reference/components/http-client-config-block.md" source="agent" version="<AGENT_VERSION>" >}}

### basic_auth block

The `basic_auth` block configures basic authentication to use when polling the
configured URL.

{{< docs/shared lookup="flow/reference/components/basic-auth-block.md" source="agent" version="<AGENT_VERSION>" >}}

### authorization block

The `authorization` block configures custom authorization to use when polling
the configured URL.

{{< docs/shared lookup="flow/reference/components/authorization-block.md" source="agent" version="<AGENT_VERSION>" >}}

### oauth2 block

The `oauth2` block configures OAuth2 authorization to use when polling the
configured URL.

{{< docs/shared lookup="flow/reference/components/oauth2-block.md" source="agent" version="<AGENT_VERSION>" >}}

### tls_config block

The `tls_config` block configures TLS settings for connecting to HTTPS servers.

{{< docs/shared lookup="flow/reference/components/tls-config-block.md" source="agent" version="<AGENT_VERSION>" >}}

## Exported fields

The following field is exported and can be referenced by other components:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`content` | `string` or `secret` | The contents of the file. | | no

If the `is_secret` argument was `true`, `content` is a secret type.

## Component health

Instances of `remote.http` report as healthy if the most recent HTTP `GET`
request of the specified URL succeeds.

## Debug information

`remote.http` does not expose any component-specific debug information.

## Debug metrics

`remote.http` does not expose any component-specific debug metrics.

## Example

This example reads a JSON array of objects from an endpoint and uses them as a
set of scrape targets:

```river
remote.http "targets" {
  url = env("MY_TARGETS_URL")
}

prometheus.scrape "default" {
  targets    = json_decode(remote.http.targets.content)
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  client {
    url = env("PROMETHEUS_URL")
  }
}
```

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.http/
aliases:
  - ../discovery.http/ # /docs/alloy/latest/reference/components/discovery.http/
description: Learn about discovery.http
title: discovery.http
---

# discovery.http

`discovery.http` provides a flexible way to define targets by querying an external http endpoint.

It fetches targets from an HTTP endpoint containing a list of zero or more target definitions.
The target must reply with an HTTP 200 response.
The HTTP header Content-Type must be `application/json`, and the body must be valid JSON.

Example response body:

```json
[
  {
    "targets": [ "<host>", ... ],
    "labels": {
      "<labelname>": "<labelvalue>", ...
    }
  },
  ...
]
```

It is possible to use additional fields in the JSON to pass parameters to [prometheus.scrape][] such as the `metricsPath` and `scrape_interval`.

[prometheus.scrape]: ../../prometheus/prometheus.scrape/#technical-details

As an example, the following will provide a target with a custom `metricsPath`, scrape interval, and timeout value:

```json
[
   {
      "labels" : {
         "__metrics_path__" : "/api/prometheus",
         "__scheme__" : "https",
         "__scrape_interval__" : "60s",
         "__scrape_timeout__" : "10s",
         "service" : "custom-api-service"
      },
      "targets" : [
         "custom-api:443"
      ]
   },
]

```

It's also possible to append query parameters to the metrics path with the `__param_<name>` syntax.

For example, the following will call a metrics path of `/health?target_data=prometheus`:

```json
[
   {
      "labels" : {
         "__metrics_path__" : "/health",
         "__scheme__" : "https",
         "__scrape_interval__" : "60s",
         "__scrape_timeout__" : "10s",
         "__param_target_data": "prometheus",
         "service" : "custom-api-service"
      },
      "targets" : [
         "custom-api:443"
      ]
   },
]

```

For more information on the potential labels you can use, see the [prometheus.scrape technical details][prometheus.scrape] section, or the [Prometheus Configuration][] documentation.

[Prometheus Configuration]: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config

## Usage

```alloy
discovery.http "LABEL" {
  url = URL
}
```

## Arguments

The following arguments are supported:

Name                     | Type                | Description                                                                                      | Default | Required
-------------------------|---------------------|--------------------------------------------------------------------------------------------------|---------|---------
`url`                    | `string`            | URL to scrape.                                                                                   |         | yes
`refresh_interval`       | `duration`          | How often to refresh targets.                                                                    | `"60s"` | no
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

[arguments]: #arguments

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

The following blocks are supported inside the definition of
`discovery.http`:

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
----------|---------------------|---------------------------------------------------
`targets` | `list(map(string))` | The set of targets discovered from the filesystem.

Each target includes the following labels:

* `__meta_url`: URL the target was obtained from.

## Component health

`discovery.http` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.http` does not expose any component-specific debug information.

## Debug metrics

* `prometheus_sd_http_failures_total` (counter): Total number of refresh failures.

## Examples

This example will query a URL every 15 seconds and expose targets that it finds:

```alloy
discovery.http "dynamic_targets" {
  url = "https://example.com/scrape_targets"
  refresh_interval = "15s"
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.http` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

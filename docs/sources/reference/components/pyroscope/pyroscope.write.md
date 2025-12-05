---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.write/
aliases:
  - ../pyroscope.write/ # /docs/alloy/latest/reference/components/pyroscope.write/
description: Learn about pyroscope.write
labels:
  stage: general-availability
  products:
    - oss
title: pyroscope.write
---

# `pyroscope.write`

`pyroscope.write` receives performance profiles from other components and forwards them to a series of user-supplied endpoints.
When `pyroscope.write` forwards profiles, all labels starting with double underscore (`__`) are dropped before the data is sent, with the following exceptions:

* `__name__` is preserved because it identifies the profile type.
* `__delta__`is preserved because it's required for delta profiles.

You can specify multiple `pyroscope.write` components by giving them different labels.

## Usage

```alloy
pyroscope.write "<LABEL>" {
  endpoint {
    url = "<PYROSCOPE_URL>"

    ...
  }

  ...
}
```

## Arguments

You can use the following argument with `pyroscope.write`:

| Name              | Type          | Description                                      | Default | Required |
| ----------------- | ------------- | ------------------------------------------------ | ------- | -------- |
| `external_labels` | `map(string)` | Labels to add to profiles sent over the network. |         | no       |

## Blocks

You can use the following blocks with `pyroscope.write`:

| Block                                              | Description                                                | Required |
| -------------------------------------------------- | ---------------------------------------------------------- | -------- |
| [`endpoint`][endpoint]                             | Location to send profiles to.                              | no       |
| `endpoint` > [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| `endpoint` > [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint. | no       |
| `endpoint` > [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `endpoint` > `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.     | no       |
| `endpoint` > [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.     | no       |

The > symbol indicates deeper levels of nesting.
For example, `endpoint` > `basic_auth` refers to a `basic_auth` block defined inside an `endpoint` block.

[endpoint]: #endpoint
[authorization]: #authorization
[basic_auth]: #basic_auth
[oauth2]: #oauth2
[tls_config]: #tls_config

### `endpoint`

The `endpoint` block describes a single location to send profiles to.
Multiple `endpoint` blocks can be provided to send profiles to multiple locations.

The following arguments are supported:

| Name                     | Type                | Description                                                                                      | Default   | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | --------- | -------- |
| `url`                    | `string`            | Full URL to send metrics to.                                                                     |           | yes      |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |           | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |           | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`    | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`    | no       |
| `headers`                | `map(string)`       | Extra headers to deliver with the request.                                                       |           | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |           | no       |
| `max_backoff_period`     | `duration`          | Maximum backoff time between retries.                                                            | `"5m"`    | no       |
| `max_backoff_retries`    | `int`               | Maximum number of retries. 0 to retry infinitely.                                                | `10`      | no       |
| `min_backoff_period`     | `duration`          | Initial backoff time between retries.                                                            | `"500ms"` | no       |
| `name`                   | `string`            | Optional name to identify the endpoint in metrics.                                               |           | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |           | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |           | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`   | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |           | no       |
| `remote_timeout`         | `duration`          | Timeout for requests made to the URL.                                                            | `"10s"`   | no       |

 At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][endpoint] argument
* [`bearer_token`][endpoint] argument
* [`oauth2`][oauth2] block

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

When you provide multiple `endpoint` blocks, profiles are concurrently forwarded to all configured locations.

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type       | Description                                                |
| ---------- | ---------- | ---------------------------------------------------------- |
| `receiver` | `receiver` | A value that other components can use to send profiles to. |

## Component health

`pyroscope.write` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields are kept at their last healthy values.

## Debug information

`pyroscope.write` doesn't expose any component-specific debug information.

## Metrics

`pyroscope.write` exposes the following metrics:

| Metric                                   | Type      | Description                                                      |
|------------------------------------------|-----------|------------------------------------------------------------------|
| `pyroscope_write_sent_bytes_total`       | Counter   | Total number of compressed bytes sent to Pyroscope endpoints.    |
| `pyroscope_write_dropped_bytes_total`    | Counter   | Total number of compressed bytes dropped by Pyroscope endpoints. |
| `pyroscope_write_sent_profiles_total`    | Counter   | Total number of profiles sent to Pyroscope endpoints.            |
| `pyroscope_write_dropped_profiles_total` | Counter   | Total number of profiles dropped by Pyroscope endpoints.         |
| `pyroscope_write_retries_total`          | Counter   | Total number of retries to Pyroscope endpoints.                  |
| `pyroscope_write_latency`                | Histogram | Write latency for sending profiles to Pyroscope endpoints.       |

All metrics include an `endpoint` label identifying the specific endpoint URL. The `pyroscope_write_latency` metric includes an additional `type` label with the following values:

- `push_total`: Total latency for push operations
- `push_endpoint`: Per-endpoint latency for push operations
- `push_downstream`: Downstream request latency for push operations
- `ingest_total`: Total latency for ingest operations
- `ingest_endpoint`: Per-endpoint latency for ingest operations
- `ingest_downstream`: Downstream request latency for ingest operations

## Troubleshoot

{{< docs/shared lookup="reference/components/pyroscope-troubleshooting.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Example

```alloy
pyroscope.write "staging" {
  // Send metrics to a locally running Pyroscope instance.
  endpoint {
    url = "http://pyroscope:4040"
    headers = {
      "X-Scope-OrgID" = "squad-1",
    }
  }
  external_labels = {
    "env" = "staging",
  }
}

pyroscope.scrape "default" {
  targets = [
    {"__address__" = "pyroscope:4040", "service_name"="pyroscope"},
    {"__address__" = "alloy:12345", "service_name"="alloy"},
  ]
  forward_to = [pyroscope.write.staging.receiver]
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`pyroscope.write` has exports that can be consumed by the following components:

- Components that consume [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

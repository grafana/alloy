---
canonical: https://grafana.com/docs/alloy/latest/reference/components/kafka/kafka.tenant_prometheus_write/
description: Learn about kafka.tenant_prometheus_write
labels:
  stage: experimental
  products:
    - oss
title: kafka.tenant_prometheus_write
---

# `kafka.tenant_prometheus_write`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`kafka.tenant_prometheus_write` is a minimal, tenant-aware Prometheus remote write sink for [`kafka.tenant_consumer`][kafka.tenant_consumer].
It sends each batch of metrics synchronously, with the `X-Scope-OrgID` header set to the tenant that `kafka.tenant_consumer` passes along with the metrics.

`kafka.tenant_prometheus_write` has no write-ahead log (WAL), no sharding, and no queue, because Kafka acts as the durable log.
It doesn't support remote write v2.

You can specify multiple `kafka.tenant_prometheus_write` components by giving them different labels.

[kafka.tenant_consumer]: ../kafka.tenant_consumer/

## Usage

```alloy
kafka.tenant_prometheus_write "<LABEL>" {
  endpoint {
    url = "<PROMETHEUS_URL>"
  }
}
```

## Arguments

The `kafka.tenant_prometheus_write` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `kafka.tenant_prometheus_write`:

{{< docs/alloy-config >}}

| Block                                              | Description                                                | Required |
| -------------------------------------------------- | ---------------------------------------------------------- | -------- |
| [`endpoint`][endpoint]                             | Location to send metrics to.                               | yes      |
| `endpoint` > [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| `endpoint` > [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint. | no       |
| `endpoint` > [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `endpoint` > `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.     | no       |
| `endpoint` > [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.     | no       |

The `>` symbol indicates deeper levels of nesting.
For example, `endpoint` > `basic_auth` refers to a `basic_auth` block defined inside an `endpoint` block.

[endpoint]: #endpoint
[authorization]: #authorization
[basic_auth]: #basic_auth
[oauth2]: #oauth2
[tls_config]: #tls_config

{{< /docs/alloy-config >}}

### `endpoint`

The `endpoint` block describes the remote write endpoint to send metrics to.

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
| `max_backoff_period`     | `duration`          | Maximum backoff time between retries.                                                            | `"5s"`    | no       |
| `max_backoff_retries`    | `int`               | Maximum number of attempts to send a request, including the first attempt.                       | `5`       | no       |
| `min_backoff_period`     | `duration`          | Initial backoff time between retries.                                                            | `"100ms"` | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |           | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |           | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`   | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |           | no       |
| `remote_timeout`         | `duration`          | Timeout for each request to the URL.                                                             | `"30s"`   | no       |

At most, one of the following can be provided:

- [`authorization`](#authorization) block
- [`basic_auth`](#basic_auth) block
- [`bearer_token_file`](#endpoint) argument
- [`bearer_token`](#endpoint) argument
- [`oauth2`](#oauth2) block

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

`kafka.tenant_prometheus_write` sends each appender transaction as one remote write v1 request when the transaction is committed.
The commit returns only after the endpoint accepted the request, or after the last attempt failed.
Because of this, `kafka.tenant_consumer` commits a Kafka offset only after the metrics reached the endpoint.

The request's `X-Scope-OrgID` header is set to the tenant from the context the metrics were sent with, and overrides any `X-Scope-OrgID` value in `headers`.
If there is no tenant in the context, the commit fails.
Components such as `prometheus.scrape` don't set a tenant, so use `kafka.tenant_prometheus_write` only as a downstream of `kafka.tenant_consumer`.

`kafka.tenant_prometheus_write` retries requests that fail with a `5xx` or `429` status code or a network error, with exponential backoff between `min_backoff_period` and `max_backoff_period`.
Requests that fail with any other `4xx` status code aren't retried.
If you set `max_backoff_retries` to `0`, requests are retried without a limit.

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

| Name       | Type              | Description                                               |
| ---------- | ----------------- | --------------------------------------------------------- |
| `receiver` | `MetricsReceiver` | A value that other components can use to send metrics to. |

## Component health

`kafka.tenant_prometheus_write` is only reported as unhealthy if given an invalid configuration.

## Debug information

`kafka.tenant_prometheus_write` doesn't expose any component-specific debug information.

## Debug metrics

The following Prometheus metrics are exposed:

| Name                                               | Type      | Description                                                                           |
| -------------------------------------------------- | --------- | ------------------------------------------------------------------------------------- |
| `kafka_tenant_prometheus_write_requests_total`     | `counter` | Total number of remote write requests, by response `code`. `0` means a network error. |
| `kafka_tenant_prometheus_write_sent_samples_total` | `counter` | Total number of samples and histograms successfully sent.                             |

## Example

The following example reads metrics from Kafka with `kafka.tenant_consumer` and sends them to Mimir, using the tenant of each record:

```alloy
local.file "tenant_registry" {
  filename = "/etc/alloy/tenant-registry.yaml"
}

kafka.tenant_consumer "default" {
  registry = local.file.tenant_registry.content

  client {
    brokers = ["kafka-0.kafka:9092"]
  }

  metrics_forward_to = [kafka.tenant_prometheus_write.mimir.receiver]
}

kafka.tenant_prometheus_write "mimir" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"

    basic_auth {
      username = "example-user"
      password = "example-password"
    }
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`kafka.tenant_prometheus_write` has exports that can be consumed by the following components:

- Components that consume [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

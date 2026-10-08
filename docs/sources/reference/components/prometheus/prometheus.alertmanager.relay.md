---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.alertmanager.relay/
description: Receive Alertmanager webhooks and relay alerts to another Alertmanager
labels:
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: prometheus.alertmanager.relay
---

# `prometheus.alertmanager.relay`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`prometheus.alertmanager.relay` receives Alertmanager webhook notifications and sends the contained alerts to another Alertmanager using its `/api/v2/alerts` API.

You can specify multiple `prometheus.alertmanager.relay` components by giving them different labels.

## Usage

```alloy
prometheus.alertmanager.relay "<LABEL>" {
  endpoint {
    url = "<ALERTMANAGER_URL>/api/v2/alerts"
  }
}
```

## Arguments

You can use the following arguments with `prometheus.alertmanager.relay`:

| Name                    | Type       | Description                                      | Default        | Required |
| ----------------------- | ---------- | ------------------------------------------------ | -------------- | -------- |
| `listen_address`        | `string`   | Network address on which to receive webhooks.    | `"127.0.0.1"` | no       |
| `listen_port`           | `int`      | Port on which to receive webhooks.               | `5001`         | no       |
| `max_request_body_size` | `string`   | Maximum size of an incoming webhook request.     | `"1MiB"`       | no       |
| `webhook_path`          | `string`   | HTTP path on which to receive webhook requests.  | `"/webhook"`   | no       |

The configured HTTP endpoint only accepts `POST` requests.
An incoming request must contain one JSON Alertmanager webhook object and at least one alert with labels.
`prometheus.alertmanager.relay` returns a successful response only after the destination Alertmanager accepts the complete alert collection with a `2xx` response.
Alertmanager can retry the webhook when the component returns an error response.

## Blocks

You can use the following blocks with `prometheus.alertmanager.relay`:

{{< docs/alloy-config >}}

| Block                                              | Description                                                | Required |
| -------------------------------------------------- | ---------------------------------------------------------- | -------- |
| [`endpoint`][endpoint]                             | Configure the destination Alertmanager.                    | yes      |
| `endpoint` > [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| `endpoint` > [`basic_auth`][basic_auth]            | Configure basic authentication to the endpoint.            | no       |
| `endpoint` > [`oauth2`][oauth2]                    | Configure OAuth 2.0 authentication to the endpoint.        | no       |
| `endpoint` > `oauth2` > [`tls_config`][tls_config] | Configure TLS for the OAuth 2.0 token endpoint.             | no       |
| `endpoint` > [`tls_config`][tls_config]            | Configure TLS for the destination Alertmanager.            | no       |

[authorization]: #authorization
[basic_auth]: #basic_auth
[endpoint]: #endpoint
[oauth2]: #oauth2
[tls_config]: #tls_config

{{< /docs/alloy-config >}}

### `endpoint`

The required `endpoint` block configures the destination Alertmanager and the HTTP client used to reach it.

You can use the following arguments with `endpoint`:

| Name                     | Type                | Description                                                                                      | Default | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | ------- | -------- |
| `url`                    | `string`            | Complete destination URL.                                                                        |         | yes      |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |         | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |         | no       |
| `enable_http2`           | `bool`              | Whether HTTP/2 is supported for requests.                                                        | `true`  | no       |
| `follow_redirects`       | `bool`              | Whether to follow redirects returned by the destination.                                         | `true`  | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to send with each request.                                                    |         | no       |
| `no_proxy`               | `string`            | Comma-separated addresses and domain names to exclude from proxying.                             |         | no       |
| `proxy_connect_header`   | `map(list(secret))` | Headers to send to proxies during `CONNECT` requests.                                             |         | no       |
| `proxy_from_environment` | `bool`              | Whether to use the proxy URL indicated by environment variables.                                 | `false` | no       |
| `proxy_url`              | `string`            | HTTP proxy through which to send requests.                                                       |         | no       |
| `timeout`                | `duration`          | Maximum duration of one request to the destination Alertmanager.                                 | `"10s"` | no       |

If the `url` argument has no path, `prometheus.alertmanager.relay` uses `/api/v2/alerts`.
If the URL contains a path, the component sends alerts to that path without modifying it.

At most one of the following authentication options can be provided:

* An [`authorization`](#authorization) block.
* A [`basic_auth`](#basic_auth) block.
* The `bearer_token_file` argument.
* The `bearer_token` argument.
* An [`oauth2`](#oauth2) block.

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `authorization`

The `authorization` block configures generic authorization for requests to the destination Alertmanager.

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

The `basic_auth` block configures basic authentication for requests to the destination Alertmanager.

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

The `oauth2` block configures OAuth 2.0 authentication for requests to the destination Alertmanager.

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

The `tls_config` block configures TLS for requests to the destination Alertmanager.
TLS certificate verification remains enabled unless you explicitly set `insecure_skip_verify` to `true`.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`prometheus.alertmanager.relay` doesn't export any fields.

## Component health

`prometheus.alertmanager.relay` reports as healthy while its webhook listener is running.
The component reports as unhealthy if the listener can't bind or the webhook server terminates unexpectedly.
Transient destination failures don't change component health and are reported through logs and debug metrics.

## Debug information

`prometheus.alertmanager.relay` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.alertmanager.relay` exposes the following metrics:

* `prometheus_alertmanager_relay_active_requests` (gauge): Current number of active webhook requests.
* `prometheus_alertmanager_relay_failed_alerts_total` (counter): Total number of received alerts that couldn't be forwarded.
* `prometheus_alertmanager_relay_forwarded_alerts_total` (counter): Total number of alerts successfully forwarded.
* `prometheus_alertmanager_relay_outbound_request_duration_seconds` (histogram): Duration of destination requests.
* `prometheus_alertmanager_relay_outbound_request_failures_total` (counter): Total number of failed destination requests, partitioned by the `reason` label.
* `prometheus_alertmanager_relay_outbound_requests_total` (counter): Total number of destination requests.
* `prometheus_alertmanager_relay_received_alerts_total` (counter): Total number of alerts received in valid webhook envelopes.
* `prometheus_alertmanager_relay_webhook_request_duration_seconds` (histogram): Duration of webhook requests.
* `prometheus_alertmanager_relay_webhook_requests_total` (counter): Total number of received webhook requests.

The `reason` label on `prometheus_alertmanager_relay_outbound_request_failures_total` can have the values `connection`, `tls`, `timeout`, `status_4xx`, `status_5xx`, and `status_other`.

## Examples

The following examples configure an Alertmanager webhook receiver and a corresponding Alloy relay.

### Configure Alertmanager

Configure the source Alertmanager to send notifications to the Alloy webhook endpoint:

```yaml
receivers:
  - name: alloy-relay
    webhook_configs:
      - url: http://alloy:5001/webhook
```

Reference the receiver from the appropriate Alertmanager route.

### Relay alerts to another Alertmanager

The following example receives webhooks on all network interfaces and sends their alerts to an HTTPS Alertmanager endpoint:

```alloy
prometheus.alertmanager.relay "main" {
  listen_address = "0.0.0.0"
  listen_port    = 5001

  endpoint {
    url = "https://alertmanager.example.com/api/v2/alerts"

    basic_auth {
      username = sys.env("ALERTMANAGER_USERNAME")
      password = sys.env("ALERTMANAGER_PASSWORD")
    }

    tls_config {
      ca_file = "/etc/alloy/alertmanager-ca.pem"
    }
  }
}
```

Start Alloy with the `--feature.community-components.enabled` flag to use this community component.

## Technical details

For each accepted webhook, `prometheus.alertmanager.relay` converts the entire `alerts` collection into one Alertmanager API request.
The component preserves labels, annotations, start and end timestamps, and the generator URL.
Webhook-only fields such as `status` and `fingerprint` aren't included in the destination payload.

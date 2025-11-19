---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.write/
aliases:
  - ../loki.write/ # /docs/alloy/latest/reference/components/loki.write/
description: Learn about loki.write
labels:
  stage: general-availability
  products:
    - oss
title: loki.write
---

# `loki.write`

`loki.write` receives log entries from other loki components and sends them over the network using the Loki `logproto` format.

You can specify multiple `loki.write` components by giving them different labels.

## Usage

```alloy
loki.write "<LABEL>" {
  endpoint {
    url = "<REMOTE_WRITE_URL>"
  }
}
```

## Arguments

You can use the following arguments with `loki.write`:

| Name              | Type          | Description                                  | Default        | Required |
| ----------------- | ------------- | -------------------------------------------- | -------------- | -------- |
| `external_labels` | `map(string)` | Labels to add to logs sent over the network. | `{}`           | no       |
| `max_streams`     | `int`         | Maximum number of active streams.            | `0` (no limit) | no       |

## Blocks

You can use the following blocks with `loki.write`:

| Block                                              | Description                                                | Required |
| -------------------------------------------------- | ---------------------------------------------------------- | -------- |
| [`endpoint`][endpoint]                             | Location to send logs to.                                  | no       |
| `endpoint` > [`authorization`][authorization]      | Configure generic authorization to the endpoint.           | no       |
| `endpoint` > [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to the endpoint. | no       |
| `endpoint` > [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to the endpoint.    | no       |
| `endpoint` > `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to the endpoint.     | no       |
| `endpoint` > [`queue_config`][queue_config]        | Configure the queue used for endpoint.                     | no       |
| `endpoint` > [`tls_config`][tls_config]            | Configure TLS settings for connecting to the endpoint.     | no       |
| [`wal`][wal]                                       | Write-ahead log configuration.                             | no       |

The > symbol indicates deeper levels of nesting.
For example, `endpoint` > `basic_auth` refers to a `basic_auth` block defined inside an `endpoint` block.

[authorization]: #authorization
[basic_auth]: #basic_auth
[endpoint]: #endpoint
[oauth2]: #oauth2
[queue_config]: #queue_config
[tls_config]: #tls_config
[wal]: #wal

### `endpoint`

The `endpoint` block describes a single location to send logs to.
You can use multiple `endpoint` blocks to send logs to multiple locations.

The following arguments are supported:

| Name                     | Type                | Description                                                                                      | Default   | Required |
| ------------------------ | ------------------- | ------------------------------------------------------------------------------------------------ | --------- | -------- |
| `url`                    | `string`            | Full URL to send logs to.                                                                        |           | yes      |
| `batch_size`             | `string`            | Maximum batch size of logs to accumulate before sending.                                         | `"1MiB"`  | no       |
| `batch_wait`             | `duration`          | Maximum amount of time to wait before sending a batch.                                           | `"1s"`    | no       |
| `bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.                                             |           | no       |
| `bearer_token`           | `secret`            | Bearer token to authenticate with.                                                               |           | no       |
| `enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                                                         | `true`    | no       |
| `follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.                                     | `true`    | no       |
| `http_headers`           | `map(list(secret))` | Custom HTTP headers to be sent along with each request. The map key is the header name.          |           | no       |
| `headers`                | `map(string)`       | Extra headers to deliver with the request.                                                       |           | no       |
| `max_backoff_period`     | `duration`          | Maximum backoff time between retries.                                                            | `"5m"`    | no       |
| `max_backoff_retries`    | `int`               | Maximum number of retries.                                                                       | `10`      | no       |
| `min_backoff_period`     | `duration`          | Initial backoff time between retries.                                                            | `"500ms"` | no       |
| `name`                   | `string`            | Optional name to identify this endpoint with.                                                    |           | no       |
| `no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. |           | no       |
| `proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests.                                    |           | no       |
| `proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.                                            | `false`   | no       |
| `proxy_url`              | `string`            | HTTP proxy to send requests through.                                                             |           | no       |
| `remote_timeout`         | `duration`          | Timeout for requests made to the URL.                                                            | `"10s"`   | no       |
| `retry_on_http_429`      | `bool`              | Retry when an HTTP 429 status code is received.                                                  | `true`    | no       |
| `tenant_id`              | `string`            | The tenant ID used by default to push logs.                                                      |           | no       |

 At most, one of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`][endpoint] argument
* [`bearer_token`][endpoint] argument
* [`oauth2`][oauth2] block

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

If no `tenant_id` is provided, the component assumes that the Loki instance at `endpoint` is running in single-tenant mode and no X-Scope-OrgID header is sent.

When multiple `endpoint` blocks are provided, the `loki.write` component creates a client for each.
Received log entries are fanned-out to these endpoints in succession. That means that if one endpint is bottlenecked, it may impact the rest.

Each endpoint has a _queue_ of batches to be sent. The `queue_config` block can be used to customize the behavior of this queue.

Endpoints can be named for easier identification in debug metrics by using the `name` argument. If the `name` argument isn't provided, a name is generated based on a hash of the endpoint settings.

The `retry_on_http_429` argument specifies whether `HTTP 429` status code responses should be treated as recoverable errors.
Other `HTTP 4xx` status code responses are never considered recoverable errors.
When `retry_on_http_429` is enabled, the retry mechanism is governed by the backoff configuration specified through `min_backoff_period`, `max_backoff_period` and `max_backoff_retries` attributes.

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `queue_config`

The optional `queue_config` block configures how the endpoint queues batches of logs sent to Loki.

The following arguments are supported:

| Name            | Type       | Description                                                                                                                                                                   | Default | Required |
| --------------- | ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `capacity`      | `string`   | Controls the size of the underlying send queue buffer. This setting should be considered a worst-case scenario of memory consumption, in which all enqueued batches are full. | `10MiB` | no       |
| `drain_timeout` | `duration` | Configures the maximum time the client can take to drain the send queue upon shutdown. During that time, it enqueues pending batches and drains the send queue sending each.  | `"1m"`  | no       |
| `min_shards`    | `number`   | Minimum amount of concurrent shards sending samples to the endpoint.                                                                                                          | `1`      | no       |

Each endpoint manages a number of concurrent _shards_ which is responsible for sending a fraction of batches, number of shards are controlled with `min_shards` argument.
Each shard has a queue of batches it keeps in memory, controlled with the `capacity` argument.

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `wal`

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

The optional `wal` block configures the Write-Ahead Log (WAL) used in the Loki remote-write client.
To enable the WAL, you must include the `wal` block in your configuration.
When the WAL is enabled, the log entries sent to the `loki.write` component are first written to a WAL under the `dir` directory and then read into the remote-write client.
This process provides durability guarantees when an entry reaches this component. The client knows when to read from the WAL using the following two mechanisms:

* The WAL-writer side of the `loki.write` component notifies the reader side that new data is available.
* The WAL-reader side periodically checks if there is new data, increasing the wait time exponentially between `min_read_frequency` and `max_read_frequency`.

The WAL is located inside a component-specific directory relative to the storage path {{< param "PRODUCT_NAME" >}} is configured to use.
Refer to the [`run` documentation][run] for more information about how to change the storage path.

The following arguments are supported:

| Name                 | Type       | Description                                                                                                    | Default   | Required |
| -------------------- | ---------- | -------------------------------------------------------------------------------------------------------------- | --------- | -------- |
| `drain_timeout`      | `duration` | Maximum time the WAL drain procedure can take, before being forcefully stopped.                                | `"30s"`   | no       |
| `enabled`            | `bool`     | Whether to enable the WAL.                                                                                     | `false`   | no       |
| `max_read_frequency` | `duration` | Maximum backoff time in the backup read mechanism.                                                             | `"1s"`    | no       |
| `max_segment_age`    | `duration` | Maximum time a WAL segment should be allowed to live. Segments older than this setting are eventually deleted. | `"1h"`    | no       |
| `min_read_frequency` | `duration` | Minimum backoff time in the backup read mechanism.                                                             | `"250ms"` | no       |

[run]: ../../../cli/run/

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type           | Description                                                   |
| ---------- | -------------- | ------------------------------------------------------------- |
| `receiver` | `LogsReceiver` | A value that other components can use to send log entries to. |

## Component health

`loki.write` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.write` doesn't expose any component-specific debug information.

## Debug metrics

* `loki_write_batch_retries_total` (counter): Number of times batches have had to be retried.
* `loki_write_dropped_bytes_total` (counter): Number of bytes dropped because failed to be sent to the ingester after all retries.
* `loki_write_dropped_entries_total` (counter): Number of log entries dropped because they failed to be sent to the ingester after all retries.
* `loki_write_encoded_bytes_total` (counter): Number of bytes encoded and ready to send.
* `loki_write_request_duration_seconds` (histogram): Duration of sent requests.
* `loki_write_sent_bytes_total` (counter): Number of bytes sent.
* `loki_write_sent_entries_total` (counter): Number of log entries sent to the ingester.
* `loki_write_stream_lag_seconds` (gauge): Difference between current time and last batch timestamp for successful sends.

## Examples

The following examples show you how to create `loki.write` components that send log entries to different destinations.

### Send log entries to a local Loki instance

You can create a `loki.write` component that sends your log entries to a local Loki instance:

```alloy
loki.write "local" {
    endpoint {
        url = "http://loki:3100/loki/api/v1/push"
    }
}
```

### Send log entries to a managed service

You can create a `loki.write` component that sends your log entries to a managed service, for example, Grafana Cloud. The Loki username and Grafana Cloud API Key are injected in this example through environment variables.

```alloy
loki.write "default" {
    endpoint {
        url = "https://logs-xxx.grafana.net/loki/api/v1/push"
        basic_auth {
            username = sys.env("LOKI_USERNAME")
            password = sys.env("GRAFANA_CLOUD_API_KEY")
        }
    }
}
```

## Technical details

`loki.write` uses [snappy](https://en.wikipedia.org/wiki/Snappy_(compression)) for compression.

Any labels that start with `__` are removed before sending to the endpoint.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.write` has exports that can be consumed by the following components:

- Components that consume [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

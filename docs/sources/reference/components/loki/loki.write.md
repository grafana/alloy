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

{{< docs/shared lookup="generated/components/loki/write/__arguments.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Blocks

You can use the following blocks with `loki.write`:

{{< docs/alloy-config >}}

{{< docs/shared lookup="generated/components/loki/write/__blocks.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< /docs/alloy-config >}}

### `endpoint`

The `endpoint` block describes a single location to send logs to.
You can use multiple `endpoint` blocks to send logs to multiple locations.

{{< docs/shared lookup="generated/components/loki/write/endpoint.md" source="alloy" version="<ALLOY_VERSION>" >}}

At most, one of the following can be provided:

* [`authorization`](#authorization) block
* [`basic_auth`](#basic_auth) block
* [`bearer_token_file`](#endpoint) argument
* [`bearer_token`](#endpoint) argument
* [`oauth2`](#oauth2) block

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

If no `tenant_id` is provided, the component assumes that the Loki instance at `endpoint` is running in single-tenant mode and no X-Scope-OrgID header is sent.

When multiple `endpoint` blocks are provided, the `loki.write` component creates a client for each.
Received log entries are fanned-out to these endpoints in succession. That means that if one endpoint is bottlenecked, it may impact the rest.

Each endpoint has a _queue_ of batches to be sent. The `queue_config` block can be used to customize the behavior of this queue.

Endpoints can be named for easier identification in debug metrics by using the `name` argument. If the `name` argument isn't provided, a name is generated based on a hash of the endpoint settings.

The `retry_on_http_429` argument specifies whether `HTTP 429` status code responses should be treated as recoverable errors.
Other `HTTP 4xx` status code responses are never considered recoverable errors.
When `retry_on_http_429` is enabled, the retry mechanism is governed by the backoff configuration specified through `min_backoff_period`, `max_backoff_period` and `max_backoff_retries` attributes.

### `authorization`

{{< docs/shared lookup="generated/common/config/authorization.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="generated/common/config/basic_auth.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `oauth2`

{{< docs/shared lookup="generated/common/config/oauth2.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `queue_config`

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

The optional `queue_config` block configures how the endpoint queues batches of logs sent to Loki.

{{< docs/shared lookup="generated/components/loki/write/queue_config.md" source="alloy" version="<ALLOY_VERSION>" >}}

Each endpoint is divided into a number of concurrent _shards_ which are responsible for sending a fraction of batches. The number of shards is controlled with `min_shards` argument.
Each shard has a queue of batches it keeps in memory, controlled with the `capacity` argument.

Queue size is calculated using `batch_size` and `capacity` for each shard. So if `batch_size` is 1MiB and `capacity` is 10MiB each shard would be able to queue up 10 batches.
The maximum amount of memory required for all configured shards can be calculated using `capacity` * `min_shards`. 

### `tls_config`

{{< docs/shared lookup="generated/common/config/tls_config.md" source="alloy" version="<ALLOY_VERSION>" >}}

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

{{< docs/shared lookup="generated/components/loki/write/wal.md" source="alloy" version="<ALLOY_VERSION>" >}}

[run]: ../../../cli/run/

## Exported fields

{{< docs/shared lookup="generated/components/loki/write/__exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`loki.write` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.write` doesn't expose any component-specific debug information.

## Debug metrics

* `loki_write_batch_retries_total` (counter): Number of times batches have had to be retried.
* `loki_write_dropped_bytes_total` (counter): Number of bytes dropped because failed to be sent to the ingester after all retries.
* `loki_write_dropped_entries_total` (counter): Number of log entries dropped because they failed to be sent to the ingester after all retries.
* `loki_write_sent_bytes_total` (counter): Number of bytes sent.
* `loki_write_sent_entries_total` (counter): Number of log entries sent to the ingester.
* `loki_write_request_size_bytes` (histogram): Number of bytes for encoded requests.
* `loki_write_request_duration_seconds` (histogram): Duration of sent requests.
* `loki_write_entry_propagation_latency_seconds` (histogram): Time in seconds from entry creation until it's either successfully sent or dropped.

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

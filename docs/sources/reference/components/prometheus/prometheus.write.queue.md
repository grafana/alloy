---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.write.queue/
description: Learn about prometheus.write.queue
labels:
  stage: experimental
  products:
    - oss
title: prometheus.write.queue
---

# `prometheus.write.queue`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`prometheus.write.queue` collects metrics sent from other components into a Write-Ahead Log (WAL) and forwards them over the network to a series of user-supplied endpoints.
Metrics are sent over the network using the [Prometheus Remote Write protocol][remote_write-spec].

{{< admonition type="note" >}}
This component is deprecated and will be removed in a future version of {{< param "PRODUCT_NAME" >}}. Please use `prometheus.remote_write` instead.
{{< /admonition >}}

You can specify multiple `prometheus.write.queue` components by giving them different labels.

[remote_write-spec]: https://prometheus.io/docs/specs/remote_write_spec/

## Usage

```alloy
prometheus.write.queue "<LABEL>" {
  endpoint "default" {
    url = "<REMOTE_WRITE_URL>"

    ...
  }

  ...
}
```

## Arguments

You can use the following argument with `prometheus.write.queue`:

| Name  | Type      | Description                                                        | Default   | Required |
| ----- | --------- | ------------------------------------------------------------------ | --------- | -------- |
| `ttl` | `duration`| How long the samples can be queued for before they're discarded.   | `"2h"`    | no       |

## Blocks

You can use the following blocks with `prometheus.write.queue`:

| Block                                     | Description                                                | Required |
| ----------------------------------------- | ---------------------------------------------------------- | -------- |
| [`endpoint`][endpoint]                    | Location to send metrics to.                               | no       |
| `endpoint` > [`basic_auth`][basic_auth]   | Configure `basic_auth` for authenticating to the endpoint. | no       |
| `endpoint` > [`tls_config`][tls_config]   | Configure TLS settings for connecting to the endpoint.     | no       |
| `endpoint` > [`parallelism`][parallelism] | Configure parallelism for the endpoint.                    | no       |
| [`persistence`][persistence]              | Configuration for persistence                              | no       |

The > symbol indicates deeper levels of nesting.
For example, `endpoint` > `basic_auth` refers to a `basic_auth` block defined inside an `endpoint` block.

[endpoint]: #endpoint
[basic_auth]: #basic_auth
[persistence]: #persistence
[tls_config]: #tls_config
[parallelism]: #parallelism

### `endpoint`

The `endpoint` block describes a single location to send metrics to.
Multiple `endpoint` blocks can be provided to send metrics to multiple locations.
Each `endpoint` has its own WAL folder.

The following arguments are supported:

| Name                     | Type          | Description                                                                      | Default                     | Required |
|--------------------------|---------------|----------------------------------------------------------------------------------|-----------------------------|----------|
| `url`                    | `string`      | Full URL to send metrics to.                                                     |                             | yes      |
| `batch_count`            | `uint`        | How many series to queue in each queue.                                          | `1000`                      | no       |
| `bearer_token`           | `secret`      | Bearer token to authenticate with.                                               |                             | no       |
| `enable_round_robin`     | `bool`        | Use round robin load balancing when there are multiple IPs for a given endpoint. | `false`                     | no       |
| `external_labels`        | `map(string)` | Labels to add to metrics sent over the network.                                  |                             | no       |
| `flush_interval`         | `duration`    | How long to wait until sending if `batch_count` isn't triggered.                 | `"1s"`                      | no       |
| `headers`                | `map(secret)` | Custom HTTP headers to add to all requests sent to the server.                   |                             | no       |
| `max_retry_attempts`     | `uint`        | Maximum number of retries before dropping the batch.                             | `0`                         | no       |
| `metadata_cache_enabled` | `bool`        | Enables an LRU cache for tracking Metadata to support sparse metadata sending.   | `false`                     | no       |
| `metadata_cache_size`    | `uint`        | Maximum number of metadata entries to keep in cache to track what has been sent. | `1000`                      | no       |
| `protobuf_message`       | `string`      | Protobuf message format to use for remote write.                                 | `"prometheus.WriteRequest"` | no       |
| `proxy_url`              | `string`      | URL of the HTTP proxy to use for requests.                                       |                             | no       |
| `proxy_from_environment` | `bool`        | Whether to read proxy configuration from environment variables.                  | `false`                     | no       |
| `proxy_connect_headers`  | `map(secret)` | HTTP headers to send to proxies during CONNECT requests.                         |                             | no       |
| `retry_backoff`          | `duration`    | How long to wait between retries.                                                | `"1s"`                      | no       |
| `write_timeout`          | `duration`    | Timeout for requests made to the URL.                                            | `"30s"`                     | no       |

`protobuf_message` must be `prometheus.WriteRequest` or `io.prometheus.write.v2.Request`. These values represent prometheus remote write protocol versions 1 and 2.

'metadata_cache_enabled' and `metadata_cache_size` are only relevant when using `io.prometheus.write.v2.Request`, and is intended to reduce the frequency of metadata sending to reduce overall network traffic.
A larger cache_size will consume more memory, but if you are sending many different metrics will also reduce how frequently metadata is sent with samples.

### `basic_auth`

| Name       | Type     | Description          | Default | Required |
| ---------- | -------- | -------------------- | ------- | -------- |
| `password` | `secret` | Basic auth password. |         | no       |
| `username` | `string` | Basic auth username. |         | no       |

### `tls_config`

| Name                   | Type     | Description                                             | Default | Required |
| ---------------------- | -------- | ------------------------------------------------------- | ------- | -------- |
| `ca_pem`               | `string` | CA PEM-encoded text to validate the server with.        |         | no       |
| `cert_pem`             | `string` | Certificate PEM-encoded text for client authentication. |         | no       |
| `insecure_skip_verify` | `bool`   | Disables validation of the server certificate.          |         | no       |
| `key_pem`              | `secret` | Key PEM-encoded text for client authentication.         |         | no       |

### `parallelism`

| Name                             | Type       | Description                                                                                                                       | Default | Required |
| -------------------------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `allowed_network_error_fraction` | `float`    | The allowed error rate before scaling down. For example `0.50` allows 50% error rate.                                             | `0.50`  | no       |
| `desired_check_interval`         | `duration` | The length of time between checking for desired connections.                                                                      | `"5s"`  | no       |
| `desired_connections_lookback`   | `duration` | The length of time that previous desired connections are kept for determining desired connections.                                | `"5m"`  | no       |
| `drift_scale_down`               | `duration` | The minimum amount of time between the timestamps of incoming signals and outgoing signals before decreasing desired connections. | `"30s"` | no       |
| `drift_scale_up`                 | `duration` | The maximum amount of time between the timestamps of incoming signals and outgoing signals before increasing desired connections. | `"60s"` | no       |
| `max_connections`                | `uint`     | The maximum number of desired connections.                                                                                        | `50`    | no       |
| `min_connections`                | `uint`     | The minimum number of desired connections.                                                                                        | `2`     | no       |
| `network_flush_interval`         | `duration` | The length of time that network successes and failures are kept for determining desired connections.                              | `"1m"`  | no       |

Parallelism determines when to scale up or down the number of desired connections.

The drift between the incoming and outgoing timestamps determines whether to increase or decrease the desired connections.
The value stays the same if the drift is between `drift_scale_up_seconds` and `drift_scale_down_seconds`.

Network successes and failures are recorded and kept in memory.
This data helps determine the nature of the drift.
For example, if the drift is increasing and the network failures are increasing, the desired connections shouldn't increase because that would increase the load on the endpoint.

The `desired_check_interval` prevents connection flapping.
Each time a desired connection is calculated, the connection is added to a lookback buffer.
Before increasing or decreasing the desired connections, `prometheus.write.queue` chooses the highest value in the lookback buffer.
For example, for the past 5 minutes, the desired connections have been: [2,1,1].
The check determines that the desired connections are 1, and the number of desired connections won't change because the value `2` is still in the lookback buffer.
On the next check, the desired connections are [1,1,1].
Since the `2` value has expired, the desired connections change to 1.
In general, the system is fast to increase and slow to decrease the desired connections.

### `persistence`

The `persistence` block describes how often and at what limits to write to disk.
Persistence settings are shared for each `endpoint`.

The following arguments are supported:

| Name                   | Type       | Description                                                                 | Default | Required |
| ---------------------- | ---------- | --------------------------------------------------------------------------- | ------- | -------- |
| `batch_interval`       | `duration` | How often to batch signals to disk if `max_signals_to_batch` isn't reached. | `"5s"`  | no       |
| `max_signals_to_batch` | `uint`     | The maximum number of signals before they're batched to disk.               | `10000` | no       |

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type              | Description                                               |
| ---------- | ----------------- | --------------------------------------------------------- |
| `receiver` | `MetricsReceiver` | A value that other components can use to send metrics to. |

## Component health

`prometheus.write.queue` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields are kept at their last healthy values.

## Debug information

`prometheus.write.queue` doesn't expose any component-specific debug information.

## Debug metrics

The following metrics are provided for backward compatibility.
They generally behave the same, but there are likely edge cases where they differ.

<!-- * `prometheus_remote_storage_enqueue_retries_total` (counter): Total number of times enqueue has failed because a shard's queue was full.
* `prometheus_remote_storage_exemplars_dropped_total` (counter): Total number of exemplars that were dropped after being read from the WAL before being sent to `remote_write` because of an unknown reference ID.
* `prometheus_remote_storage_exemplars_failed_total` (counter): Total number of exemplars that failed to send to remote storage due to non-recoverable errors.
* `prometheus_remote_storage_exemplars_in_total` (counter): Exemplars read into remote storage.
* `prometheus_remote_storage_exemplars_retried_total` (counter): Total number of exemplars that failed to send to remote storage but were retried due to recoverable errors.
* `prometheus_remote_storage_exemplars_total` (counter): Total number of exemplars sent to remote storage. -->
* `prometheus_remote_storage_metadata_failed_total` (counter): Total number of metadata entries that failed to send to remote storage due to non-recoverable errors.
* `prometheus_remote_storage_metadata_retried_total` (counter): Total number of metadata entries that failed to send to remote storage but were retried due to recoverable errors.
* `prometheus_remote_storage_metadata_total` (counter): Total number of metadata entries sent to remote storage.
* `prometheus_remote_storage_queue_highest_sent_timestamp_seconds` (gauge): Unix timestamp of the latest WAL sample successfully sent by a queue.
* `prometheus_remote_storage_highest_timestamp_in_seconds` TODO
<!-- * `prometheus_remote_storage_samples_dropped_total` (counter): Total number of samples which were dropped after being read from the WAL before being sent to `remote_write` because of an unknown reference ID. -->
* `prometheus_remote_storage_samples_failed_total` (counter): Total number of samples that failed to send to remote storage due to non-recoverable errors.
<!-- * `prometheus_remote_storage_samples_in_total` (counter): Samples read into remote storage. -->
* `prometheus_remote_storage_samples_retried_total` (counter): Total number of samples that failed to send to remote storage but were retried due to recoverable errors.
* `prometheus_remote_storage_samples_total` (counter): Total number of samples sent to remote storage.
* `prometheus_remote_storage_sent_batch_duration_seconds` (histogram): Duration of send calls to remote storage.
<!-- * `prometheus_remote_write_wal_exemplars_appended_total` (counter): Total number of exemplars appended to the WAL. -->
<!-- * `prometheus_remote_write_wal_samples_appended_total` (counter): Total number of samples appended to the WAL. -->
<!-- * `prometheus_remote_write_wal_storage_created_series_total` (counter): Total number of created series appended to the WAL.
* `prometheus_remote_write_wal_storage_removed_series_total` (counter): Total number of series removed from the WAL. -->
* `prometheus_remote_storage_bytes_total` (counter): Total number of bytes of data sent by queues after compression.
* `prometheus_remote_storage_sent_bytes_total` (counter): Total number of bytes of data sent by queues after compression. (same as `prometheus_remote_storage_bytes_total`)
* `prometheus_remote_storage_sent_batch_duration_seconds` (histogram): Duration of send calls to remote storage.
* `prometheus_remote_storage_shards_max` (gauge): The maximum number of a shards a queue is allowed to run.
* `prometheus_remote_storage_shards_min` (gauge): The minimum number of shards a queue is allowed to run.
* `prometheus_remote_storage_shards` (gauge): The number of shards used for concurrent delivery of metrics to an endpoint.

Metrics that are new to `prometheus.write.queue`. These are highly subject to change.

* `alloy_queue_metadata_network_sent_total` (counter): Number of metadata sent successfully.
* `alloy_queue_metadata_serializer_errors_total` (counter): Number of errors for metadata written to serializer.
* `alloy_queue_metadata_serializer_incoming_signals_total` (counter): Total number of metadata written to serialization.
* `alloy_queue_metadata_network_failed_total` (counter): Number of metadata failed.
* `alloy_queue_metadata_network_duration_seconds` (histogram): Duration writing metadata to endpoint.
* `alloy_queue_metadata_network_errors_total` (counter): Number of errors writing metadata to network.
* `alloy_queue_metadata_network_retried_429_total` (counter): Number of metadata retried due to status code 429.
* `alloy_queue_metadata_network_retried_5xx_total` (counter): Number of metadata retried due to status code 5xx.
* `alloy_queue_metadata_network_retried_total` (counter): Number of metadata retried due to network issues.
* `alloy_queue_series_network_failed_total` (counter): Number of series failed.
* `alloy_queue_series_network_duration_seconds` (histogram): Duration writing series to endpoint.
* `alloy_queue_series_network_errors_total` (counter): Number of errors writing series to network.
* `alloy_queue_series_network_retried_429_total` (counter): Number of series retried due to status code 429.
* `alloy_queue_series_network_retried_5xx_total` (counter): Number of series retried due to status code 5xx.
* `alloy_queue_series_network_retried_total` (counter): Number of series retried due to network issues.
* `alloy_queue_series_network_sent_total` (counter): Number of series sent successfully.
* `alloy_queue_series_network_timestamp_seconds` (gauge): Highest timestamp written to an endpoint.
* `alloy_queue_series_serializer_errors_total` (counter): Number of errors for series written to serializer.
* `alloy_queue_series_serializer_incoming_signals_total` (counter): Total number of series written to serialization.
* `alloy_queue_series_serializer_incoming_exemplars_total` (counter): Total number of exemplars written to serialization.
* `alloy_queue_series_serializer_incoming_timestamp_seconds` (gauge): Highest timestamp of incoming series.
* `alloy_queue_series_disk_compressed_bytes_read_total` (counter): Total number of compressed bytes read from disk.
* `alloy_queue_series_disk_compressed_bytes_written_total` (counter): Total number of compressed bytes written to disk.
* `alloy_queue_series_disk_uncompressed_bytes_read_total` (counter): Total number of uncompressed bytes read from disk.
* `alloy_queue_series_disk_uncompressed_bytes_written_total` (counter): Total number of uncompressed bytes written to disk.
* `alloy_queue_series_file_id_written` (gauge): Current file id written, file id being a numeric number.
* `alloy_queue_series_file_id_read` (gauge): Current file id read, file id being a numeric number.


## Examples

The following examples show you how to create `prometheus.write.queue` components that send metrics to different destinations.

### Send metrics to a local Mimir instance

You can create a `prometheus.write.queue` component that sends your metrics to a local Mimir instance:

```alloy
prometheus.write.queue "staging" {
  // Send metrics to a locally running Mimir.
  endpoint "mimir" {
    url = "http://mimir:9009/api/v1/push"

    basic_auth {
      username = "example-user"
      password = "example-password"
    }
  }
}

// Configure a prometheus.scrape component to send metrics to
// prometheus.write.queue component.
prometheus.scrape "demo" {
  targets = [
    // Collect metrics from the default HTTP listen address.
    {"__address__" = "127.0.0.1:12345"},
  ]
  forward_to = [prometheus.write.queue.staging.receiver]
}
```

## Technical details

`prometheus.write.queue` uses [zstd][] for compression.
`prometheus.write.queue` sends native histograms by default.
Any labels that start with `__` will be removed before sending to the endpoint.

### Data retention

Data is written to disk in blocks utilizing [zstd][] compression. These blocks are read on startup and resent if they're still within the TTL.
Any data that hasn't been written to disk, or that's in the network queues is lost if {{< param "PRODUCT_NAME" >}} is restarted.

### Retries

`prometheus.write.queue`  retries sending data if the following errors or HTTP status codes are returned:

* Network errors.
* HTTP 429 errors.
* HTTP 5XX errors.

`prometheus.write.queue` won't retry sending data if any other unsuccessful status codes are returned.

### Memory

`prometheus.write.queue` is meant to be memory efficient.
You can adjust the `max_signals_to_batch`, `parallelism`, and `batch_size` to control how much memory is used.
A higher `max_signals_to_batch` allows for more efficient disk compression.
A higher `parallelism` allows more parallel writes, and `batch_size` allows more data sent at one time.
This can allow greater throughput at the cost of more memory on both {{< param "PRODUCT_NAME" >}} and the endpoint.
The defaults are suitable for most common usages.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.write.queue` has exports that can be consumed by the following components:

- Components that consume [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

[zstd]: https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md

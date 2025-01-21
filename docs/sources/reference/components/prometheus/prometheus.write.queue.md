---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.write.queue/
description: Learn about prometheus.write.queue
title: prometheus.write.queue
---


<span class="badge docs-labels__stage docs-labels__item">Experimental</span>

# prometheus.write.queue

`prometheus.write.queue` collects metrics sent from other components into a
Write-Ahead Log (WAL) and forwards them over the network to a series of
user-supplied endpoints. Metrics are sent over the network using the
[Prometheus Remote Write protocol][remote_write-spec].

You can specify multiple `prometheus.write.queue` components by giving them different labels.

You should consider everything here extremely experimental and highly subject to change.
[remote_write-spec]: https://prometheus.io/docs/specs/remote_write_spec/

  

## Usage

```alloy
prometheus.write.queue "LABEL" {
  endpoint "default "{
    url = REMOTE_WRITE_URL

    ...
  }

  ...
}
```

## Arguments

The following arguments are supported:

 Name  | Type   | Description | Default                                                           | Required 
-------|--------|-------------|-------------------------------------------------------------------|----------
 `ttl` | `time` | `duration`  | How long the samples can be queued for before they are discarded. | `2h`     | no

## Blocks

The following blocks are supported inside the definition of
`prometheus.write.queue`:

 Hierarchy             | Block            | Description                                              | Required 
-----------------------|------------------|----------------------------------------------------------|----------
 persistence           | [persistence][]  | Configuration for persistence                            | no       
 endpoint              | [endpoint][]     | Location to send metrics to.                             | no       
 endpoint > basic_auth | [basic_auth][]   | Configure basic_auth for authenticating to the endpoint. | no       
 endpoint > tls_config | [tls_config][]   | Configure TLS settings for connecting to the endpoint.   | no


The `>` symbol indicates deeper levels of nesting. For example, `endpoint >
basic_auth` refers to a `basic_auth` block defined inside an
`endpoint` block.

[endpoint]: #endpoint-block
[basic_auth]: #basic_auth-block
[persistence]: #persistence-block
[tls_config]: #tls_config-block


### persistence block

The `persistence` block describes how often and at what limits to write to disk. Persistence settings
are shared for each `endpoint`.

The following arguments are supported:

 Name                   | Type       | Description                                                                  | Default | Required 
------------------------|------------|------------------------------------------------------------------------------|---------|----------
 `max_signals_to_batch` | `uint`     | The maximum number of signals before they are batched to disk.               | `10000` | no       
 `batch_interval`       | `duration` | How often to batch signals to disk if `max_signals_to_batch` is not reached. | `5s`    | no       


### endpoint block

The `endpoint` block describes a single location to send metrics to. Multiple
`endpoint` blocks can be provided to send metrics to multiple locations. Each
`endpoint` will have its own WAL folder.

The following arguments are supported:

 Name                 | Type          | Description                                                                                 | Default | Required 
----------------------|---------------|---------------------------------------------------------------------------------------------|---------|----------
 `url`                | `string`      | Full URL to send metrics to.                                                                |         | yes      
 `bearer_token`        | `secret`      | Bearer token to authenticate with.                                                          |         | no
 `write_timeout`      | `duration`    | Timeout for requests made to the URL.                                                       | `"30s"` | no       
 `retry_backoff`      | `duration`    | How long to wait between retries.                                                           | `1s`    | no       
 `max_retry_attempts` | `uint`        | Maximum number of retries before dropping the batch.                                        | `0`     | no      
 `batch_count`        | `uint`        | How many series to queue in each queue.                                                     | `1000`  | no       
 `flush_interval`     | `duration`    | How long to wait until sending if `batch_count` is not trigger.                             | `1s`    | no       
 `parallelism`        | `uint`        | How many parallel batches to write.                                                         | 10      | no       
 `external_labels`    | `map(string)` | Labels to add to metrics sent over the network.                                             |         | no       
 `enable_round_robin` | `bool`        | Use round robin load balancing when there are multiple IPs for a given endpoint. | `false` | no       


### basic_auth block

Name            | Type     | Description                              | Default | Required
----------------|----------|------------------------------------------|---------|---------
`password`      | `secret` | Basic auth password.                     |         | no
`username`      | `string` | Basic auth username.                     |         | no

### tls_config block

Name                   | Type     | Description                                              | Default | Required
-----------------------|----------|----------------------------------------------------------|---------|---------
`ca_pem`               | `string` | CA PEM-encoded text to validate the server with.         |         | no
`cert_pem`             | `string` | Certificate PEM-encoded text for client authentication.  |         | no
`insecure_skip_verify` | `bool`   | Disables validation of the server certificate.           |         | no
`key_pem`              | `secret` | Key PEM-encoded text for client authentication.          |         | no

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`receiver` | `MetricsReceiver` | A value that other components can use to send metrics to.

## Component health

`prometheus.write.queue` is only reported as unhealthy if given an invalid
configuration. In those cases, exported fields are kept at their last healthy
values.

## Debug information

`prometheus.write.queue` does not expose any component-specific debug
information.

## Debug metrics

The following metrics are provided for backward compatibility.
They generally behave the same, but there are likely edge cases where they differ.

* `prometheus_remote_write_wal_storage_created_series_total` (counter): Total number of created
  series appended to the WAL.
* `prometheus_remote_write_wal_storage_removed_series_total` (counter): Total number of series
  removed from the WAL.
* `prometheus_remote_write_wal_samples_appended_total` (counter): Total number of samples
  appended to the WAL.
* `prometheus_remote_write_wal_exemplars_appended_total` (counter): Total number of exemplars
  appended to the WAL.
* `prometheus_remote_storage_samples_total` (counter): Total number of samples
  sent to remote storage.
* `prometheus_remote_storage_exemplars_total` (counter): Total number of
  exemplars sent to remote storage.
* `prometheus_remote_storage_metadata_total` (counter): Total number of
  metadata entries sent to remote storage.
* `prometheus_remote_storage_samples_failed_total` (counter): Total number of
  samples that failed to send to remote storage due to non-recoverable errors.
* `prometheus_remote_storage_exemplars_failed_total` (counter): Total number of
  exemplars that failed to send to remote storage due to non-recoverable errors.
* `prometheus_remote_storage_metadata_failed_total` (counter): Total number of
  metadata entries that failed to send to remote storage due to
  non-recoverable errors.
* `prometheus_remote_storage_samples_retries_total` (counter): Total number of
  samples that failed to send to remote storage but were retried due to
  recoverable errors.
* `prometheus_remote_storage_exemplars_retried_total` (counter): Total number of
  exemplars that failed to send to remote storage but were retried due to
  recoverable errors.
* `prometheus_remote_storage_metadata_retried_total` (counter): Total number of
  metadata entries that failed to send to remote storage but were retried due
  to recoverable errors.
* `prometheus_remote_storage_samples_dropped_total` (counter): Total number of
  samples which were dropped after being read from the WAL before being sent to
  remote_write because of an unknown reference ID.
* `prometheus_remote_storage_exemplars_dropped_total` (counter): Total number
  of exemplars that were dropped after being read from the WAL before being
  sent to remote_write because of an unknown reference ID.
* `prometheus_remote_storage_enqueue_retries_total` (counter): Total number of
  times enqueue has failed because a shard's queue was full.
* `prometheus_remote_storage_sent_batch_duration_seconds` (histogram): Duration
  of send calls to remote storage.
* `prometheus_remote_storage_queue_highest_sent_timestamp_seconds` (gauge):
  Unix timestamp of the latest WAL sample successfully sent by a queue.
* `prometheus_remote_storage_samples_in_total` (counter): Samples read into
  remote storage.
* `prometheus_remote_storage_exemplars_in_total` (counter): Exemplars read into
  remote storage.

Metrics that are new to `prometheus.write.queue`. These are highly subject to change.

* `alloy_queue_series_serializer_incoming_signals` (counter): Total number of series written to serialization.
* `alloy_queue_metadata_serializer_incoming_signals` (counter): Total number of metadata written to serialization.
* `alloy_queue_series_serializer_incoming_timestamp_seconds` (gauge): Highest timestamp of incoming series.
* `alloy_queue_series_serializer_errors` (gauge): Number of errors for series written to serializer.
* `alloy_queue_metadata_serializer_errors` (gauge): Number of errors for metadata written to serializer.
* `alloy_queue_series_network_timestamp_seconds` (gauge): Highest timestamp written to an endpoint.
* `alloy_queue_series_network_sent` (counter): Number of series sent successfully.
* `alloy_queue_metadata_network_sent` (counter): Number of metadata sent successfully.
* `alloy_queue_network_series_failed` (counter): Number of series failed.
* `alloy_queue_network_metadata_failed` (counter): Number of metadata failed.
* `alloy_queue_network_series_retried` (counter): Number of series retried due to network issues.
* `alloy_queue_network_metadata_retried` (counter): Number of metadata retried due to network issues.
* `alloy_queue_network_series_retried_429` (counter): Number of series retried due to status code 429.
* `alloy_queue_network_metadata_retried_429` (counter): Number of metadata retried due to status code 429.
* `alloy_queue_network_series_retried_5xx` (counter): Number of series retried due to status code 5xx.
* `alloy_queue_network_metadata_retried_5xx` (counter): Number of metadata retried due to status code 5xx.
* `alloy_queue_network_series_network_duration_seconds` (histogram): Duration writing series to endpoint.
* `alloy_queue_network_metadata_network_duration_seconds` (histogram): Duration writing metadata to endpoint.
* `alloy_queue_network_series_network_errors` (counter): Number of errors writing series to network.
* `alloy_queue_network_metadata_network_errors` (counter): Number of errors writing metadata to network.

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

`prometheus.write.queue` uses [snappy][] for compression.
`prometheus.write.queue` sends native histograms by default.
Any labels that start with `__` will be removed before sending to the endpoint.

### Data retention

Data is written to disk in blocks utilizing [snappy][] compression. These blocks are read on startup and resent if they are still within the TTL. 
Any data that has not been written to disk, or that is in the network queues is lost if {{< param "PRODUCT_NAME" >}} is restarted.

### Retries

`prometheus.write.queue`  will retry sending data if the following errors or HTTP status codes are returned:

 * Network errors. 
 * HTTP 429 errors. 
 * HTTP 5XX errors.
 
`prometheus.write.queue`  will  not retry sending data if any other unsuccessful status codes are returned. 

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

[snappy]: https://en.wikipedia.org/wiki/Snappy_(compression)
[Stop]: ../../../../set-up/run/
[run]: ../../../cli/run/

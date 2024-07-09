---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.remote_write/
aliases:
  - ../prometheus.remote_write/ # /docs/alloy/latest/reference/components/prometheus.remote_write/
description: Learn about prometheus.remote_write
title: prometheus.remote_write
---

# prometheus.remote_write

`prometheus.remote_write` collects metrics sent from other components into a
Write-Ahead Log (WAL) and forwards them over the network to a series of
user-supplied endpoints. Metrics are sent over the network using the
[Prometheus Remote Write protocol][remote_write-spec].

Multiple `prometheus.remote_write` components can be specified by giving them
different labels.

[remote_write-spec]: https://docs.google.com/document/d/1LPhVRSFkGNSuU1fBd81ulhsCPR4hkSZyyBj1SZ8fWOM/edit

## Usage

```alloy
prometheus.remote_write "LABEL" {
  endpoint {
    url = REMOTE_WRITE_URL

    ...
  }

  ...
}
```

## Arguments

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`external_labels` | `map(string)` | Labels to add to metrics sent over the network. | | no

## Blocks

The following blocks are supported inside the definition of
`prometheus.remote_write`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
endpoint | [endpoint][] | Location to send metrics to. | no
endpoint > basic_auth | [basic_auth][] | Configure basic_auth for authenticating to the endpoint. | no
endpoint > authorization | [authorization][] | Configure generic authorization to the endpoint. | no
endpoint > oauth2 | [oauth2][] | Configure OAuth2 for authenticating to the endpoint. | no
endpoint > oauth2 > tls_config | [tls_config][] | Configure TLS settings for connecting to the endpoint. | no
endpoint > sigv4 | [sigv4][] | Configure AWS Signature Verification 4 for authenticating to the endpoint. | no
endpoint > azuread | [azuread][] | Configure AzureAD for authenticating to the endpoint. | no
endpoint > azuread > managed_identity | [managed_identity][] | Configure Azure user-assigned managed identity. | yes
endpoint > tls_config | [tls_config][] | Configure TLS settings for connecting to the endpoint. | no
endpoint > queue_config | [queue_config][] | Configuration for how metrics are batched before sending. | no
endpoint > metadata_config | [metadata_config][] | Configuration for how metric metadata is sent. | no
endpoint > write_relabel_config | [write_relabel_config][] | Configuration for write_relabel_config. | no
wal | [wal][] | Configuration for the component's WAL. | no

The `>` symbol indicates deeper levels of nesting. For example, `endpoint >
basic_auth` refers to a `basic_auth` block defined inside an
`endpoint` block.

[endpoint]: #endpoint-block
[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[sigv4]: #sigv4-block
[azuread]: #azuread-block
[managed_identity]: #managed_identity-block
[tls_config]: #tls_config-block
[queue_config]: #queue_config-block
[metadata_config]: #metadata_config-block
[write_relabel_config]: #write_relabel_config-block
[wal]: #wal-block

### endpoint block

The `endpoint` block describes a single location to send metrics to. Multiple
`endpoint` blocks can be provided to send metrics to multiple locations.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`url` | `string` | Full URL to send metrics to. | | yes
`name` | `string` | Optional name to identify the endpoint in metrics. | | no
`remote_timeout` | `duration` | Timeout for requests made to the URL. | `"30s"` | no
`headers` | `map(string)` | Extra headers to deliver with the request. | | no
`send_exemplars` | `bool` | Whether exemplars should be sent. | `true` | no
`send_native_histograms` | `bool` | Whether native histograms should be sent. | `false` | no
`bearer_token_file`      | `string`            | File containing a bearer token to authenticate with.          |         | no
`bearer_token`           | `secret`            | Bearer token to authenticate with.                            |         | no
`enable_http2`           | `bool`              | Whether HTTP2 is supported for requests.                      | `true`  | no
`follow_redirects`       | `bool`              | Whether redirects returned by the server should be followed.  | `true`  | no
`proxy_url`              | `string`            | HTTP proxy to send requests through.                          |         | no
`no_proxy`               | `string`            | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying. | | no
`proxy_from_environment` | `bool`              | Use the proxy URL indicated by environment variables.         | `false` | no
`proxy_connect_header`   | `map(list(secret))` | Specifies headers to send to proxies during CONNECT requests. |         | no

 At most, one of the following can be provided:
 - [`bearer_token` argument](#endpoint-block).
 - [`bearer_token_file` argument](#endpoint-block).
 - [`basic_auth` block][basic_auth].
 - [`authorization` block][authorization].
 - [`oauth2` block][oauth2].
 - [`sigv4` block][sigv4].
 - [`azuread` block][azuread].

When multiple `endpoint` blocks are provided, metrics are concurrently sent to all
configured locations. Each endpoint has a _queue_ which is used to read metrics
from the WAL and queue them for sending. The `queue_config` block can be used
to customize the behavior of the queue.

Endpoints can be named for easier identification in debug metrics using the
`name` argument. If the `name` argument isn't provided, a name is generated
based on a hash of the endpoint settings.

When `send_native_histograms` is `true`, native Prometheus histogram samples
sent to `prometheus.remote_write` are forwarded to the configured endpoint. If
the endpoint doesn't support receiving native histogram samples, pushing
metrics fails.

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

### basic_auth block

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### authorization block

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### oauth2 block

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### sigv4 block

{{< docs/shared lookup="reference/components/sigv4-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### azuread block

{{< docs/shared lookup="reference/components/azuread-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### managed_identity block

{{< docs/shared lookup="reference/components/managed_identity-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### queue_config block

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`capacity` | `number` | Number of samples to buffer per shard. | `10000` | no
`min_shards` | `number` | Minimum amount of concurrent shards sending samples to the endpoint. | `1` | no
`max_shards` | `number` | Maximum number of concurrent shards sending samples to the endpoint. | `50` | no
`max_samples_per_send` | `number` | Maximum number of samples per send. | `2000` | no
`batch_send_deadline` | `duration` | Maximum time samples will wait in the buffer before sending. | `"5s"` | no
`min_backoff` | `duration` | Initial retry delay. The backoff time gets doubled for each retry. | `"30ms"` | no
`max_backoff` | `duration` | Maximum retry delay. | `"5s"` | no
`retry_on_http_429` | `bool` | Retry when an HTTP 429 status code is received. | `true` | no
`sample_age_limit` | `duration` | Maximum age of samples to send. | `"0s"` | no

Each queue then manages a number of concurrent _shards_ which is responsible
for sending a fraction of data to their respective endpoints. The number of
shards is automatically raised if samples are not being sent to the endpoint
quickly enough. The range of permitted shards can be configured with the
`min_shards` and `max_shards` arguments. Refer to  [Tuning `max_shards`](#tuning-max_shards)
for more information about how to configure `max_shards`.

Each shard has a buffer of samples it will keep in memory, controlled with the
`capacity` argument. New metrics aren't read from the WAL unless there is at
least one shard that is not at maximum capacity.

The buffer of a shard is flushed and sent to the endpoint either after the
shard reaches the number of samples specified by `max_samples_per_send` or the
duration specified by `batch_send_deadline` has elapsed since the last flush
for that shard.

Shards retry requests which fail due to a recoverable error. An error is
recoverable if the server responds with an `HTTP 5xx` status code. The delay
between retries can be customized with the `min_backoff` and `max_backoff`
arguments.

The `retry_on_http_429` argument specifies whether `HTTP 429` status code
responses should be treated as recoverable errors; other `HTTP 4xx` status code
responses are never considered recoverable errors. When `retry_on_http_429` is
enabled, `Retry-After` response headers from the servers are honored.

The `sample_age_limit` argument specifies the maximum age of samples to send. Any
samples older than the limit are dropped and won't be sent to the remote storage.
The default value is `0s`, which means that all samples are sent (feature is disabled).

### metadata_config block

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`send` | `bool` | Controls whether metric metadata is sent to the endpoint. | `true` | no
`send_interval` | `duration` | How frequently metric metadata is sent to the endpoint. | `"1m"` | no
`max_samples_per_send` | `number` | Maximum number of metadata samples to send to the endpoint at once. | `2000` | no

### write_relabel_config block

{{< docs/shared lookup="reference/components/write_relabel_config.md" source="alloy" version="<ALLOY_VERSION>" >}}

### wal block

The `wal` block customizes the Write-Ahead Log (WAL) used to temporarily store
metrics before they are sent to the configured set of endpoints.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`truncate_frequency` | `duration` | How frequently to clean up the WAL. | `"2h"` | no
`min_keepalive_time` | `duration` | Minimum time to keep data in the WAL before it can be removed. | `"5m"` | no
`max_keepalive_time` | `duration` | Maximum time to keep data in the WAL before removing it. | `"8h"` | no

The WAL serves two primary purposes:

* Buffer unsent metrics in case of intermittent network issues.
* Populate in-memory cache after a process restart.

The WAL is located inside a component-specific directory relative to the
storage path {{< param "PRODUCT_NAME" >}} is configured to use. See the
[`run` documentation][run] for how to change the storage path.

The `truncate_frequency` argument configures how often to clean up the WAL.
Every time the `truncate_frequency` period elapses, the lower two-thirds of
data is removed from the WAL and is no available for sending.

When a WAL clean-up starts, the lowest successfully sent timestamp is used to
determine how much data is safe to remove from the WAL. The
`min_keepalive_time` and `max_keepalive_time` control the permitted age range
of data in the WAL; samples aren't removed until they are at least as old as
`min_keepalive_time`, and samples are forcibly removed if they are older than
`max_keepalive_time`.

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`receiver` | `MetricsReceiver` | A value which other components can use to send metrics to.

## Component health

`prometheus.remote_write` is only reported as unhealthy if given an invalid
configuration. In those cases, exported fields are kept at their last healthy
values.

## Debug information

`prometheus.remote_write` does not expose any component-specific debug
information.

## Debug metrics

* `prometheus_remote_write_wal_storage_active_series` (gauge): Current number of active series
  being tracked by the WAL.
* `prometheus_remote_write_wal_storage_deleted_series` (gauge): Current number of series marked
  for deletion from memory.
* `prometheus_remote_write_wal_out_of_order_samples_total` (counter): Total number of out of
  order samples ingestion failed attempts.
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
  of exemplars which were dropped after being read from the WAL before being
  sent to remote_write because of an unknown reference ID.
* `prometheus_remote_storage_enqueue_retries_total` (counter): Total number of
  times enqueue has failed because a shard's queue was full.
* `prometheus_remote_storage_sent_batch_duration_seconds` (histogram): Duration
  of send calls to remote storage.
* `prometheus_remote_storage_queue_highest_sent_timestamp_seconds` (gauge):
  Unix timestamp of the latest WAL sample successfully sent by a queue.
* `prometheus_remote_storage_samples_pending` (gauge): The number of samples
  pending in shards to be sent to remote storage.
* `prometheus_remote_storage_exemplars_pending` (gauge): The number of
  exemplars pending in shards to be sent to remote storage.
* `prometheus_remote_storage_shard_capacity` (gauge): The capacity of shards
  within a given queue.
* `prometheus_remote_storage_shards` (gauge): The number of shards used for
  concurrent delivery of metrics to an endpoint.
* `prometheus_remote_storage_shards_min` (gauge): The minimum number of shards
  a queue is allowed to run.
* `prometheus_remote_storage_shards_max` (gauge): The maximum number of a
  shards a queue is allowed to run.
* `prometheus_remote_storage_shards_desired` (gauge): The number of shards a
  queue wants to run to be able to keep up with the amount of incoming metrics.
* `prometheus_remote_storage_bytes_total` (counter): Total number of bytes of
  data sent by queues after compression.
* `prometheus_remote_storage_metadata_bytes_total` (counter): Total number of
  bytes of metadata sent by queues after compression.
* `prometheus_remote_storage_max_samples_per_send` (gauge): The maximum number
  of samples each shard is allowed to send in a single request.
* `prometheus_remote_storage_samples_in_total` (counter): Samples read into
  remote storage.
* `prometheus_remote_storage_exemplars_in_total` (counter): Exemplars read into
  remote storage.

## Examples

The following examples show you how to create `prometheus.remote_write` components that send metrics to different destinations.

### Send metrics to a local Mimir instance

You can create a `prometheus.remote_write` component that sends your metrics to a local Mimir instance:

```alloy
prometheus.remote_write "staging" {
  // Send metrics to a locally running Mimir.
  endpoint {
    url = "http://mimir:9009/api/v1/push"

    basic_auth {
      username = "example-user"
      password = "example-password"
    }
  }
}

// Configure a prometheus.scrape component to send metrics to
// prometheus.remote_write component.
prometheus.scrape "demo" {
  targets = [
    // Collect metrics from the default HTTP listen address.
    {"__address__" = "127.0.0.1:12345"},
  ]
  forward_to = [prometheus.remote_write.staging.receiver]
}
```


### Send metrics to a Mimir instance with a tenant specified

You can create a `prometheus.remote_write` component that sends your metrics to a specific tenant within the Mimir instance.
This is useful when your Mimir instance is using more than one tenant:

```alloy
prometheus.remote_write "staging" {
  // Send metrics to a Mimir instance
  endpoint {
    url = "http://mimir:9009/api/v1/push"

    headers = {
      "X-Scope-OrgID" = "staging",
    }
  }
}
```

### Send metrics to a managed service

You can create a `prometheus.remote_write` component that sends your metrics to a managed service, for example, Grafana Cloud.
The Prometheus username and the Grafana Cloud API Key are injected in this example through environment variables.

```alloy
prometheus.remote_write "default" {
  endpoint {
    url = "https://prometheus-xxx.grafana.net/api/prom/push"
      basic_auth {
        username = env("PROMETHEUS_USERNAME")
        password = env("GRAFANA_CLOUD_API_KEY")
      }
  }
}
```

## Technical details

`prometheus.remote_write` uses [snappy][] for compression.

Any labels that start with `__` will be removed before sending to the endpoint.

### Data retention

The `prometheus.remote_write` component uses a Write Ahead Log (WAL) to prevent
data loss during network outages. The component buffers the received metrics in
a WAL for each configured endpoint. The queue shards can use the WAL after the
network outage is resolved and flush the buffered metrics to the endpoints.

The WAL records metrics in 128 MB files called segments. To avoid having a WAL
that grows on-disk indefinitely, the component _truncates_ its segments on a
set interval.

On each truncation, the WAL deletes references to series that are no longer
present and also _checkpoints_ roughly the oldest two thirds of the segments
(rounded down to the nearest integer) written to it since the last truncation
period. A checkpoint means that the WAL only keeps track of the unique
identifier for each existing metrics series, and can no longer use the samples
for remote writing. If that data has not yet been pushed to the remote
endpoint, it is lost.

This behavior dictates the data retention for the `prometheus.remote_write`
component. It also means that it's impossible to directly correlate data
retention directly to the data age itself, as the truncation logic works on
_segments_, not the samples themselves. This makes data retention less
predictable when the component receives a non-consistent rate of data.

The [WAL block][] contains some configurable parameters that can be used to control the tradeoff
between memory usage, disk usage, and data retention.

The `truncate_frequency` or `wal_truncate_frequency` parameter configures the
interval at which truncations happen. A lower value leads to reduced memory
usage, but also provides less resiliency to long outages.

When a WAL clean-up starts, the most recently successfully sent timestamp is
used to determine how much data is safe to remove from the WAL.
The `min_keepalive_time` or `min_wal_time` controls the minimum age of samples
considered for removal. No samples more recent than `min_keepalive_time` are
removed. The `max_keepalive_time` or `max_wal_time` controls the maximum age of
samples that can be kept in the WAL. Samples older than
`max_keepalive_time` are forcibly removed.

### Extended `remote_write` outages
When the remote write endpoint is unreachable over a period of time, the most
recent successfully sent timestamp is not updated. The
`min_keepalive_time` and `max_keepalive_time` arguments control the age range
of data kept in the WAL.

If the remote write outage is longer than the `max_keepalive_time` parameter,
then the WAL is truncated, and the oldest data is lost.

### Intermittent `remote_write` outages
If the remote write endpoint is intermittently reachable, the most recent
successfully sent timestamp is updated whenever the connection is successful.
A successful connection updates the series' comparison with
`min_keepalive_time` and triggers a truncation on the next `truncate_frequency`
interval which checkpoints two thirds of the segments (rounded down to the
nearest integer) written since the previous truncation.

### Falling behind
If the queue shards cannot flush data quickly enough to keep
up-to-date with the most recent data buffered in the WAL, we say that the
component is 'falling behind'.
It's not unusual for the component to temporarily fall behind 2 or 3 scrape intervals.
If the component falls behind more than one third of the data written since the
last truncate interval, it is possible for the truncate loop to checkpoint data
before being pushed to the remote_write endpoint.

### Tuning `max_shards`

The [`queue_config`](#queue_config-block) block allows you to configure `max_shards`. The `max_shards` is the maximum
number of concurrent shards sending samples to the Prometheus-compatible remote write endpoint.
For each shard, a single remote write request can send up to `max_samples_per_send` samples.

{{< param "PRODUCT_NAME" >}} will try not to use too many shards, but if the queue falls behind, the remote write
component will increase the number of shards up to `max_shards` to increase throughput. A high number of shards may
potentially overwhelm the remote endpoint or increase {{< param "PRODUCT_NAME" >}} memory utilization. For this reason,
it's important to tune `max_shards` to a reasonable value that is good enough to keep up with the backlog of data
to send to the remote endpoint without overwhelming it.

The maximum throughput that {{< param "PRODUCT_NAME" >}} can achieve when remote writing is equal to
`max_shards * max_samples_per_send * <1 / average write request latency>`. For example, running {{< param "PRODUCT_NAME" >}} with the
default configuration of 50 `max_shards` and 2000 `max_samples_per_send`, and assuming the
average latency of a remote write request is 500ms, the maximum throughput achievable is
about `50 * 2000 * (1s / 500ms) = 200K samples / s`.

The default `max_shards` configuration is good for most use cases, especially if each {{< param "PRODUCT_NAME" >}}
instance scrapes up to 1 million active series. However, if you run {{< param "PRODUCT_NAME" >}}
at a large scale and each instance scrapes more than 1 million series, we recommend
increasing the value of `max_shards`.

{{< param "PRODUCT_NAME" >}} exposes a few metrics that you can use to monitor the remote write shards:

* `prometheus_remote_storage_shards` (gauge): The number of shards used for concurrent delivery of metrics to an endpoint.
* `prometheus_remote_storage_shards_min` (gauge): The minimum number of shards a queue is allowed to run.
* `prometheus_remote_storage_shards_max` (gauge): The maximum number of shards a queue is allowed to run.
* `prometheus_remote_storage_shards_desired` (gauge): The number of shards a queue wants to run to keep up with the number of incoming metrics.

If you're already running {{< param "PRODUCT_NAME" >}}, a rule of thumb is to set `max_shards` to
4x shard utilization. Using the metrics explained above, you can run the following PromQL instant query
to compute the suggested `max_shards` value for each remote write endpoint `url`:

```
clamp_min(
    (
        # Calculate the 90th percentile desired shards over the last seven-day period.
        # If you're running {{< param "PRODUCT_NAME" >}} for less than seven days, then
        # reduce the [7d] period to cover only the time range since when you deployed it.
        ceil(quantile_over_time(0.9, prometheus_remote_storage_shards_desired[7d]))

        # Add room for spikes.
        * 4
    ),
    # We recommend setting max_shards to a value of no less than 50, as in the default configuration.
    50
)
```

If you aren't running {{< param "PRODUCT_NAME" >}} yet, we recommend running it with the default `max_shards`
and then using the PromQL instant query mentioned above to compute the recommended `max_shards`.

### WAL corruption

WAL corruption can occur when {{< param "PRODUCT_NAME" >}} unexpectedly stops
while the latest WAL segments are still being written to disk. For example, the
host computer has a general disk failure and crashes before you can stop
{{< param "PRODUCT_NAME" >}} and other running services. When you restart
{{< param "PRODUCT_NAME" >}}, it verifies the WAL, removing any corrupt
segments it finds. Sometimes, this repair is unsuccessful, and you must
manually delete the corrupted WAL to continue. If the WAL becomes corrupted,
{{< param "PRODUCT_NAME" >}} writes error messages such as
`err="failed to find segment for index"` to the log file.

{{< admonition type="note" >}}
Deleting a WAL segment or a WAL file permanently deletes the stored WAL data.
{{< /admonition >}}

To delete the corrupted WAL:

1. [Stop][] {{< param "PRODUCT_NAME" >}}.
1. Find and delete the contents of the `wal` directory.

   By default the `wal` directory is a subdirectory
   of the `data-alloy` directory located in the {{< param "PRODUCT_NAME" >}} working directory. The WAL data directory
   may be different than the default depending on the path specified by the [command line flag][run] `--storage-path`.

   {{< admonition type="note" >}}
   There is one `wal` directory per `prometheus.remote_write` component.
   {{< /admonition >}}

1. [Start][Stop] {{< param "PRODUCT_NAME" >}} and verify that the WAL is working correctly.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.remote_write` has exports that can be consumed by the following components:

- Components that consume [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

[snappy]: https://en.wikipedia.org/wiki/Snappy_(compression)
[WAL block]: #wal-block
[Stop]: ../../../../set-up/run/
[run]: ../../../cli/run/

---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/prometheus.remote_write/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/prometheus.remote_write/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/prometheus.remote_write/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.remote_write/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/prometheus.remote_write/
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

```river
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

{{< docs/shared lookup="flow/reference/components/http-client-proxy-config-description.md" source="agent" version="<AGENT_VERSION>" >}}

### basic_auth block

{{< docs/shared lookup="flow/reference/components/basic-auth-block.md" source="agent" version="<AGENT_VERSION>" >}}

### authorization block

{{< docs/shared lookup="flow/reference/components/authorization-block.md" source="agent" version="<AGENT_VERSION>" >}}

### oauth2 block

{{< docs/shared lookup="flow/reference/components/oauth2-block.md" source="agent" version="<AGENT_VERSION>" >}}

### sigv4 block

{{< docs/shared lookup="flow/reference/components/sigv4-block.md" source="agent" version="<AGENT_VERSION>" >}}

### azuread block

{{< docs/shared lookup="flow/reference/components/azuread-block.md" source="agent" version="<AGENT_VERSION>" >}}

### managed_identity block

{{< docs/shared lookup="flow/reference/components/managed_identity-block.md" source="agent" version="<AGENT_VERSION>" >}}

### tls_config block

{{< docs/shared lookup="flow/reference/components/tls-config-block.md" source="agent" version="<AGENT_VERSION>" >}}

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
`min_shards` and `max_shards` arguments.

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

{{< docs/shared lookup="flow/reference/components/write_relabel_config.md" source="agent" version="<AGENT_VERSION>" >}}

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
[`agent run` documentation][run] for how to change the storage path.

The `truncate_frequency` argument configures how often to clean up the WAL.
Every time the `truncate_frequency` period elapses, the lower two-thirds of
data is removed from the WAL and is no available for sending.

When a WAL clean-up starts, the lowest successfully sent timestamp is used to
determine how much data is safe to remove from the WAL. The
`min_keepalive_time` and `max_keepalive_time` control the permitted age range
of data in the WAL; samples aren't removed until they are at least as old as
`min_keepalive_time`, and samples are forcibly removed if they are older than
`max_keepalive_time`.

[run]: {{< relref "../cli/run.md" >}}

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

* `agent_wal_storage_active_series` (gauge): Current number of active series
  being tracked by the WAL.
* `agent_wal_storage_deleted_series` (gauge): Current number of series marked
  for deletion from memory.
* `agent_wal_out_of_order_samples_total` (counter): Total number of out of
  order samples ingestion failed attempts.
* `agent_wal_storage_created_series_total` (counter): Total number of created
  series appended to the WAL.
* `agent_wal_storage_removed_series_total` (counter): Total number of series
  removed from the WAL.
* `agent_wal_samples_appended_total` (counter): Total number of samples
  appended to the WAL.
* `agent_wal_exemplars_appended_total` (counter): Total number of exemplars
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

```river
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

You can create a `prometheus.remote_write` component that sends your metrics to a specific tenant within the Mimir instance. This is useful when your Mimir instance is using more than one tenant:

```river
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

You can create a `prometheus.remote_write` component that sends your metrics to a managed service, for example, Grafana Cloud. The Prometheus username and the Grafana Cloud API Key are injected in this example through environment variables.

```river
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

`prometheus.remote_write` uses [snappy](https://en.wikipedia.org/wiki/Snappy_(compression)) for compression.

Any labels that start with `__` will be removed before sending to the endpoint.

## Data retention

{{< docs/shared source="agent" lookup="/wal-data-retention.md" version="<AGENT_VERSION>" >}}

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.remote_write` has exports that can be consumed by the following components:

- Components that consume [Prometheus `MetricsReceiver`](../../compatibility/#prometheus-metricsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

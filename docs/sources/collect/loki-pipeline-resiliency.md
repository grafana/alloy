---
canonical: https://grafana.com/docs/alloy/latest/collect/loki-pipeline-resiliency/
description: Learn how to configure loki.write to survive a Loki outage with the write-ahead log, and how to plan around ingestion limits.
title: Configure Loki pipeline resiliency
menuTitle: Loki pipeline resiliency
weight: 255
---

# Configure Loki pipeline resiliency

The endpoint that `loki.write` sends to can become unreachable during an outage or a network failure.
You can configure {{< param "FULL_PRODUCT_NAME" >}} to keep accepting logs while that endpoint recovers, instead of stalling or dropping them.

To configure Loki pipeline resiliency, you:

1. Tune the retry behavior of a `loki.write` endpoint.
1. Enable the write-ahead log, or WAL, so ingestion doesn't stall.
1. Monitor the pipeline for dropped log entries.

> **EXPERIMENTAL**: The WAL feature of `loki.write` is [experimental][].
> Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.
> The `wal` block itself isn't gated behind the `stability.level` [flag][], but changing any `queue_config` argument is.

[flag]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/cli/run/
[experimental]: https://grafana.com/docs/release-life-cycle/

## Components used in this topic

- [`loki.source.api`][loki.source.api]
- [`loki.source.file`][loki.source.file]
- [`loki.write`][loki.write]

## Before you begin

- Ensure you have a pipeline that forwards logs through one or more `loki.source.*` components into a `loki.write` component.
- Identify how long an endpoint outage you need the pipeline to survive.
  This value determines how you size the WAL.

## Understand the default behavior during an outage

When `loki.write` can't reach its configured endpoint, it retries each batch with exponential backoff.
`max_backoff_retries` bounds how many retries follow the initial request.
Each wait starts at `min_backoff_period` and doubles toward `max_backoff_period`.

With the defaults of `"500ms"`, `"5m"`, and `10`, the configured backoff schedule adds up to 511.5 seconds, or about 8.5 minutes.
Elapsed time varies from that figure for two reasons.
`loki.write` randomizes each wait within the current backoff range.
Each request can also take up to `remote_timeout`, which defaults to `"10s"`.
What happens upstream while that retry loop runs depends on which component is feeding it:

- **`loki.source.file`**: stops reading new lines from the tailed file while the pipeline is stalled.
  This bounds the memory usage of {{< param "PRODUCT_NAME" >}}.
  However, {{< param "PRODUCT_NAME" >}} only detects rotation when it resumes reading.
  A single rotation is safe, because {{< param "PRODUCT_NAME" >}} drains the file it still has open before it switches to the new one.
  If the file rotates twice during the stall, the file created by the first rotation is replaced before {{< param "PRODUCT_NAME" >}} ever opens it.
  The lines written to that file are lost.
  There's no buffering of file-read position beyond the entries already handed to the pipeline.
- **Request-accepting components**: components such as `loki.source.api` hand each batch to the pipeline over an unbuffered channel.
  While `loki.write` is stalled, that handoff blocks and the HTTP request stays open.
  The component returns a `503` status once the request context ends, which happens when the client disconnects or gives up.
  Senders see slow requests and eventual failures rather than accepted writes.

In both cases, if `loki.write` exhausts its retries and drops a batch, the read position has already advanced.
Positions advance when a line enters the pipeline, not when it's confirmed delivered.
`loki.source.file` flushes the position to disk periodically rather than on every line.

The pipeline has no mechanism to report delivery failure back to the source component.
The result is a silent, incremental loss of log lines proportional to how long the outage lasts, rather than an all-or-nothing failure.

## Choose an approach

Four configurations trade off differently between ingestion stalls, durability, and exposure to the ingestion-age limits Loki applies:

| Configuration                          | Ingestion stalls? | Data survives {{< param "PRODUCT_NAME" >}} restart? | Subject to age limits on reconnect? |
| -------------------------------------- | ----------------- | --------------------------------------------------- | ----------------------------------- |
| Default backoff, no WAL                | Yes               | No                                                  | N/A, data already dropped           |
| `max_backoff_retries = 0`, no WAL      | Yes, indefinitely | No                                                  | Yes                                 |
| WAL enabled, default backoff           | No                | Only until retries are exhausted                    | Yes                                 |
| WAL enabled, `max_backoff_retries = 0` | No                | Yes, within `max_segment_age`                       | Yes                                 |

The WAL alone doesn't guarantee durability.
When an endpoint exhausts `max_backoff_retries`, `loki.write` drops the batch and marks that data consumed in the WAL, so the WAL never replays it.
Set `max_backoff_retries` to `0` alongside the WAL to hold data for the full `max_segment_age` window.

## Tune the retry behavior

By default, `loki.write` gives up on a batch once it exhausts `max_backoff_retries` and drops it.
You can trade that behavior for unbounded retry, which stops the slow trickle of dropped lines.

To tune the retry behavior of a `loki.write` endpoint, complete the following steps:

1. Locate the `loki.write` component in your configuration file and add `max_backoff_retries` to its `endpoint` block.

   ```alloy
   loki.write "<LABEL>" {
     endpoint {
       url = "<LOKI_URL>"

       max_backoff_retries = 0 // retry indefinitely on connection errors, 429 responses, and 5xx responses
     }
   }
   ```

   Replace the following:

   - _`<LABEL>`_: The label for the component.
     The label you use must be unique across all `loki.write` components in the same configuration file.
   - _`<LOKI_URL>`_: The full URL of the Loki endpoint where you send logs.

1. If you have more than one endpoint to write logs to, repeat the `endpoint` block for additional endpoints.

Unbounded retry doesn't buffer to disk.
If {{< param "PRODUCT_NAME" >}} restarts during the outage, in-memory data is still lost.

This isn't equivalent to WAL-backed durability.
It only changes how long `loki.write` keeps retrying before giving up.

## Enable the write-ahead log

With the WAL enabled, `loki.write` persists incoming log entries to disk before attempting delivery.
This means:

- Ingestion into the pipeline never stalls waiting on the remote endpoint.
  `loki.source.*` components keep reading and advancing normally.
- Buffered entries survive an {{< param "PRODUCT_NAME" >}} process restart during the outage, because they're on disk rather than only in memory.
  This is true for a component with a single `endpoint` block.
  All endpoints in one `loki.write` component share a single WAL marker file.
  With more than one endpoint, a restart can overwrite the recorded read position.
- The WAL is only as durable as the storage behind it.
  `loki.write` discards the error when a WAL write fails.
  A storage path that's full or otherwise unavailable drops the entry after the source component has already advanced its read position.
  Watch the {{< param "PRODUCT_NAME" >}} logs for the `failed to write entry` message, which is the only signal that this happened.
- The WAL doesn't extend the retry window on its own.
  An endpoint that exhausts `max_backoff_retries` drops the batch and reports that data as consumed, which lets the WAL reclaim the segment.
  Pair the WAL with `max_backoff_retries = 0` to remove that particular limit.
  Other drop paths still apply, including non-retryable responses, oversized batches, `max_streams`, and queue overflow.
  Refer to [Monitor for dropped log entries](#monitor-for-dropped-log-entries) for the full list.
- The `queue_config` block, when used without the WAL, bounds the in-memory send queue by size in bytes.
  The `capacity` argument defaults to `10MiB` per shard, so the send-queue budget is `capacity` multiplied by `min_shards`.
  It doesn't add durability, and changing any `queue_config` argument requires `--stability.level=experimental`.
  The WAL is the piece that removes the stall and drop behavior, not `queue_config` on its own.

By default, `loki.write` doesn't use the WAL.
To enable it, complete the following steps:

1. Add a `wal` block to your `loki.write` component.

   ```alloy
   loki.write "<LABEL>" {
     endpoint {
       url = "<LOKI_URL>"

       max_backoff_retries = 0
     }

     wal {
       enabled         = true
       max_segment_age = "<MAX_SEGMENT_AGE>"
     }
   }
   ```

   Replace the following:

   - _`<LABEL>`_: The label for the component.
   - _`<LOKI_URL>`_: The full URL of the Loki endpoint where you send logs.
   - _`<MAX_SEGMENT_AGE>`_: How long a WAL segment can live before {{< param "PRODUCT_NAME" >}} deletes it.
     The default is `1h`.
     Set this value to cover the longest outage you need to survive.
     {{< param "PRODUCT_NAME" >}} deletes segments older than this threshold whether or not Loki accepted them, apart from the highest-numbered segment.

   The `max_backoff_retries = 0` setting is required for the WAL to hold data for the full `max_segment_age` window.
   Leave `retry_on_http_429` at its default of `true` so that rate-limited batches are retried instead of dropped.

1. Confirm that the storage path {{< param "PRODUCT_NAME" >}} uses has room for `max_segment_age` worth of logs at your normal ingest rate.
   At an ingest rate of 5 MB/s with `max_segment_age` set to `4h`, budget roughly 72 GB.
   The WAL directory sits inside a component-specific directory relative to that path, and you can't configure it separately.

1. Confirm that the WAL is active after {{< param "PRODUCT_NAME" >}} reloads the configuration.
   {{< param "PRODUCT_NAME" >}} exports the following metrics only while the WAL is enabled, so their presence confirms the block took effect.

   | Metric                                         | Description                                                                           |
   | ---------------------------------------------- | ------------------------------------------------------------------------------------- |
   | `loki_write_wal_watcher_running`               | Number of WAL watchers running. A value of `0` means the WAL isn't active.            |
   | `loki_write_wal_writer_last_written_timestamp` | Latest timestamp written to the WAL. This value advances while entries arrive.        |
   | `loki_write_last_read_timestamp`               | Latest timestamp read from the WAL and queued for delivery, labeled by endpoint `id`. |

   During an outage the written timestamp keeps advancing while the read timestamp stalls.
   The gap between the two is the size of your backlog in time.

The `wal` block accepts `enabled`, `max_segment_age`, `min_read_frequency`, `max_read_frequency`, and `drain_timeout`.
There's no argument for the WAL directory and no maximum size argument.
Age is the only eviction policy.

The following example combines both procedures into a single component:

```alloy
loki.write "default" {
  endpoint {
    url = "<LOKI_URL>"

    max_backoff_retries = 0
  }

  wal {
    enabled         = true
    max_segment_age = "4h"
  }
}
```

Replace the following:

- _`<LOKI_URL>`_: The full URL of the Loki endpoint where you send logs.

The WAL doesn't remove the ingestion limits Loki applies to old data.
Refer to [Practical limits, even with the WAL](#practical-limits-even-with-the-wal) for details.

## Monitor for dropped log entries

Whether or not you enable the WAL, `loki.write` exports metrics that record when it discards data.
Use them to confirm your configuration holds up during a real outage.

To monitor the pipeline for dropped log entries, complete the following steps:

1. Collect the {{< param "PRODUCT_NAME" >}} metrics.
   Refer to [Monitor components][component_metrics] for the available options.

1. Alert on the following metrics.

   | Metric                             | Description                                                                                    |
   | ---------------------------------- | ---------------------------------------------------------------------------------------------- |
   | `loki_write_dropped_entries_total` | Number of log entries dropped, labeled by `reason`.                                            |
   | `loki_write_dropped_bytes_total`   | Number of bytes dropped, labeled by `reason`.                                                  |
   | `loki_write_batch_retries_total`   | Number of batch send retries. A sustained increase means the endpoint is becoming unreachable. |

   Exhausted retries are only one of the drop paths.
   The `reason` label identifies which one applied.

   | `reason`          | Cause                                                                                                             |
   | ----------------- | ----------------------------------------------------------------------------------------------------------------- |
   | `batch_too_large` | Loki rejected the batch as too large on the final attempt.                                                        |
   | `ingester_error`  | The final send attempt failed, either by exhausting `max_backoff_retries` or by returning a non-retryable status. |
   | `queue_is_full`   | The send queue was full and `block_on_overflow` is `false`.                                                       |
   | `rate_limited`    | Loki rate-limited the batch, either on the final attempt or immediately when `retry_on_http_429` is `false`.      |
   | `stream_limited`  | The entry would have exceeded `max_streams`.                                                                      |

   Alert on any increase in the two dropped-data counters, because a healthy pipeline never increments them.
   Retries happen during ordinary transient failures, so alert on `loki_write_batch_retries_total` only when the rate stays elevated for several minutes.

1. Search the {{< param "PRODUCT_NAME" >}} logs for the following messages.
   Two of these loss paths don't increment any drop counter, so the logs are the only signal.

   | Message                                                     | Meaning                                                                                      |
   | ----------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
   | `error encoding batch`                                      | A batch couldn't be encoded and was discarded. No drop counter increments.                   |
   | `failed to write entry`                                     | A WAL write failed, so the entry was lost after the source advanced its position. No drop counter increments. |
   | `final error sending batch, no retries left, dropping data` | A batch reached the end of its retry schedule. This path also increments the drop counters.  |

## Practical limits, even with the WAL

Data buffered during an outage only helps if Loki accepts it once connectivity is restored.
Two separate Loki limits reject old entries, and they're configured independently:

- **Out-of-order window**: Loki rejects an entry older than half of `max_chunk_age` for its stream, with `reason=too_far_behind`.
  The default `max_chunk_age` of `2h` gives a one hour window, and this setting is global rather than per tenant.
  On Grafana Cloud, automatic time-based stream sharding is enabled by default to keep this window from becoming user-visible.
  If you see this rejection regularly, raise it with support rather than treating it as expected.
- **Absolute sample age**: Loki rejects an entry older than `reject_old_samples_max_age`, with `reason=greater_than_max_sample_age`.
  This limit applies while `reject_old_samples` is `true`, which is the default, and the maximum age defaults to one week.
  You can override `reject_old_samples` and `reject_old_samples_max_age` per tenant through the Loki runtime overrides file.

In practice, three limits apply, and the shortest one decides the outcome.
{{< param "PRODUCT_NAME" >}} holds buffered data for `max_segment_age`, and only when you pair the WAL with `max_backoff_retries = 0`.
Loki then accepts the replayed entries only if they satisfy both its out-of-order window and its absolute age limit.

Compare `max_segment_age` against the cluster `max_chunk_age` and the effective `reject_old_samples_max_age` for your tenant.
Both Loki rejections return `400`, which `loki.write` doesn't retry, so those entries are dropped with `reason=ingester_error`.

Neither limit is controlled from the {{< param "PRODUCT_NAME" >}} side.
`reject_old_samples_max_age` is set in [`limits_config`][loki-limits], so a tenant override can change it.
`max_chunk_age` is set in the Loki `ingester` block and applies to the whole cluster.

Refer to [Enforce rate limits and push request validation][loki-validation] for what each rejection reason means.
Refer to [Automatic stream sharding][loki-ooo] for how Grafana Cloud keeps the out-of-order window from surfacing.

## Resource consumption during an outage

What `loki.write` consumes during an outage depends on whether you enable the WAL.
The WAL trades in-memory buffering for disk usage, and the two are bounded differently:

- **Disk**: the WAL doesn't grow for the full length of an outage.
  A cleanup routine deletes segments older than `max_segment_age`, whether or not Loki accepted them.
  The routine never deletes the highest-numbered segment, because the watcher may still be reading it, so one segment always survives regardless of age.
  The disk footprint settles at roughly `max_segment_age` worth of logs at your ingest rate, plus that retained segment.
  Age is the only eviction policy available.
  There's no maximum size argument and no ring buffer.
- **Memory**: without the WAL, `queue_config` bounds the send queue rather than process memory as a whole.
  `capacity` defaults to `10MiB` and `min_shards` defaults to `1`, so the default send-queue budget is `10MiB`.
  Raising `min_shards` multiplies that budget, because each shard gets its own queue.
  Each shard also holds one in-progress batch per tenant outside that budget, so memory grows with the number of tenants.
  Every shard keeps reusable encoding buffers sized from `batch_size` as well.
  The default `block_on_overflow` value of `true` makes `loki.write` wait for space once the queue is full, which slows the components feeding it.
  Setting `block_on_overflow` to `false` instead drops each newly arriving entry while the queue stays full.
  Nothing evicts entries that are already queued.

## Avoid data loss when connectivity is restored

When the endpoint becomes reachable again, a WAL-backed `loki.write` needs to drain its backlog without running entries into the ingestion-age limits Loki applies.
Three factors determine whether that drain succeeds:

- **Outage length**: the longer the outage, the closer buffered entries get to the out-of-order acceptance window by the time they're sent.
  Drain speed matters, not just buffering capacity.
- **Rate limiting**: a burst of buffered data replayed all at once can trip per-tenant ingestion limits.
  Watch for rate-limiting responses from Loki during backlog drain.
  Leave `retry_on_http_429` at its default of `true`, because setting it to `false` drops rate-limited batches immediately.
  A dropped batch is reported as consumed, so the WAL doesn't replay it.
- **Drain rate**: `loki.write` has no control over the rate at which it drains a backlog.
  Drain speed follows from the retry and backoff arguments and from `min_shards`, which sets how many shards send concurrently.
  `min_shards` is a `queue_config` argument, so changing it requires the experimental stability level.
  The `drain_timeout` arguments on the `wal` and `queue_config` blocks both default to `"15s"`.
  They cap how long a drain may take at shutdown rather than throttling a drain in normal operation.

## Next steps

- Review the [`loki.write`][loki.write] component reference for current WAL configuration arguments and stability status.
- Learn how to [Collect Kubernetes logs and forward them to Loki][logs-in-kubernetes].
- Learn how to [Monitor components][component_metrics] to track dropped log entries over time.

[component_metrics]: ../../troubleshoot/component_metrics/
[logs-in-kubernetes]: ../logs-in-kubernetes/
[loki-limits]: https://grafana.com/docs/loki/latest/configure/#limits_config
[loki-ooo]: https://grafana.com/docs/loki/latest/operations/automatic-stream-sharding/
[loki-validation]: https://grafana.com/docs/loki/latest/operations/request-validation-rate-limits/
[loki.source.api]: ../../reference/components/loki/loki.source.api/
[loki.source.file]: ../../reference/components/loki/loki.source.file/
[loki.write]: ../../reference/components/loki/loki.write/

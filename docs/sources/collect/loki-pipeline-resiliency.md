---
canonical: https://grafana.com/docs/alloy/latest/collect/loki-pipeline-resiliency/
description: Learn how to configure loki.write to survive a Loki outage with the write-ahead log, and how to plan around ingestion limits.
review_date: 2026-10-21
title: Configure Loki pipeline resiliency
menuTitle: Loki pipeline resiliency
weight: 255
---

# Configure Loki pipeline resiliency

The Grafana Loki endpoint that `loki.write` sends to can become unreachable during an outage or a network failure.
What {{< param "FULL_PRODUCT_NAME" >}} does next depends on how you configure the component.

To mitigate the effects of service interruptions and improve Loki pipeline resiliency, you:

1. Tune the retry behavior of a `loki.write` endpoint.
1. Enable the write-ahead log, or WAL, so ingestion doesn't stall.
1. Monitor the pipeline for dropped log entries.

Before you enable the WAL, plan its size around the ingestion limits Loki applies.

> **EXPERIMENTAL**: The WAL feature of `loki.write` is [experimental][].
> Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.
> To set `enabled = true` in the `wal` block or change any `queue_config` argument, you must set the `stability.level` [flag][] to `experimental`.

[flag]: ../../reference/cli/run/
[experimental]: https://grafana.com/docs/release-life-cycle/

## Components used in this topic

- [`loki.source.api`][loki.source.api]
- [`loki.source.file`][loki.source.file]
- [`loki.write`][loki.write]
- [`otelcol.exporter.loki`][otelcol.exporter.loki]

## Before you begin

- Ensure you have a pipeline that forwards logs into a `loki.write` component.
  The source can be one or more `loki.source.*` components, or an OpenTelemetry pipeline that ends at `otelcol.exporter.loki`.
- Identify your log throughput and the disk space available to {{< param "PRODUCT_NAME" >}}.
  The WAL configuration and your throughput together determine how long a service interruption the pipeline can support.
- Identify the label that names a single {{< param "PRODUCT_NAME" >}} process in your metrics.
  Every query uses _`<INSTANCE_LABEL>`_ for it.
  Standard Prometheus scrape configurations set `instance`, and Kubernetes deployments often use `pod`.
- If you plan to enable the WAL, run {{< param "PRODUCT_NAME" >}} with `--stability.level=experimental`.

## Understand the default behavior during an outage

When `loki.write` can't reach its configured endpoint, it retries each batch with exponential backoff.

### Retry schedule

`max_backoff_retries` sets the total number of send attempts, including the initial request.
Each wait is a random duration between the current backoff value and double that value.
The backoff value starts at `min_backoff_period` and doubles after each wait, up to `max_backoff_period`.

With the defaults of `"500ms"`, `"5m"`, and `10`, `loki.write` sends a batch up to 10 times, and the waits add up to between about 4 and 8.5 minutes.
Each request can also take up to `remote_timeout`, which defaults to `"10s"`.

### Effect on upstream components

What happens upstream while that retry loop runs depends on which component is feeding it:

- **`loki.source.file`**: stops reading new lines from the tailed file while `loki.write` blocks the pipeline.
  This bounds the memory usage of {{< param "PRODUCT_NAME" >}}.
  However, {{< param "PRODUCT_NAME" >}} only detects rotation when it resumes reading.
  A single rotation is safe, because {{< param "PRODUCT_NAME" >}} drains the file it still has open before it switches to the new one.
  If the file rotates twice during the stall, the second rotation replaces the file from the first rotation.
  {{< param "PRODUCT_NAME" >}} never opens that file and loses the lines written to it.
  The component doesn't buffer read positions beyond the entries it already handed to the pipeline.
- **Request-accepting components**: components such as `loki.source.api` hand each batch directly to the pipeline, with no buffer in between.
  While `loki.write` can't accept entries, that handoff blocks and the HTTP request stays open.
  The component returns a `503` status once the request context ends, which happens when the client disconnects or gives up.
  Senders see slow requests and eventual failures rather than accepted writes.
- **`otelcol.exporter.loki`**: converts the entire OpenTelemetry batch into an in-memory slice, then forwards the entries to `loki.write` one at a time.
  While `loki.write` can't accept entries, that forwarding blocks, and the block propagates back through the OpenTelemetry pipeline to the receiver.
  If the request context ends while the exporter waits, it stops forwarding and returns no error.
  The exporter loses the entries it hadn't forwarded yet, and no drop counter increments.

### Cost of a dropped batch

The pipeline has no mechanism to report delivery failure back to the source component, so what a dropped batch costs you depends on the source.

For `loki.source.file`, a dropped batch is invisible, and in most cases unrecoverable.
The component advances its read position when a line enters the pipeline, not when Loki confirms delivery.
It flushes that position to disk every 10 seconds, so by the time `loki.write` gives up, the position has already moved past those lines.

The result is a silent, incremental loss proportional to how long the outage lasts, rather than an all-or-nothing failure.
A graceful shutdown flushes the position before exiting, which makes that loss permanent.

If {{< param "PRODUCT_NAME" >}} terminates unexpectedly, the positions file still holds the offset from the last flush.
`loki.source.file` resumes from there and rereads every line written since, including lines that Loki already stored.

Request-accepting components don't store a position, and the sender holds the data.
A sender that receives a `503` knows the write didn't land and can retry it.
A sender that receives a success response has no such signal, because the component answers as soon as the pipeline accepts the batch.
`loki.write` can still drop that batch afterward, so treat an accepted write as queued rather than delivered.

The OpenTelemetry path gives the weakest signal of the three.
`otelcol.exporter.loki` returns no error when its context ends mid-batch, so the rest of the OpenTelemetry pipeline treats a partial forward as a success.
The exporter counts entries it converted rather than entries `loki.write` accepted, so its counters can't confirm delivery either.

## Choose an approach

Four configurations trade off differently between ingestion stalls, durability, and exposure to the ingestion-age limits Loki applies:

| Configuration                          | Ingestion stalls? | Data survives {{< param "PRODUCT_NAME" >}} restart? | Subject to age limits on reconnect?                        |
| -------------------------------------- | ----------------- | --------------------------------------------------- | ---------------------------------------------------------- |
| Default backoff, no WAL                | Yes               | No                                                  | Yes, if the endpoint recovers before retries are exhausted |
| `max_backoff_retries = 0`, no WAL      | Yes, indefinitely | No                                                  | Yes                                                        |
| WAL enabled, default backoff           | No                | Only until retries run out                          | Yes                                                        |
| WAL enabled, `max_backoff_retries = 0` | No                | Yes, within `max_segment_age`                       | Yes                                                        |

The WAL alone doesn't guarantee durability.
When an endpoint exhausts `max_backoff_retries`, `loki.write` drops the batch and marks that data consumed in the WAL, so the WAL never replays it.
To keep data during an outage without stalling ingestion, enable the WAL and set `max_backoff_retries` to `0`.
This combination holds data for the full `max_segment_age` window.

## Tune the retry behavior

By default, `loki.write` gives up on a batch once it exhausts `max_backoff_retries` and drops it.
You can trade that behavior for unbounded retry, which prevents a gradual loss of lines during a service interruption.

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

1. Set `max_backoff_retries` in each `endpoint` block.
   Each endpoint has its own retry configuration.

Unbounded retry keeps data in memory rather than on disk, so an {{< param "PRODUCT_NAME" >}} restart still loses whatever the queue held.
Setting `max_backoff_retries` to `0` changes how long `loki.write` keeps trying, not whether that data survives.

## Practical limits, even with the WAL

Data that {{< param "PRODUCT_NAME" >}} buffers during an outage only helps if Loki accepts it after the connection recovers.
Two separate Loki limits reject old entries, and they're configured independently:

- **Out-of-order window**: Loki rejects an entry older than half of `max_chunk_age` for its stream, with `reason=too_far_behind`.
  The default `max_chunk_age` of `2h` gives a one hour window.
  Loki sets `max_chunk_age` in the `ingester` block, so it applies to the whole cluster rather than per tenant.
  Grafana Cloud enables automatic stream sharding by default, which reduces how often streams reach this window.
  Refer to [Automatic stream sharding][loki-ooo] for details.
  If `too_far_behind` errors occur regularly, investigate them with your Loki administrator or Grafana Cloud support.
- **Absolute sample age**: Loki rejects an entry older than `reject_old_samples_max_age`, with `reason=greater_than_max_sample_age`.
  This limit applies while `reject_old_samples` is `true`, which is the default, and the maximum age defaults to one week.
  You can override `reject_old_samples` and `reject_old_samples_max_age` per tenant through the Loki runtime overrides file.

In practice, three limits apply, and the shortest one decides whether the data reaches Loki.
{{< param "PRODUCT_NAME" >}} holds buffered data for `max_segment_age`, the `wal` block argument that sets how long WAL segments live.
Loki then accepts the replayed entries only if they satisfy both its out-of-order window and its absolute age limit.

With the default `max_chunk_age` of `2h`, an entry has to reach Loki within one hour of the newest entry in its stream.
Raising `max_segment_age` beyond that only retains entries that Loki rejects.
Both Loki rejections return `400`, which `loki.write` doesn't retry, so `loki.write` drops those entries with `reason=ingester_error`.

{{< param "PRODUCT_NAME" >}} doesn't control either limit.
Loki sets `reject_old_samples_max_age` in [`limits_config`][loki-limits], so a tenant override can change it.
`max_chunk_age` has no per-tenant override, so changing it requires a cluster-wide configuration change.

Refer to [Enforce rate limits and push request validation][loki-validation] for what each rejection reason means.

## Resource consumption during an outage

What `loki.write` consumes during an outage depends on whether you enable the WAL.
The WAL trades in-memory buffering for disk usage, and different limits bound each one:

- **Disk**: the WAL doesn't grow for the full length of an outage.
  A cleanup routine deletes segments older than `max_segment_age`, whether or not Loki accepted them.
  The routine never deletes the highest-numbered segment, because the watcher may still be reading it, so one segment always survives regardless of age.
  The disk footprint settles at roughly `max_segment_age` worth of logs at your ingest rate, plus that retained segment.
  Age is the only eviction policy available.
  `loki.write` has no maximum size argument and no ring buffer.
- **Memory**: `queue_config` bounds the send queue of each endpoint, with or without the WAL, rather than process memory as a whole.
  `capacity` defaults to `10MiB` and `min_shards` defaults to `1`, so the default send-queue budget is `10MiB`.
  Raising `min_shards` multiplies that budget, because each shard gets its own queue.
  Raising `capacity` doesn't add durability.
  Each shard holds one in-progress batch per tenant outside that budget, so memory grows with the number of tenants.
  Every shard also keeps reusable encoding buffers that start at `batch_size` and grow to the largest batch it has sent.
  Without the WAL, the default `block_on_overflow` value of `true` makes `loki.write` wait for space once the queue is full, which stalls the components feeding it.
  With the WAL, a full queue pauses the WAL watcher instead, and the backlog stays on disk.
  Setting `block_on_overflow` to `false` instead drops an entry when it doesn't fit in the active batch for its tenant and the queue has no room for that full batch.
  Nothing evicts entries that are already queued.

## Enable the write-ahead log

With the WAL enabled, `loki.write` persists incoming log entries to disk before attempting delivery.
This means:

- Ingestion into the pipeline never stalls waiting on the remote endpoint.
  `loki.source.*` components keep reading and advancing normally.
- Buffered entries survive an {{< param "PRODUCT_NAME" >}} process restart during the outage, because they're on disk rather than only in memory.
  Each endpoint records its own progress, so after a restart each endpoint resumes from its own position.
- The WAL is only as durable as the storage behind it.
  When a WAL write fails, for example because the storage path is full, `loki.write` retries the write with backoff.
  The components that feed `loki.write` stall until the write succeeds.
  The retries only log a `failed to write entry, retrying` message at the `debug` level, so monitor the free space on the storage path.
- The WAL doesn't extend the retry window on its own.
  An endpoint that exhausts `max_backoff_retries` drops the batch and reports that data as consumed, which lets the WAL reclaim the segment.
  Pair the WAL with `max_backoff_retries = 0` to remove that particular limit.
  Other drop paths still apply, including non-retryable responses, oversized batches, `max_streams`, and queue overflow.
  Refer to [Monitor for dropped log entries][monitor-drops] for the full list.

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
     Set this value to cover the longest outage you need to survive, bounded by what Loki still accepts.
     Refer to [Practical limits, even with the WAL][practical-limits] for that upper bound.
     {{< param "PRODUCT_NAME" >}} deletes segments older than this threshold whether or not Loki accepted them, apart from the highest-numbered segment.

   The WAL needs `max_backoff_retries = 0` to hold data for the full `max_segment_age` window.
   Leave `retry_on_http_429` at its default of `true` so that `loki.write` retries rate-limited batches instead of dropping them.

   Run {{< param "PRODUCT_NAME" >}} with `--stability.level=experimental`.
   Otherwise, a configuration that sets `enabled = true` in the `wal` block fails to load.

   Set `name` on each `endpoint` block.
   {{< param "PRODUCT_NAME" >}} stores WAL progress per endpoint name.
   Without `name`, it derives the name from the URL and `tenant_id`, so changing either one makes the endpoint skip its unsent backlog.

1. Confirm that the `--storage.path` directory is on persistent storage and has room for `max_segment_age` worth of logs at your normal ingest rate.
   Rather than estimating your ingest rate, size the WAL from observed peak throughput.
   The query returns bytes for the busiest instance, and sums every `loki.write` component on it, because they share one storage path.

   ```promql
   max(
     max_over_time(
       sum by (<INSTANCE_LABEL>) (rate(loki_write_batch_size_bytes_sum[5m]))[7d:5m]
     )
   ) * <MAX_SEGMENT_AGE_HOURS> * 3600
   ```

   Replace the following:

   - _`<INSTANCE_LABEL>`_: The label that names a single {{< param "PRODUCT_NAME" >}} process.
   - _`<MAX_SEGMENT_AGE_HOURS>`_: The `max_segment_age` you chose, expressed in hours.

   The result is an upper bound on uncompressed batch volume, not just log-entry volume, because `loki_write_batch_size_bytes` includes stream-label bytes.
   {{< param "PRODUCT_NAME" >}} compresses WAL records with Snappy, which shrinks them, while record framing and 32 KiB page padding add to them.
   Cleanup also never deletes the highest-numbered segment, so up to 128 MiB persists beyond the `max_segment_age` window.

   The query overestimates in two more cases, both of which leave you with spare capacity rather than too little.
   A component with several `endpoint` blocks counts its volume once per endpoint, while the WAL stores each entry once.
   Components without a `wal` block add to the total without writing anything to the storage path.

   If the disk fills, WAL writes fail and the pipeline stalls until the cleanup routine deletes segments older than `max_segment_age`, or until you free space.
   The WAL directory sits inside a component-specific directory relative to that path, and you can't configure it separately.

1. Confirm that the WAL is active after {{< param "PRODUCT_NAME" >}} reloads the configuration.
   Check that `loki_write_wal_watcher_running` is greater than `0`.
   {{< param "PRODUCT_NAME" >}} decrements it when a watcher stops, so a value of `0` means the WAL isn't active.
   Check the value rather than whether the metric exists.
   {{< param "PRODUCT_NAME" >}} keeps the metrics registry for the component's lifetime, so the metric remains exported after a reload turns the WAL off.

1. After the WAL runs longer than `max_segment_age`, refine the size estimate.
   This query reports the ratio of WAL bytes on disk to raw log bytes, aggregated the same way as the sizing query:

   ```promql
   sum by (<INSTANCE_LABEL>) (rate(loki_write_wal_writer_reclaimed_space[6h]))
     / sum by (<INSTANCE_LABEL>) (rate(loki_write_batch_size_bytes_sum[6h]))
   ```

   Multiply the sizing result by that ratio, then leave headroom on top.

## Monitor for dropped log entries

Whether or not you enable the WAL, `loki.write` exports metrics that record when it discards data.
Use them to confirm your configuration works during an outage.

To monitor the pipeline for dropped log entries, complete the following steps:

1. Collect the {{< param "PRODUCT_NAME" >}} self-monitoring metrics.
   Refer to [Monitor components][component_metrics] for the available options.

1. Alert on the following metrics.

   | Metric                             | Description                                                                                       |
   | ---------------------------------- | ------------------------------------------------------------------------------------------------- |
   | `loki_write_dropped_entries_total` | Number of log entries dropped, labeled by `reason`.                                               |
   | `loki_write_dropped_bytes_total`   | Number of bytes dropped, labeled by `reason`. Increments on the same events as the entry counter. |
   | `loki_write_batch_retries_total`   | Number of batch send retries. A sustained increase means the endpoint is becoming unreachable.    |

   Exhausted retries are only one of the drop paths.
   The `reason` label identifies which one applied.

   | `reason`          | Cause                                                                                                             |
   | ----------------- | ----------------------------------------------------------------------------------------------------------------- |
   | `batch_too_large` | Loki rejected the batch as too large on the final attempt.                                                        |
   | `ingester_error`  | The final send attempt failed, either by exhausting `max_backoff_retries` or by returning a non-retryable status. |
   | `queue_is_full`   | The send queue was full and `block_on_overflow` is `false`.                                                       |
   | `rate_limited`    | Loki rate-limited the batch, either on the final attempt or immediately when `retry_on_http_429` is `false`.      |
   | `stream_limited`  | The entry would have exceeded `max_streams`.                                                                      |

   Alert on any increase in `loki_write_dropped_entries_total` for reasons other than `rate_limited` and `stream_limited`, because a healthy pipeline never increments those other reasons on its own.
   Busy tenants can reach `rate_limited` and `stream_limited` during normal operation, so track those two reasons on a separate threshold instead of alerting on any increase.
   A `rate_limited` drop happens after retries run out, unless `retry_on_http_429` is `false`.
   A `stream_limited` drop happens immediately, without retrying.
   `loki_write_dropped_bytes_total` increments on the same events, so it tells you how much data you lost rather than whether you lost any.
   Retries happen during ordinary transient failures, so alert on `loki_write_batch_retries_total` only when the rate stays elevated for several minutes.

   This query lists every active drop path that increments the counter:

   ```promql
   sum by (<INSTANCE_LABEL>, component_path, component_id, reason) (rate(loki_write_dropped_entries_total[5m])) > 0
   ```

   For entries that reach a batch, {{< param "PRODUCT_NAME" >}} also records propagation latency, measured from entry creation until `loki.write` sends or drops the batch.
   Entries dropped before they reach a batch, such as `queue_is_full` drops, don't contribute to this metric.
   This query reports the 99th percentile in seconds:

   ```promql
   histogram_quantile(0.99, sum by (le, <INSTANCE_LABEL>, component_path, component_id) (rate(loki_write_entry_propagation_latency_seconds_bucket[5m])))
   ```

   This latency isn't the WAL backlog, and it isn't what the Loki age limits measure.
   Use the WAL timestamp gauges for backlog expressed in log time.

1. If you enable the WAL, alert before the backlog reaches the shortest age limit that applies to it.
   You don't have to estimate the outage and drain durations to know whether you're approaching that limit.
   Compare the two WAL timestamp gauges instead, because the gap between them tracks how far the watcher trails the writer.

   | Metric                                         | Description                                                                                    |
   | ---------------------------------------------- | ---------------------------------------------------------------------------------------------- |
   | `loki_write_wal_writer_last_written_timestamp` | Timestamp of the newest log entry written to the WAL.                                          |
   | `loki_write_last_read_timestamp`               | Timestamp of the newest log entry read from the WAL and queued for delivery, by endpoint `id`. |

   Both gauges report log entry timestamps rather than wall-clock times, so the gap between them approximates the backlog rather than measuring it exactly.
   During an outage the written timestamp keeps advancing.
   The read timestamp advances until the endpoint queue fills, and stalls after that.

   The following alert fires when that gap passes half an hour, which leaves time to react before the default one hour window closes:

   ```promql
   loki_write_wal_writer_last_written_timestamp
     - on (<INSTANCE_LABEL>, component_path, component_id) group_right loki_write_last_read_timestamp
     > 1800
   ```

   Both gauges stall together when no logs arrive, so an idle pipeline reports a gap of zero rather than triggering the alert.
   Set the threshold below the shortest of three limits, expressed in seconds: half of the cluster `max_chunk_age`, the `reject_old_samples_max_age` for the tenant, and `max_segment_age`.
   Leave enough margin for the backlog to drain after the endpoint recovers.
   Refer to [Practical limits, even with the WAL][practical-limits] for how Loki sets its two limits.

   The read gauge advances when the watcher queues an entry, not when Loki accepts it.
   The gap therefore measures the WAL backlog and stops at the endpoint queue.

   A stalled sender can hold old entries in that queue while the gap stays small.
   Pair this alert with `loki_write_batch_retries_total` and the drop counters so a stuck endpoint still triggers an alert.

   When cleanup deletes a segment that an endpoint hasn't sent yet, {{< param "PRODUCT_NAME" >}} loses those entries, and no drop counter increments.

1. Search the {{< param "PRODUCT_NAME" >}} logs for the following messages.
   Three of these drop paths never increment a `loki_write_*` counter, so a metric alone won't reveal them.

   | Message                                                     | Meaning                                                                                                    |
   | ----------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
   | `dropping entry, write did not succeed before shutdown`     | {{< param "PRODUCT_NAME" >}} shut down before `loki.write` accepted the entry. No drop counter increments. |
   | `error encoding batch`                                      | `loki.write` couldn't encode a batch and discarded it. No drop counter increments.                         |
   | `failed to convert log to loki entry`                       | `otelcol.exporter.loki` skipped an OpenTelemetry record it couldn't translate.                             |
   | `final error sending batch, no retries left, dropping data` | A batch reached the end of its retry schedule. This path also increments the drop counters.                |

   When an OpenTelemetry pipeline feeds `loki.write`, also track `otelcol_exporter_loki_entries_failed`, which counts skipped records.
   Compare `otelcol_exporter_loki_entries_processed` against the `loki.write` counters rather than reading it as a delivery count.

## Avoid data loss when the connection recovers

When the endpoint becomes reachable again, a WAL-backed `loki.write` needs to drain its backlog without running entries into the ingestion-age limits Loki applies.
Three factors determine whether that drain succeeds:

- **Outage length**: the longer the outage, the closer buffered entries get to the out-of-order window by the time `loki.write` sends them.
  Drain speed matters, not just buffering capacity.
  Refer to [Practical limits, even with the WAL][practical-limits] for how Loki sets that window.
- **Rate limiting**: a burst of buffered data replayed all at once can trip per-tenant ingestion limits.
  Watch for rate-limiting responses from Loki during backlog drain.
  Leave `retry_on_http_429` at its default of `true`, because setting it to `false` drops rate-limited batches immediately.
  `loki.write` reports a dropped batch as consumed, so the WAL doesn't replay it.
- **Drain rate**: `loki.write` has no dedicated throttle for WAL replay.
  Drain throughput still follows from the retry and backoff arguments, from `batch_wait` and `batch_size`, and from `min_shards`, which sets how many shards send concurrently.
  `min_shards` is a `queue_config` argument, so changing it requires the experimental stability level.
  The `drain_timeout` arguments on the `wal` and `queue_config` blocks both default to `"15s"`.
  They cap how long a drain may take at shutdown and don't affect drain in normal operation.

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
[monitor-drops]: #monitor-for-dropped-log-entries
[otelcol.exporter.loki]: ../../reference/components/otelcol/otelcol.exporter.loki/
[practical-limits]: #practical-limits-even-with-the-wal

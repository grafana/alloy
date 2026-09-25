---
canonical: https://grafana.com/docs/alloy/latest/collect/loki-pipeline-resiliency/
description: Learn how to configure loki.write to survive a Loki outage with the write-ahead log, and how to plan around ingestion limits.
title: Configure Loki pipeline resiliency
menuTitle: Loki pipeline resiliency
weight: 255
---

# Configure Loki pipeline resiliency

The endpoint that `loki.write` sends to can become unreachable during an outage or a network failure.
What {{< param "PRODUCT_NAME" >}} does next depends on how you configure the component.

To mitigate the effects of service interruptions and improve Loki pipeline resiliency, you:

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
- [`otelcol.exporter.loki`][otelcol.exporter.loki]

## Before you begin

- Ensure you have a pipeline that forwards logs into a `loki.write` component.
  The source can be one or more `loki.source.*` components, or an OpenTelemetry pipeline that ends at `otelcol.exporter.loki`.
- Identify your log throughput and the disk space available to {{< param "PRODUCT_NAME" >}}.
  The WAL configuration and your throughput together determine how long a service interruption the pipeline can support.
- Identify the label that names a single {{< param "PRODUCT_NAME" >}} process in your metrics.
  Every query uses _`<INSTANCE_LABEL>`_ for it.
  Standard Prometheus scrape configurations set `instance`, and Kubernetes deployments often use `pod`.

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
- **Request-accepting components**: components such as `loki.source.api` hand each batch directly to the pipeline, with no buffer in between.
  While `loki.write` is stalled, that handoff blocks and the HTTP request stays open.
  The component returns a `503` status once the request context ends, which happens when the client disconnects or gives up.
  Senders see slow requests and eventual failures rather than accepted writes.
- **`otelcol.exporter.loki`**: converts each OpenTelemetry log record, then forwards the entries to `loki.write` one at a time with no buffer in between.
  While `loki.write` is stalled, that forwarding blocks, and the block propagates back through the OpenTelemetry pipeline to the receiver.
  If the request context ends while the exporter is blocked, it stops forwarding and returns no error.
  The entries it hadn't forwarded yet are lost, and no drop counter increments.

The pipeline has no mechanism to report delivery failure back to the source component, so what a dropped batch costs you depends on the source.

For file tailers, a dropped batch is invisible, and in most cases unrecoverable.
`loki.source.file` advances its read position when a line enters the pipeline, not when Loki confirms delivery.
It flushes that position to disk every 10 seconds, so by the time `loki.write` gives up, the position has already moved past those lines.

The result is a silent, incremental loss proportional to how long the outage lasts, rather than an all-or-nothing failure.
A graceful shutdown flushes the position before exiting, which makes that loss permanent.

If {{< param "PRODUCT_NAME" >}} terminates unexpectedly, the positions file still holds the offset from the last flush.
The tailer resumes from there and rereads every line written since, including lines that Loki already stored.

For request-accepting components, there's no stored position, and the sender holds the data.
A sender that receives a `503` knows the write didn't land and can retry it.
A sender that receives a success response has no such signal, because the component answers as soon as the pipeline accepts the batch.
`loki.write` can still drop that batch afterward, so treat an accepted write as queued rather than delivered.

The OpenTelemetry path gives the weakest signal of the three.
`otelcol.exporter.loki` returns no error when its context ends mid-batch, so the rest of the OpenTelemetry pipeline treats a partial forward as a success.
The exporter counts entries it converted rather than entries `loki.write` accepted, so its counters can't confirm delivery either.

## Choose an approach

Four configurations trade off differently between ingestion stalls, durability, and exposure to the ingestion-age limits Loki applies:

| Configuration                          | Ingestion stalls? | Data survives {{< param "PRODUCT_NAME" >}} restart? | Subject to age limits on reconnect? |
| -------------------------------------- | ----------------- | --------------------------------------------------- | ----------------------------------- |
| Default backoff, no WAL                | Yes               | No                                                  | N/A, data already dropped           |
| `max_backoff_retries = 0`, no WAL      | Yes, indefinitely | No                                                  | Yes                                 |
| WAL enabled, default backoff           | No                | Only until retries are exhausted                    | Yes                                 |
| WAL enabled, `max_backoff_retries = 0` | No                | Yes, within `max_segment_age`, single endpoint only | Yes                                 |

The WAL alone doesn't guarantee durability.
When an endpoint exhausts `max_backoff_retries`, `loki.write` drops the batch and marks that data consumed in the WAL, so the WAL never replays it.
Set `max_backoff_retries` to `0` alongside the WAL to hold data for the full `max_segment_age` window.

## Tune the retry behavior

By default, `loki.write` gives up on a batch once it exhausts `max_backoff_retries` and drops it.
You can trade that behavior for unbounded retry, which removes the possibility of a slow trickle of dropped lines during a service interruption.

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
   If you also plan to enable the WAL, define a separate `loki.write` component for each endpoint instead.
   Refer to [Enable the write-ahead log](#enable-the-write-ahead-log) for the reason.

Unbounded retry keeps data in memory rather than on disk, so an {{< param "PRODUCT_NAME" >}} restart still loses whatever was queued.
Setting `max_backoff_retries` to `0` changes how long `loki.write` keeps trying, not whether that data survives.

## Enable the write-ahead log

With the WAL enabled, `loki.write` persists incoming log entries to disk before attempting delivery.
This means:

- Ingestion into the pipeline never stalls waiting on the remote endpoint.
  `loki.source.*` components keep reading and advancing normally.
- Buffered entries survive an {{< param "PRODUCT_NAME" >}} process restart during the outage, because they're on disk rather than only in memory.
  This holds only for a component with a single `endpoint` block.
  All endpoints in one `loki.write` component share a single WAL marker file, and each endpoint advances it from its own delivery progress alone.
  A healthy endpoint can therefore mark a segment that still holds entries an unreachable endpoint hasn't sent.
  After a restart, the watcher for the unreachable endpoint resumes after the marked segment and never replays those entries.
  Define one WAL-enabled `loki.write` component per endpoint so that each one keeps its own marker.
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
     Set this value to cover the longest outage you need to survive, bounded by what Loki still accepts.
     Refer to [Practical limits, even with the WAL](#practical-limits-even-with-the-wal) for that upper bound.
     {{< param "PRODUCT_NAME" >}} deletes segments older than this threshold whether or not Loki accepted them, apart from the highest-numbered segment.

   The `max_backoff_retries = 0` setting is required for the WAL to hold data for the full `max_segment_age` window.
   Leave `retry_on_http_429` at its default of `true` so that rate-limited batches are retried instead of dropped.

1. Confirm that the storage path {{< param "PRODUCT_NAME" >}} uses has room for `max_segment_age` worth of logs at your normal ingest rate.
   Rather than estimating your ingest rate, size the WAL from observed peak throughput.
   The query returns bytes for the busiest instance, and sums every `loki.write` component on it, because they share one disk.

   ```promql
   max(
     max_over_time(
       sum by (<INSTANCE_LABEL>) (rate(loki_write_sent_bytes_total[5m]))[7d:5m]
     )
   ) * <MAX_SEGMENT_AGE_HOURS> * 3600
   ```

   Replace the following:

   - _`<INSTANCE_LABEL>`_: The label that names a single {{< param "PRODUCT_NAME" >}} process.
   - _`<MAX_SEGMENT_AGE_HOURS>`_: The `max_segment_age` you chose, expressed in hours.

   The result is the uncompressed log volume in bytes rather than the size of the WAL on disk.
   {{< param "PRODUCT_NAME" >}} compresses WAL records with Snappy, which shrinks them, while record framing and 32 KiB page padding add to them.
   Cleanup also never deletes the highest-numbered segment, so up to 128 MiB persists beyond the `max_segment_age` window.

   After the WAL has run longer than `max_segment_age`, this query reports the ratio of WAL bytes on disk to raw log bytes:

   ```promql
   sum by (<INSTANCE_LABEL>, component_id) (rate(loki_write_wal_writer_reclaimed_space[6h]))
     / sum by (<INSTANCE_LABEL>, component_id) (rate(loki_write_sent_bytes_total[6h]))
   ```

   Multiply the sizing result by that ratio, then leave headroom on top.
   A WAL write that fails on a full disk loses the entry, and the only signal is a `failed to write entry` log message.
   The WAL directory sits inside a component-specific directory relative to that path, and you can't configure it separately.

1. Confirm that the WAL is active after {{< param "PRODUCT_NAME" >}} reloads the configuration.
   Check the value of `loki_write_wal_watcher_running` rather than checking whether the metric exists.
   {{< param "PRODUCT_NAME" >}} keeps the metrics registry for the component's lifetime, so these metrics remain exported after a reload turns the WAL off.

   | Metric                                         | Description                                                                                                                             |
   | ---------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
   | `loki_write_wal_watcher_running`               | Number of WAL watchers running. {{< param "PRODUCT_NAME" >}} decrements it when a watcher stops, so a value of `0` means the WAL isn't active. |
   | `loki_write_wal_writer_last_written_timestamp` | Timestamp of the newest log entry written to the WAL.                                                                                   |
   | `loki_write_last_read_timestamp`               | Timestamp of the newest log entry read from the WAL and queued for delivery, by endpoint `id`.                                          |

   Both gauges report log entry timestamps rather than wall-clock times, so the gap between them approximates the backlog rather than measuring it exactly.
   During an outage the written timestamp keeps advancing.
   The read timestamp advances until the endpoint queue fills, and stalls after that.

   This query reports how far the delivery pointer trails real time, in seconds:

   ```promql
   time() - loki_write_last_read_timestamp
   ```

The `wal` block accepts `enabled`, `max_segment_age`, `min_read_frequency`, `max_read_frequency`, and `drain_timeout` arguments.
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

   Alert on any increase in `loki_write_dropped_entries_total`, because a healthy pipeline never increments it.
   `loki_write_dropped_bytes_total` increments on the same events, so it tells you how much data you lost rather than whether you lost any.
   Retries happen during ordinary transient failures, so alert on `loki_write_batch_retries_total` only when the rate stays elevated for several minutes.

   This query lists every drop path that's currently active:

   ```promql
   sum by (<INSTANCE_LABEL>, component_id, reason) (rate(loki_write_dropped_entries_total[5m])) > 0
   ```

   {{< param "PRODUCT_NAME" >}} also records how long entries take to reach their final disposition.
   This query reports the 99th percentile in seconds, which is the drain time to compare against the Loki age limits:

   ```promql
   histogram_quantile(0.99, sum by (le, <INSTANCE_LABEL>, component_id) (rate(loki_write_entry_propagation_latency_seconds_bucket[5m])))
   ```

1. Search the {{< param "PRODUCT_NAME" >}} logs for the following messages.
   Two of these loss paths don't increment any drop counter, so the logs are the only signal.

   | Message                                                     | Meaning                                                                                                       |
   | ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
   | `error encoding batch`                                      | A batch couldn't be encoded and was discarded. No drop counter increments.                                    |
   | `failed to convert log to loki entry`                       | `otelcol.exporter.loki` skipped an OpenTelemetry record it couldn't translate.                                |
   | `failed to write entry`                                     | A WAL write failed, so the entry was lost after the source advanced its position. No drop counter increments. |
   | `final error sending batch, no retries left, dropping data` | A batch reached the end of its retry schedule. This path also increments the drop counters.                   |

   When an OpenTelemetry pipeline feeds `loki.write`, also track `otelcol_exporter_loki_entries_failed`, which counts skipped records.
   Compare `otelcol_exporter_loki_entries_processed` against the `loki.write` counters rather than reading it as a delivery count.

## Practical limits, even with the WAL

Data buffered during an outage only helps if Loki accepts it once connectivity is restored.
Two separate Loki limits reject old entries, and they're configured independently:

- **Out-of-order window**: Loki rejects an entry older than half of `max_chunk_age` for its stream, with `reason=too_far_behind`.
  The default `max_chunk_age` of `2h` gives a one hour window, and this setting is global rather than per tenant.
  Grafana Cloud enables automatic stream sharding by default, which reduces how often streams reach this window.
  Regular `too_far_behind` errors should be investigated with your Loki administrator or Grafana Cloud support.
- **Absolute sample age**: Loki rejects an entry older than `reject_old_samples_max_age`, with `reason=greater_than_max_sample_age`.
  This limit applies while `reject_old_samples` is `true`, which is the default, and the maximum age defaults to one week.
  You can override `reject_old_samples` and `reject_old_samples_max_age` per tenant through the Loki runtime overrides file.

In practice, three limits apply, and the shortest one will decide if data is dropped.
{{< param "PRODUCT_NAME" >}} holds buffered data for `max_segment_age`.
Loki then accepts the replayed entries only if they satisfy both its out-of-order window and its absolute age limit.

With the default `max_chunk_age` of `2h`, an entry has to reach Loki within one hour of the newest entry in its stream.
Raising `max_segment_age` beyond that only retains entries that Loki rejects.
Both Loki rejections return `400`, which `loki.write` doesn't retry, so those entries are dropped with `reason=ingester_error`.

You don't have to estimate the outage and drain durations to know whether you're approaching that limit.
The following alert fires when delivery falls half an hour behind, which leaves time to react before the one hour window closes:

```promql
time() - loki_write_last_read_timestamp > 1800
```

Set the threshold below your own window, which is half of the cluster `max_chunk_age` expressed in seconds.

Neither limit is controlled from {{< param "PRODUCT_NAME" >}}.
`reject_old_samples_max_age` is set in [`limits_config`][loki-limits], so a tenant override can change it.
`max_chunk_age` is set in the Loki `ingester` block and applies to the whole cluster.

Refer to [Enforce rate limits and push request validation][loki-validation] for what each rejection reason means.
Refer to [Automatic stream sharding][loki-ooo] for details.

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
  The default `block_on_overflow` value of `true` makes `loki.write` wait for space once the queue is full, which stalls the components feeding it.
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
- **Drain rate**: `loki.write` has no dedicated throttle for WAL replay.
  Drain throughput still follows from the retry and backoff arguments, from `batch_wait` and `batch_size`, and from `min_shards`, which sets how many shards send concurrently.
  `min_shards` is a `queue_config` argument, so changing it requires the experimental stability level.
  The `drain_timeout` arguments on the `wal` and `queue_config` blocks both default to `"15s"`.
  They cap how long a drain may take at shutdown and do not affect drain in normal operation.

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
[otelcol.exporter.loki]: ../../reference/components/otelcol/otelcol.exporter.loki/

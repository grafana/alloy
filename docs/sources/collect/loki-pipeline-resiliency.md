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
> To enable and use an experimental feature, you must set the `stability.level` [flag][] to `experimental`.

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
The wait starts at `min_backoff_period` and doubles toward `max_backoff_period`, for at most `max_backoff_retries` attempts.
`loki.write` randomizes each wait within the current backoff range, so the total isn't fixed.

With the defaults of `"500ms"`, `"5m"`, and `10`, `loki.write` retries a batch for roughly 9 to 13 minutes, then drops it.
What happens upstream while that retry loop runs depends on which component is feeding it:

- **`loki.source.file`**: stops reading new lines from the tailed file while the pipeline is stalled.
  This bounds the memory usage of {{< param "PRODUCT_NAME" >}}.
  However, {{< param "PRODUCT_NAME" >}} only detects rotation when it resumes reading.
  A single rotation is safe, because {{< param "PRODUCT_NAME" >}} drains the file it still has open before it switches to the new one.
  If the file rotates twice during the stall, the file created by the first rotation is replaced before {{< param "PRODUCT_NAME" >}} ever opens it.
  The lines written to that file are lost.
  There's no buffering of file-read position beyond the entries already handed to the pipeline.
- **Request-accepting components**: components such as `loki.source.api` start timing out incoming requests once they can no longer hand data off to a stalled `loki.write`.

In both cases, if `loki.write` exhausts its retries and drops a batch, the read position has already advanced.
Positions advance when a line enters the pipeline, not when it's confirmed delivered.
`loki.source.file` flushes the position to disk periodically rather than on every line.

The pipeline has no mechanism to report delivery failure back to the source component.
The result is a silent, incremental loss of log lines proportional to how long the outage lasts, rather than an all-or-nothing failure.

## Choose an approach

Three configurations trade off differently between ingestion stalls, durability, and exposure to the ingestion-age limits Loki applies:

| Configuration                     | Ingestion stalls? | Data survives {{< param "PRODUCT_NAME" >}} restart? | Subject to age limits on reconnect? |
| --------------------------------- | ----------------- | --------------------------------------------------- | ----------------------------------- |
| Default backoff, no WAL           | Yes               | No                                                  | N/A, data already dropped           |
| `max_backoff_retries = 0`, no WAL | Yes, indefinitely | No                                                  | Yes                                 |
| WAL enabled                       | No                | Yes, within `max_segment_age`                       | Yes                                 |

The WAL is the only configuration that keeps ingestion moving and survives a restart.

## Tune the retry behavior

By default, `loki.write` gives up on a batch after `max_backoff_retries` attempts and drops it.
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
- The `queue_config` block, when used without the WAL, bounds the in-memory send queue by size in bytes.
  The `capacity` argument defaults to `10MiB` per shard, so the total ceiling is `capacity` multiplied by `min_shards`.
  It doesn't add durability, and changing any `queue_config` argument requires `--stability.level=experimental`.
  The WAL is the piece that removes the stall and drop behavior, not `queue_config` on its own.

By default, `loki.write` doesn't use the WAL.
To enable it, complete the following steps:

1. Add a `wal` block to your `loki.write` component.

   ```alloy
   loki.write "<LABEL>" {
     endpoint {
       url = "<LOKI_URL>"
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
     {{< param "PRODUCT_NAME" >}} deletes segments older than this threshold whether or not Loki accepted them.

1. Confirm that the storage path {{< param "PRODUCT_NAME" >}} uses has room for `max_segment_age` worth of logs at your normal ingest rate.
   At an ingest rate of 5 MB/s with `max_segment_age` set to `4h`, budget roughly 72 GB.
   The WAL directory lives inside a component-specific directory relative to that path, and you can't configure it separately.

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
   | `loki_write_dropped_entries_total` | Number of log entries dropped after all retries failed, labeled by `reason`.                   |
   | `loki_write_dropped_bytes_total`   | Number of bytes dropped after all retries failed, labeled by `reason`.                         |
   | `loki_write_batch_retries_total`   | Number of batch send retries. A sustained increase means the endpoint is becoming unreachable. |

   Alert on any increase in the two dropped-data counters, because a healthy pipeline never increments them.
   Retries happen during ordinary transient failures, so alert on `loki_write_batch_retries_total` only when the rate stays elevated for several minutes.

1. Search the {{< param "PRODUCT_NAME" >}} logs for the `final error sending batch, no retries left, dropping data` message.
   This message confirms a batch reached the end of its retry schedule and that `loki.write` discarded it.

## Practical limits, even with the WAL

Data buffered during an outage only helps if Loki accepts it once connectivity is restored.
Two limits determine how much outage a WAL-backed pipeline can actually ride out:

- Loki, including Grafana Cloud, rejects entries with `entry too far behind` outside a per-stream, out-of-order acceptance window.
  On Grafana Cloud, automatic time-based stream sharding is enabled by default specifically to keep this window from becoming user-visible in normal operation.
  If you're seeing this error regularly, that's a signal worth raising with support rather than an expected outcome of a long outage.
- Regardless of sharding behavior, Loki applies a hard outer bound on entry age.
  That bound is on the order of days, not hours.

In practice, the WAL buys you resiliency across ingestion stalls and outages shorter than `max_segment_age`.

The acceptance window isn't controlled from the {{< param "PRODUCT_NAME" >}} side.
Refer to [Automatic stream sharding][loki-ooo] in the Loki documentation for your specific Loki or Grafana Cloud tenant configuration.

## Resource consumption during an outage

What `loki.write` consumes during an outage depends on whether you enable the WAL.
The WAL trades in-memory buffering for disk usage, and each has its own ceiling:

- **Disk**: the WAL doesn't grow for the full length of an outage.
  A cleanup routine deletes any segment older than `max_segment_age`, whether or not Loki accepted it.
  The disk footprint settles at roughly `max_segment_age` worth of logs at your ingest rate.
  Age is the only eviction policy available.
  There's no maximum size argument and no ring buffer.
- **Memory**: without the WAL, `queue_config` bounds the in-memory send queue by size in bytes.
  `capacity` defaults to `10MiB` and `min_shards` defaults to `1`, so the default ceiling is `10MiB`.
  Raising `min_shards` multiplies that ceiling, because each shard gets its own queue.
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
  Some of the "successfully buffered" data is then rejected anyway.
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
[loki-ooo]: https://grafana.com/docs/loki/latest/operations/automatic-stream-sharding/
[loki.source.api]: ../../reference/components/loki/loki.source.api/
[loki.source.file]: ../../reference/components/loki/loki.source.file/
[loki.write]: ../../reference/components/loki/loki.write/

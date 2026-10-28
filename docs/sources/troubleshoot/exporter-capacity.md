---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/exporter-capacity/
description: Learn how to estimate the maximum throughput of an OTLP exporter in Grafana Alloy
menuTitle: Estimate exporter capacity
title: Estimate the maximum throughput of an OTLP exporter
weight: 275
---

# Estimate the maximum throughput of an OTLP exporter

An `otelcol.exporter.otlp` or `otelcol.exporter.otlphttp` component can only send a limited amount of data each second.
When the incoming data rate is higher than that limit, the sending queue fills, data arrives late at the destination, and the exporter drops data.

This page shows you how to use the exporter's own metrics to calculate its maximum throughput.
You can then compare the maximum with your real traffic, find out how much headroom you have, and decide which settings to change.

Before you begin, ensure you have the following:

- An {{< param "PRODUCT_NAME" >}} instance that runs an `otelcol.exporter.otlp` or `otelcol.exporter.otlphttp` component.
- A Prometheus-compatible database that collects the {{< param "PRODUCT_NAME" >}} internal metrics.
  For setup details, refer to [Monitor components](../component_metrics/).
- A way to run PromQL queries, for example Grafana Explore.
  If your backend doesn't support PromQL, you can write similar queries in the query language of your backend.

## How an exporter sends data

The calculation on this page depends on how the exporter sends data.
This section gives a short overview of the parts that set the throughput limit.

Each exporter has a `sending_queue` block and a `retry_on_failure` block:

- **Sending queue:** Incoming data goes into an in-memory queue first.
  The `queue_size` argument sets how much the queue holds, and the `sizer` argument sets the unit.
  The default unit is requests.
  When the queue is full, the exporter rejects new data, or blocks the sender if `block_on_overflow` is `true`.
- **Consumers:** Consumers read from the queue, and the `num_consumers` argument sets how many there are.
  Each consumer sends one request at a time and waits for the response before it sends the next request.
- **Retries:** When a request fails with a retryable error, the consumer waits and sends the same request again.
  The consumer doesn't send new data while it waits.

The consumers set the maximum throughput.
If all consumers are busy, the exporter can't send faster, even if the destination has spare capacity.

This page applies when the sending queue is on.
The queue is on by default.
If you set `enabled` to `false` in the `sending_queue` block, the exporter has no consumers, and the calculations on this page don't apply.

### Each signal has its own queue

An exporter component creates one internal exporter for each signal that it receives: metrics, logs, and traces.
Each internal exporter has its own sending queue and its own `num_consumers` consumers.
For example, an exporter with `num_consumers` set to `10` that sends metrics and logs has 20 consumers in total.

Calculate the capacity for each signal separately.
The queue and in-flight metrics on this page have a `data_type` label that identifies the signal.
The HTTP and gRPC client latency histograms don't have this label, so they combine the requests of all signals.
For an accurate result, use a dedicated exporter component for each signal that you want to measure.

For the full list of arguments and their default values, refer to the [`otelcol.exporter.otlp`](../../reference/components/otelcol/otelcol.exporter.otlp/) and [`otelcol.exporter.otlphttp`](../../reference/components/otelcol/otelcol.exporter.otlphttp/) reference pages.

## Calculate the maximum request rate

[Little's Law](https://en.wikipedia.org/wiki/Little%27s_law) connects concurrency, latency, and throughput.
Because each consumer sends one request at a time, the maximum request rate is:

```text
max requests per second = num_consumers / mean request latency
```

For example, with 10 consumers and a mean latency of 1.2 seconds, the exporter can send at most `10 / 1.2 ≈ 8.3` requests per second.

Use the mean latency, not the median.
Slow requests hold a consumer for longer, so a median value gives a result that's too high.

### Measure the mean request latency

The exporter's HTTP or gRPC client records a latency histogram for each request.
To calculate the mean latency, divide the histogram's sum by its count.

For `otelcol.exporter.otlphttp`, use the following query.
The result is in seconds:

```promql
sum(rate(http_client_request_duration_seconds_sum{instance="<INSTANCE>", component_id="<COMPONENT_ID>"}[$__rate_interval]))
/
sum(rate(http_client_request_duration_seconds_count{instance="<INSTANCE>", component_id="<COMPONENT_ID>"}[$__rate_interval]))
```

For `otelcol.exporter.otlp`, use the following query.
The result is in seconds:

```promql
sum(rate(rpc_client_call_duration_seconds_sum{instance="<INSTANCE>", component_id="<COMPONENT_ID>"}[$__rate_interval]))
/
sum(rate(rpc_client_call_duration_seconds_count{instance="<INSTANCE>", component_id="<COMPONENT_ID>"}[$__rate_interval]))
```

{{< admonition type="note" >}}
{{< param "PRODUCT_NAME" >}} v1.20.0 and later report the `rpc_client_call_duration_seconds` histogram.
Earlier versions report the `rpc_client_duration_milliseconds` histogram instead.
This histogram is in milliseconds, so divide the result of the query by `1000` to get seconds.

In v1.20.0 and later, you can also get the legacy histogram.
To do this, set the `OTEL_SEMCONV_STABILITY_OPT_IN` environment variable to `rpc/old` or `rpc/dup`.
{{< /admonition >}}

Replace the following:

- _`<INSTANCE>`_: The `instance` label value of one {{< param "PRODUCT_NAME" >}} instance.
- _`<COMPONENT_ID>`_: The ID of the exporter component, for example `otelcol.exporter.otlphttp.default`.

Filter on one instance, because each instance has its own limit.
If you query several instances together, the sums mix their traffic and the result doesn't match the limit of any one instance.

If you use modules or `foreach` blocks, components in different module instances can have the same ID.
In this case, also filter each query on the `component_path` label.

To check the spread of the latency, you can also calculate percentiles from the same histogram, for example with `histogram_quantile(0.99, ...)`.
Use the mean in the calculation, and use the 99th percentile as a worst-case check.

## Calculate the maximum data throughput

The number of items in each request depends on your batching configuration.
An item is a metric data point, a log record, or a span.
To convert requests per second into items per second, multiply by the number of items in each request:

```text
max items per second = max requests per second × mean items per request
```

### Measure the mean items per request

Don't assume that every request is full.
Batching is off by default.
If you don't enable `batch` in the `sending_queue` block, and you don't use an upstream `otelcol.processor.batch` component, each request carries the data of one incoming batch.
In this case, the request size depends on the sender.

If you enable batching, the exporter sends a batch when the batch reaches `min_size`, or when `flush_timeout` expires.
At low traffic, the exporter sends most batches when the timeout expires, and these batches are smaller than `min_size`.
Under load, batches can be larger than `min_size`, but they never exceed `max_size`.

For `otelcol.exporter.otlphttp`, the following query divides the number of metric data points that the exporter sends by the number of requests:

```promql
sum(rate(otelcol_exporter_sent_metric_points_total{instance="<INSTANCE>", component_id="<COMPONENT_ID>"}[$__rate_interval]))
/
sum(rate(http_client_request_duration_seconds_count{instance="<INSTANCE>", component_id="<COMPONENT_ID>"}[$__rate_interval]))
```

For other signals, replace `otelcol_exporter_sent_metric_points_total` with `otelcol_exporter_sent_log_records_total` or `otelcol_exporter_sent_spans_total`.
For `otelcol.exporter.otlp`, replace `http_client_request_duration_seconds_count` with `rpc_client_call_duration_seconds_count`.

The request count includes failed requests and retries.
When the error rate is high, the result is lower than the real batch size.

If you need the maximum throughput under load, measure during a period of high traffic.
Don't use `max_size` as the request size.
Batches can flush at `min_size`, so the real mean under load can be lower than `max_size`.

### Example calculation

This example uses the following values:

- `num_consumers` is `10`.
- The mean request latency is 1.2 seconds, and the 99th percentile is 2.5 seconds.
- Each request carries 8192 metric data points.

The calculation gives the following results:

| Case                   | Requests per second | Data points per second |
| ---------------------- | ------------------- | ---------------------- |
| Mean latency (1.2 s)   | 10 / 1.2 ≈ 8.3      | 8.3 × 8192 ≈ 68,000    |
| Worst case (2.5 s)     | 10 / 2.5 = 4        | 4 × 8192 ≈ 33,000      |

The maximum throughput of this exporter is about 68,000 data points per second.
If all requests are as slow as the 99th percentile, the maximum falls to about 33,000 data points per second.

The limit applies to each exporter component on each {{< param "PRODUCT_NAME" >}} instance.
If you run several instances, each instance has its own limit.

{{< admonition type="note" >}}
An OpenTelemetry metric data point isn't always one sample in a Prometheus-compatible database.
For example, one histogram data point becomes one sample for each bucket, plus the sum and the count.
The number of samples that the destination ingests can be much higher than the number of data points that the exporter sends.
{{< /admonition >}}

## Compare the maximum with real traffic

After you calculate the maximum throughput, compare it with the rate of data that goes into the exporter.
This section shows the queries that tell you if the exporter is near its limit.

The following query shows the rate of metric data points that the exporter sends:

```promql
sum(rate(otelcol_exporter_sent_metric_points_total{instance="<INSTANCE>", component_id="<COMPONENT_ID>"}[$__rate_interval]))
```

If the sent rate stays near the maximum for long periods, the exporter has no headroom.

The following query shows how many requests are in flight.
This number includes requests that wait in retry backoff:

```promql
sum(otelcol_exporter_in_flight_requests{instance="<INSTANCE>", component_id="<COMPONENT_ID>", data_type="metrics"})
```

If this value is often equal to `num_consumers`, all consumers are busy and the exporter is at its limit.
For other signals, set `data_type` to `logs` or `traces`.

The following query shows how full the sending queue is, as a fraction between `0` and `1`:

```promql
sum(otelcol_exporter_queue_size{instance="<INSTANCE>", component_id="<COMPONENT_ID>", data_type="metrics"})
/
sum(otelcol_exporter_queue_capacity{instance="<INSTANCE>", component_id="<COMPONENT_ID>", data_type="metrics"})
```

A queue that grows and doesn't drain means that data comes in faster than the exporter can send it.
When the queue is full, the exporter rejects new data.
The `otelcol_exporter_enqueue_failed_metric_points_total` metric counts the data points that the exporter rejects.

## Understand how failures reduce throughput

The maximum throughput assumes that each consumer always has a request in flight.
Failures change this assumption in different ways, based on the type of error.

### Retryable errors

When a request fails with a retryable error, the consumer waits before it retries.
The wait starts at `initial_interval`, grows by `multiplier` after each attempt, and stops at `max_interval`.
The consumer retries until the request succeeds or `max_elapsed_time` expires.

While a consumer waits, it counts as busy but it sends no data.
If half of the consumers are in backoff, the maximum throughput falls by half.
If all consumers are in backoff, the exporter sends nothing until a wait ends.

For `otelcol.exporter.otlphttp`, the following HTTP status codes are retryable:

- `429` Too Many Requests
- `502` Bad Gateway
- `503` Service Unavailable
- `504` Gateway Timeout

### Non-retryable errors

`otelcol.exporter.otlphttp` treats other error status codes as permanent.
For example, the exporter doesn't retry a request that fails with `500` Internal Server Error.
The exporter drops the data at once, and the consumer moves to the next request.

Permanent errors don't cause retry backoff, so the consumer doesn't wait before it sends the next request.
The response latency still holds the consumer, and you lose the data in those requests.
The `otelcol_exporter_send_failed_metric_points_total` metric counts the data points in failed requests.

### Estimate the time to drain the queue

After an outage ends, the exporter sends the data in the queue, and new data continues to arrive.
The time to drain a full queue is approximately:

```text
drain time = queued items / (max items per second − incoming items per second)
```

Use items for the backlog and for both rates.
The `otelcol_exporter_queue_size` metric uses the unit of the `sizer` argument.
With the default `requests` sizer, the queue counts incoming batches, not items.
If batching is on, the exporter can merge or split these batches before it sends them, so the batch count doesn't match the number of requests that the exporter sends.
To get a backlog in items, set `sizer` to `items`.

The queue drains only if the maximum is higher than the incoming rate.
If the incoming rate is equal to or higher than the maximum, the queue never drains.

For example, with 1,360,000 queued items, a maximum of 68,000 items per second, and 20,000 incoming items per second, the drain takes about 28 seconds.

During the drain, the destination receives old data.
Some destinations reject data that's too old.

Grafana Mimir and Grafana Cloud Metrics reject old samples with the following errors:

- **`err-mimir-sample-out-of-order`:** The series already has a sample with a more recent timestamp, and out-of-order ingestion is off.
- **`err-mimir-sample-timestamp-too-old`:** The sample is more than 1 hour older than the most recent timestamp for the tenant.
  If out-of-order ingestion is on, the sample is older than the `out_of_order_time_window` setting allows.
- **`err-mimir-too-far-in-past`:** The sample is older than the wall clock time minus the `out_of_order_time_window` and `past_grace_period` settings.

For more information about these errors and how to fix them, refer to the [Mimir runbooks](https://grafana.com/docs/mimir/latest/manage/mimir-runbooks/).

Grafana Cloud Adaptive Metrics also rejects samples that arrive too late to aggregate, with the reason `aggregator-sample-too-old`.
For more information, refer to [Troubleshoot discarded raw samples](https://grafana.com/docs/grafana-cloud/adaptive-telemetry/adaptive-metrics/troubleshoot-discarded-raw-samples/).

## Increase the maximum throughput

If the exporter is near its limit, you can change the configuration to increase the maximum, or to reduce the effect of failures.
Test each change before you use it in production.

- **Increase `num_consumers`:** The maximum request rate increases with the number of consumers until the destination or the network becomes the limit.
  More consumers use more connections and more memory.
- **Increase the batch size:** Larger batches send more items in each request.
  Larger batches can also increase the latency of each request, and the destination can reject requests that are too large.
  Measure the latency again after you change the batch size.
- **Reduce `max_elapsed_time`:** A shorter retry period stops consumers from waiting a long time for one request.
  The exporter drops the data sooner, but other data continues to flow.
- **Scale out:** Each {{< param "PRODUCT_NAME" >}} instance has its own exporters and its own limit.
  Spread the incoming data across more instances to increase the total throughput.

## Next steps

- [Monitor components](../component_metrics/)
- [Import rendered mixin dashboards](../import-mixin-dashboards/)
- [`otelcol.exporter.otlp`](../../reference/components/otelcol/otelcol.exporter.otlp/) reference
- [`otelcol.exporter.otlphttp`](../../reference/components/otelcol/otelcol.exporter.otlphttp/) reference
- [`otelcol.processor.batch`](../../reference/components/otelcol/otelcol.processor.batch/) reference


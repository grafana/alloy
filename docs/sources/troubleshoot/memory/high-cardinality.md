---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/memory/high-cardinality/
description: Learn how high cardinality can cause memory issues in Grafana Alloy
menuTitle: High cardinality
title: High cardinality memory issues
weight: 250
---

# High cardinality memory issues

High cardinality occurs when metrics or logs contain a very large number of unique label or attribute combinations.
Each unique combination creates a separate time series or log stream inside {{< param "PRODUCT_NAME" >}}, which increases memory usage.

In environments with unbounded labels, memory usage may grow steadily as additional series or streams appear.

## Symptoms

High cardinality memory issues often appear as:

- Memory that grows steadily during normal operation
- Large numbers of active series or log streams
- Increased CPU usage during ingestion
- Back pressure that builds even when endpoints are healthy
- High WAL growth in metrics pipelines
- Increasing time series counts in your metrics backend

## Common causes

High cardinality issues often appear after:

- Deploying services or instrumentation
- Adding labels derived from dynamic values
- Enabling high-dimensional metrics

These situations commonly introduce labels or attributes that contain highly variable values.

Examples include:

| Problematic label | Example                  |
| ----------------- | ------------------------ |
| `user_id`         | unique per user          |
| `session_id`      | unique per session       |
| `request_id`      | unique per transaction   |
| `timestamp`       | nearly unique per sample |
| `path` with IDs   | `/orders/1283812`        |

These labels can create thousands or millions of unique combinations.

For example:

```text
http_requests_total{service="checkout",user_id="184928"}
```

Each unique `user_id` creates another series.

## Diagnose high cardinality

If you're diagnosing gradual memory growth, first review the back pressure diagnostic guidance in the [memory troubleshooting overview](../).
Back pressure and high cardinality can produce similar symptoms, but require different solutions.

### Inspect active series

Check the number of active time series in your metrics pipeline.
You can query series counts from your metrics backend or inspect component metrics exposed by {{< param "PRODUCT_NAME" >}}.

A sudden increase in active series usually indicates label or instrumentation changes.

If the series count continues increasing during steady traffic, labels likely contain unbounded values.

### Identify problematic labels

Inspect metric labels or log attributes that change frequently.

Look for labels containing:

- User identifiers
- Request IDs
- Session tokens
- Dynamically generated paths

These labels often generate series continuously.

### Check ingestion behavior

High cardinality can also cause secondary symptoms:

- Increased WAL size
- Back pressure on remote write pipelines
- Higher CPU usage during ingestion

If these symptoms appear alongside growing series counts, cardinality is likely the root cause.

## Reduce cardinality

To reduce cardinality, remove unbounded labels or reshape them into stable dimensions.

Common approaches include:

### Remove unbounded labels

Avoid labels that contain values unique to individual requests or users.

For example, a metric with a `user_id` label creates a separate series for each user:

```text
http_requests_total{user_id="12345"}
```

Remove the unbounded label and keep only stable dimensions:

```text
http_requests_total{service="checkout"}
```

Use the [`prometheus.relabel`][prometheus-relabel] component to drop or rewrite labels before sending metrics to remote endpoints.

### Normalize dynamic paths

If labels contain URL paths with dynamic identifiers, normalize them to a common pattern.

For example, a `path` label might capture individual order IDs:

```text
http_requests_total{path="/orders/18273"}
http_requests_total{path="/orders/29471"}
http_requests_total{path="/orders/99472"}
```

Each unique path creates a separate series.
Normalize the path to reduce cardinality:

```text
http_requests_total{path="/orders/{id}"}
```

### Aggregate metrics earlier

Instead of emitting highly dimensional metrics, aggregate data in the application before exporting it.

### Limit attribute sets in log pipelines

If logs contain highly dynamic attributes, consider removing or dropping them during ingestion.
Use the [`loki.process`][loki-process] component to drop or transform attributes before forwarding logs.

## Monitor cardinality over time

Track metrics that indicate series growth.

Examples include:

- Active time series
- Ingestion rate
- WAL size
- Remote write latency and throughput

If series count grows continuously even when traffic is stable, investigate label usage.

Refer to [Monitor components](../../component_metrics/) for available metrics.

## When to investigate further

If memory continues growing after you reduce cardinality:

1. Confirm ingestion rate hasn't increased.
1. Verify remote endpoints respond normally.
1. Capture heap profiles to identify retained objects.

Refer to [Profile resource consumption](../../profile/) for profiling guidance.

If profiling suggests a memory leak, refer to [Report a potential memory leak](../#report-a-potential-memory-leak).

[prometheus-relabel]: ../../../reference/components/prometheus/prometheus.relabel/
[loki-process]: ../../../reference/components/loki/loki.process/
---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/memory/
description: Learn how to troubleshoot memory issues in Grafana Alloy
title: Troubleshoot memory issues
menuTitle: Memory issues
weight: 100
---

# Troubleshoot memory issues

Most memory issues in {{< param "PRODUCT_NAME" >}} stem from misconfigured resource limits, WAL replay during startup, or back pressure when remote endpoints can't accept data fast enough.

Common symptoms include:

- Container restarts with `OOMKilled`, common in Kubernetes
- Memory spikes immediately after restart
- Memory grows steadily and never drops
- Memory remains high after traffic decreases
- Memory grows steadily and series count increases

## Diagnostic checklist

Before investigating memory issues in detail, identify the pattern that matches your environment.

| Symptom                                          | Likely cause                                                           |
| ------------------------------------------------ | ---------------------------------------------------------------------- |
| Container restarts with `OOMKilled`              | [Kubernetes memory limits][kubernetes] or [WAL replay][prometheus-wal] |
| Memory spikes immediately after restart          | [WAL replay][prometheus-wal]                                           |
| Memory grows steadily while queues increase      | [Back pressure][loki-backpressure] from slow or failing remote systems |
| Memory grows steadily and series count increases | [High cardinality][high-cardinality]                                   |
| Memory remains high after traffic drops          | [Go runtime retaining memory][prometheus-high-memory] for reuse        |

## Understand {{% param "PRODUCT_NAME" %}} memory behavior

{{< param "PRODUCT_NAME" >}} uses [`automemlimit`][automemlimit] to automatically set the [`GOMEMLIMIT`][env-vars] environment variable to 90% of the container memory limit.
This leaves room for memory that the Go garbage collector doesn't manage, such as runtime overhead, stacks, and other allocations.
`GOMEMLIMIT` is a soft limit.
When memory approaches this threshold, the Go runtime runs garbage collection more aggressively to try to stay under it.
Memory usage can still temporarily exceed this limit if the runtime can't free memory quickly enough.

Expect short spikes above `GOMEMLIMIT` during periods of high allocation activity or when {{< param "PRODUCT_NAME" >}} processes bursts of telemetry data.

To override the default, set `GOMEMLIMIT` manually.
Refer to [Environment variables][env-vars] for more information.

## Identify the source

Memory issues often have multiple contributing factors.
Start by identifying which category matches your symptoms:

- **`OOMKilled` or startup crashes**: Refer to [Kubernetes memory issues][kubernetes] for resource configuration and persistent storage guidance.
- **Memory spikes after restart or WAL issues**: Refer to [Prometheus component memory issues][prometheus] for WAL replay and retention configuration.
- **Back pressure from HTTP ingestion sources**: Refer to [Loki component memory issues][loki] for [`loki.source.api`][loki-source-api] and [`loki.source.awsfirehose`][loki-source-awsfirehose] troubleshooting.
- **Memory grows steadily and series count increases**: Refer to [High cardinality memory issues][high-cardinality] for guidance on identifying and reducing high-cardinality labels.
- **Gradual memory growth during normal operation**: This usually indicates queue buildup caused by slow or failing remote systems.
  Review endpoint latency and internal queue metrics.
  If your configuration includes Prometheus or other metrics ingestion pipelines, refer to [Prometheus component memory issues][prometheus] for remote write queues, WAL replay behavior, and cardinality-related memory usage.

## Diagnose back pressure and queue buildup

In many environments, gradual memory growth occurs because {{< param "PRODUCT_NAME" >}} receives telemetry faster than it can forward it to remote endpoints.

When this happens, components buffer telemetry in memory until remote systems catch up.
This behavior can resemble a memory leak, but it usually indicates **back pressure** rather than a defect.

Back pressure most commonly occurs when:

- Remote systems respond slowly or intermittently fail
- Remote endpoints return errors such as `429` or `5xx`
- Retry loops delay successful delivery
- Incoming telemetry volume temporarily exceeds processing capacity

### Verify whether queues are growing

Start by confirming whether telemetry is accumulating inside {{< param "PRODUCT_NAME" >}}.

1. Check logs for delivery errors or retries when sending telemetry to remote endpoints.
1. Inspect component metrics to determine whether internal queues are growing.
1. Compare ingestion rate to forwarding rate to determine whether {{< param "PRODUCT_NAME" >}} is receiving data faster than it can send it.

The following metrics help identify queue buildup and delivery issues:

**Metrics pipelines (`prometheus.remote_write`):**

| Metric                                                           | Type    | Indicates                                  |
| ---------------------------------------------------------------- | ------- | ------------------------------------------ |
| `prometheus_remote_storage_samples_pending`                      | gauge   | Samples queued for delivery                |
| `prometheus_remote_storage_shards_desired`                       | gauge   | Shard scaling demand due to queue backlog  |
| `prometheus_remote_storage_queue_highest_sent_timestamp_seconds` | gauge   | How far behind the queue is                |
| `prometheus_remote_storage_samples_failed_total`                 | counter | Delivery failures                          |
| `prometheus_remote_storage_samples_retries_total`                | counter | Retries due to recoverable errors          |

**Log pipelines (`loki.write`):**

| Metric                             | Type    | Indicates                                |
| ---------------------------------- | ------- | ---------------------------------------- |
| `loki_write_sent_entries_total`    | counter | Successfully delivered log entries       |
| `loki_write_dropped_entries_total` | counter | Entries dropped after all retries failed |
| `loki_write_sent_bytes_total`      | counter | Bytes successfully sent                  |
| `loki_write_dropped_bytes_total`   | counter | Bytes dropped after all retries failed   |
| `loki_write_batch_retries_total`   | counter | Batch retry attempts                     |

If queue depth increases over time while latency or errors are present, memory growth likely reflects buffered telemetry rather than a memory leak.

Refer to the pipeline-specific topics for detailed troubleshooting steps:

- Log ingestion pipelines: [Loki component memory issues][loki]
- Metrics ingestion pipelines: [Prometheus component memory issues][prometheus]

## Capture profiles for diagnosis

Heap and goroutine profiles help identify what consumes memory.
Collect two profiles several minutes apart and compare them to identify allocations that continue to grow over time.
Refer to [Profile resource consumption][profile] for more information.

## Report a potential memory leak

If local troubleshooting and profiling doesn't identify the root cause, collect the following information and [open an issue][alloy-issues]:

- [Support bundle][support-bundle]
- Profiles: heap and goroutine
- {{< param "PRODUCT_NAME" >}} configuration
- Kubernetes Pod specification

Redact any sensitive information before attaching files.

[automemlimit]: https://github.com/KimMachineGun/automemlimit
[env-vars]: ../../reference/cli/environment-variables/#gomemlimit
[loki-source-api]: ../../reference/components/loki/loki.source.api/
[loki-source-awsfirehose]: ../../reference/components/loki/loki.source.awsfirehose/
[profile]: ../profile/
[support-bundle]: ../support_bundle/
[alloy-issues]: https://github.com/grafana/alloy/issues/
[kubernetes]: ./kubernetes/
[prometheus]: ./prometheus/
[prometheus-wal]: ./prometheus/#memory-spikes-after-restart
[prometheus-high-memory]: ./prometheus/#memory-remains-high-after-traffic-drops
[loki]: ./loki/
[loki-backpressure]: ./loki/#back-pressure-from-http-sources
[high-cardinality]: ./high-cardinality/

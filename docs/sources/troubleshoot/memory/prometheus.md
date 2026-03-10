---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/memory/prometheus/
description: Learn how to troubleshoot Prometheus component memory issues in Grafana Alloy
title: Prometheus component memory issues
menuTitle: Prometheus
weight: 200
---

# Prometheus component memory issues

Prometheus components in {{< param "PRODUCT_NAME" >}} use a write-ahead log (WAL) to buffer data before sending it to remote endpoints.
Memory issues with these components often relate to WAL replay, queue buildup, or high cardinality.

## Configure WAL retention

Large WAL segments increase memory usage during replay.
Reduce WAL retention to limit replay memory consumption.
Refer to the [WAL configuration][wal-config] for retention settings.

## Memory spikes after restart

Memory often increases sharply after {{< param "PRODUCT_NAME" >}} starts because it replays the WAL.
Replay loads segments into memory and processes backlogged data.
The spike should be temporary.

If memory remains elevated or triggers restarts, review WAL size and limits.

1. Inspect WAL size.

   Check the size of the {{< param "PRODUCT_NAME" >}} data directory.
   Large WAL segments require additional memory during replay.

1. Observe memory after replay completes.

   Memory should decrease once replay finishes and queues drain.
   If memory stabilizes at a lower level, replay caused the spike.

1. Reduce replay pressure if needed.

   - Increase Pod memory limit.
     Refer to [Kubernetes memory issues][kubernetes] for resource configuration.
   - Reduce WAL retention using [WAL configuration settings][wal-config].
   - Ensure [persistent storage][kubernetes-storage] exists for WAL data.

If spikes persist after adjusting these settings, [capture a profile][profile] during startup to identify the source.
If profiling suggests a memory leak, refer to [Report a potential memory leak][report-leak].

## Memory grows steadily over time

Gradual memory growth usually indicates that data is arriving faster than {{< param "PRODUCT_NAME" >}} can send it, sustained load increase, or a component retaining objects longer than expected.
In some cases, this behavior indicates a leak.

Common causes include:

- Remote write endpoints that respond slowly or reject requests
- High cardinality causing large internal data structures
- Processing components with expensive operations

### Diagnose the cause

1. Check endpoint latency.

   Inspect remote write latency.
   When endpoints respond slowly, components like `prometheus.remote_write` queue data in memory.

1. Confirm whether traffic volume increased.

   Compare ingestion rate to historical baselines.
   Increased load results in higher steady-state memory.
   Refer to [Estimate resource usage][estimate-resource-usage] for more information.

1. Capture heap profiles.

   Collect two profiles several minutes apart and compare them to identify growing allocations.
   Refer to [Profile resource consumption][profile] for more information.

1. Inspect queue metrics.

   Check metrics like `prometheus_remote_storage_shards_desired` and `prometheus_remote_storage_queue_highest_sent_timestamp_seconds` to determine if queues are falling behind.
   Refer to [Monitor components][monitor-components] for more information.

If memory continues to grow with stable traffic and healthy endpoints, refer to [Report a potential memory leak][report-leak].

## Memory remains high after traffic drops

Memory should decrease after ingestion slows and queues drain.
If memory remains high despite lower traffic, the component may still hold objects in memory.

Validate that the workload actually decreased, then inspect retained allocations.

1. Confirm ingestion rate decreased.

   Verify that metrics decreased at the source.

1. Capture a heap profile.

   Identify retained objects and the component responsible.
   Refer to [Profile resource consumption][profile] for more information.

1. Look for:

   - Exporters holding buffered data
   - Queues that haven't drained
   - Unbounded label or cardinality growth

If retained allocations continue to grow with stable traffic, treat the behavior as a potential leak.
Refer to [Report a potential memory leak][report-leak] for next steps.


[kubernetes]: ../kubernetes/
[kubernetes-storage]: ../kubernetes/#configure-persistent-storage
[wal-config]: ../../../reference/components/prometheus/prometheus.remote_write/#wal
[profile]: ../../profile/
[estimate-resource-usage]: ../../../set-up/estimate-resource-usage/
[monitor-components]: ../../component_metrics/
[report-leak]: ../#report-a-potential-memory-leak

---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/memory/prometheus/
description: Learn how to troubleshoot Prometheus component memory issues in Grafana Alloy
title: Prometheus component memory issues
menuTitle: Prometheus
weight: 200
---

# Prometheus component memory issues

Prometheus components in {{< param "PRODUCT_NAME" >}} use a WAL to buffer data before sending it to remote endpoints.
Memory issues with these components often relate to WAL replay, queue buildup in remote write components, or high cardinality.

## Configure WAL retention

Large WAL segments increase memory usage during replay at startup.
Reduce WAL retention to limit replay memory consumption.
Refer to the [WAL configuration][wal-config] for retention settings.

## Memory spikes after restart

Memory often increases sharply after {{< param "PRODUCT_NAME" >}} starts because it replays the WAL.
Replay loads WAL segments into memory and processes backlogged data.
The spike should be temporary.

If memory remains elevated or triggers restarts, review WAL size and limits.

1. Inspect WAL size.

   Check the size of the {{< param "PRODUCT_NAME" >}} data directory.
   Large WAL segments require additional memory during replay.

1. Observe memory after replay completes.

   Memory should decrease once replay finishes and queues drain.
   If memory stabilizes at a lower level, replay caused the spike.

1. Reduce replay pressure if needed.

   - Increase Kubernetes Pod memory limit.
     Refer to [Kubernetes memory issues][kubernetes] for resource configuration.
   - Reduce WAL retention using [WAL configuration settings][wal-config].
   - Ensure [persistent storage][kubernetes-storage] exists for WAL data.

If spikes persist after adjusting these settings, [capture a profile][profile] during startup to identify the source.
If profiling suggests a memory leak, refer to [Report a potential memory leak][report-leak].

## Memory grows steadily over time

If you're diagnosing gradual memory growth, first review [Diagnose back pressure and queue buildup][memory-backpressure] in the memory troubleshooting overview.

Gradual memory growth usually indicates that data is arriving faster than {{< param "PRODUCT_NAME" >}} can send it, sustained load increase, or a component retaining objects longer than expected.
In some cases, this behavior indicates a leak.

Common causes include:

- Remote write endpoints that respond slowly or reject requests
- [High cardinality][high-cardinality] causing large internal data structures.
- Processing components with expensive metric transformations

## Large numbers of scrape targets

Memory and CPU usage can increase significantly if {{< param "PRODUCT_NAME" >}} discovers a large number of scrape targets.

This situation often occurs after changes to service discovery configuration or relabeling rules.
When the number of targets increases unexpectedly, {{< param "PRODUCT_NAME" >}} must schedule additional scrape loops and process more samples, which increases memory usage.

### Symptoms

Large increases in scrape targets often appear as:

- Sudden increases in memory and CPU usage
- A sharp rise in the number of active scrape targets
- Higher scrape load after configuration or deployment changes
- Increased ingestion rates even when application traffic hasn't changed
- Remote write queues growing due to increased sample volume

### Diagnose excessive scrape targets

1. Check the number of discovered scrape targets.

   Compare the current number of targets to historical baselines in your monitoring system.

1. Review recent configuration changes.

   Discovery rules, service monitors, or relabeling changes can unintentionally expose many additional targets.

1. Inspect service discovery configuration.

   Ensure discovery rules filter targets correctly and avoid unintentionally matching large sets of services or endpoints.

### Resolve excessive scrape targets

To reduce the number of scrape targets:

- Restrict service discovery rules to only the intended targets
- Add relabeling rules to drop unnecessary targets
- Reduce scrape intervals for low-priority targets
- Partition scrape workloads across multiple {{< param "PRODUCT_NAME" >}} instances

## Remote write queue buildup

Prometheus pipelines buffer samples in memory before sending them to remote endpoints using [`prometheus.remote_write`][prometheus-remote-write].

If remote endpoints respond slowly or temporarily reject requests, samples accumulate in the remote write queue.
As the queue grows, {{< param "PRODUCT_NAME" >}} may increase the number of remote write shards to keep up with ingestion.

This behavior increases memory usage because queued samples remain buffered in memory until {{< param "PRODUCT_NAME" >}} successfully sends them.

### Symptoms

Remote write queue buildup often appears as:

- Memory growing steadily during normal operation
- Increasing remote write shard counts
- Remote write queues falling behind ingestion
- Growing lag between sent and received samples

### Diagnose queue buildup

1. Check remote write endpoint latency.

   Slow or unreliable endpoints can prevent queues from draining.

1. Inspect remote write queue metrics.

   Metrics such as `prometheus_remote_storage_shards_desired` and `prometheus_remote_storage_queue_highest_sent_timestamp_seconds` can indicate that queues are falling behind.
   Refer to [Monitor components][monitor-components] for more information.

1. Compare ingestion rate to forwarding rate.

   If {{< param "PRODUCT_NAME" >}} receives samples faster than it can send them to remote endpoints, queues grow and memory usage increases.
   Refer to [Estimate resource usage][estimate-resource-usage] for baseline guidance.

1. Capture heap profiles.

   Collect two profiles several minutes apart and compare them to identify growing allocations.
   Refer to [Profile resource consumption][profile] for more information.

If memory continues to grow with stable traffic and healthy endpoints, refer to [Report a potential memory leak][report-leak].

### Resolve queue buildup

Possible solutions include:

- Investigate latency or availability issues with the remote write endpoint
- Temporarily increase memory limits to absorb buffered samples
- Reduce ingestion rate or sample volume if the destination system can't keep up
- Scale {{< param "PRODUCT_NAME" >}} horizontally to distribute ingestion load

## Memory remains high after traffic drops

Memory should decrease after ingestion slows and queues drain.
However, memory may remain elevated for some time because Go retains previously allocated memory for reuse.

This behavior is normal after periods of high ingestion or queue buildup.
If memory usage stabilizes and doesn't continue increasing, the behavior usually reflects retained allocations rather than a leak.

Validate that the workload actually decreased, then inspect retained allocations.

1. Confirm ingestion rate decreased.

   Verify that metrics ingestion rate decreased at the source.

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
[prometheus-remote-write]: ../../../reference/components/prometheus/prometheus.remote_write/
[wal-config]: ../../../reference/components/prometheus/prometheus.remote_write/#wal
[profile]: ../../profile/
[estimate-resource-usage]: ../../../set-up/estimate-resource-usage/
[monitor-components]: ../../component_metrics/
[report-leak]: ../#report-a-potential-memory-leak
[memory-backpressure]: ../#diagnose-back-pressure-and-queue-buildup
[high-cardinality]: ../high-cardinality/

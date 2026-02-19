---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/memory_issues/
description: Learn how to troubleshoot memory issues in Grafana Alloy
title: Troubleshoot memory issues
menuTitle: Troubleshoot memory issues
weight: 200
---

# Troubleshoot memory issues

Memory problems in {{< param "PRODUCT_NAME" >}} usually appear as:

- Kubernetes Pod restarts with `OOMKilled`
- Memory spikes immediately after restart
- Memory grows steadily and never drops
- Memory remains high after traffic decreases

Each pattern points to a different root cause.

## {{% param "PRODUCT_NAME" %}} is `OOMKilled`

Kubernetes terminates a container with `OOMKilled` when it exceeds its memory limit.
This usually happens because the Pod limit is too low, `GOMEMLIMIT` isn't configured, write-ahead log (WAL) replay consumes additional memory at startup, or internal queues grow when endpoints can't accept data fast enough.

Start by validating resource configuration before assuming a leak.

1. Inspect the Pod memory configuration.

   ```bash
   kubectl describe pod <POD_NAME>
   ```

   Verify that memory requests and limits exist.
   If no limit is defined, set one.
   If the limit is close to observed usage, increase it.

   Set the limit high enough to absorb WAL replay and temporary queue growth.
   In most environments, this means at least two to four times steady-state usage.
   Refer to [Estimate resource usage][estimate-resource-usage] for baseline guidance.

1. Configure `GOMEMLIMIT`.

   Set `GOMEMLIMIT` to approximately 90% of your container memory limit.
   For example, with a 2GiB limit, use `GOMEMLIMIT=1800MiB`.

   Without `GOMEMLIMIT`, the Go runtime may expand memory until Kubernetes terminates the container.

   Refer to [Environment variables][env-vars] for more information.

1. Check whether WAL replay triggers the spike.

   If memory jumps immediately after startup, inspect the WAL directory size.
   Large WAL segments increase memory usage during replay.

   If replay causes out of memory (OOM) errors:
   - Increase the memory limit.
   - Reduce WAL retention.
     Refer to the [WAL configuration][wal-config] for retention settings.
   - Ensure you have [persistent storage](#configure-kubernetes-correctly) so the WAL persists across restarts and doesn't grow unbounded.

1. Capture a heap profile if OOM continues.

   Collect heap and goroutine profiles to identify what consumes memory.
   Refer to [Profile resource consumption][profile] for more information.

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
   - Reduce WAL retention using [WAL configuration settings][wal-config].
   - Ensure [persistent storage](#configure-kubernetes-correctly) exists for WAL and positions data.

1. Avoid restart loops.

   Frequent restarts increase replay cost and can create repeated spikes.

## Memory grows steadily over time

Gradual memory growth usually indicates that data is arriving faster than {{< param "PRODUCT_NAME" >}} can send it, sustained load increase, or a component retaining objects longer than expected.
In some cases, this behavior indicates a leak.

Common causes include:

- Remote write endpoints that respond slowly or reject requests
- High cardinality causing large internal data structures
- Processing components with expensive operations

Determine whether traffic, endpoint latency, or retained allocations cause the growth.

1. Check endpoint latency.

   Inspect remote write, log ingestion, or trace exporter latency.
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

## Memory remains high after traffic drops

Memory should decrease after ingestion slows and queues drain.
If memory remains high despite lower traffic, the component may still hold objects in memory.

Validate that the workload actually decreased, then inspect retained allocations.

1. Confirm ingestion rate decreased.

   Verify that logs, metrics, or traces decreased at the source.

1. Capture a heap profile.

   Identify retained objects and the component responsible.
   Refer to [Profile resource consumption][profile] for more information.

1. Look for:

   - Exporters holding buffered data
   - Queues that haven't drained
   - Unbounded label or cardinality growth

1. If retained allocations continue to grow with stable traffic, treat the behavior as a potential leak and collect:

   - [Support bundle][support-bundle]
   - Heap profile
   - {{< param "PRODUCT_NAME" >}} configuration
   - Pod specification

   Provide these artifacts when [opening an issue][alloy-issues].

## Configure Kubernetes correctly

Many memory incidents originate from resource configuration rather than defects.
Incorrect limits, missing persistent storage, or unsuitable workload types increase replay cost and memory pressure.

Define memory requests and limits for every {{< param "PRODUCT_NAME" >}} Pod.
Don't rely on defaults.

Mount persistent storage for:

- WAL data
- Positions file

Example:

```yaml
volumeMounts:
  - name: alloy-data
    mountPath: /var/lib/alloy

volumes:
  - name: alloy-data
    persistentVolumeClaim:
      claimName: alloy-pvc
```

{{< admonition type="note" >}}
Without persistent storage, {{< param "PRODUCT_NAME" >}} loses buffered data on restart and must replay the WAL from scratch each time.
Refer to [Data durability][data-durability] for more information.
{{< /admonition >}}

Use a DaemonSet for node-local log collection.
Use a StatefulSet when stable identity or persistent storage per replica is required.
Refer to [Deploy {{< param "FULL_PRODUCT_NAME" >}}][deploy] for more information.

[estimate-resource-usage]: ../../set-up/estimate-resource-usage/
[env-vars]: ../../reference/cli/environment-variables/#gomemlimit
[wal-config]: ../../reference/components/prometheus/prometheus.remote_write/#wal
[profile]: ../profile/
[support-bundle]: ../support_bundle/
[alloy-issues]: https://github.com/grafana/alloy/issues/
[data-durability]: ../../introduction/requirements/#data-durability
[deploy]: ../../set-up/deploy/
[monitor-components]: ../component_metrics/
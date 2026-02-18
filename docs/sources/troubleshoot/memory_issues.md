---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/memory_issues/
description: Learn how to troubleshoot memory issues in Grafana Alloy
title: Troubleshoot memory issues
menuTitle: Troubleshoot memory issues
weight: 200
---

# Troubleshoot memory issues

Memory problems in {{< param "PRODUCT_NAME" >}} usually appear as:

- Pod restarts with `OOMKilled`
- Memory spikes immediately after restart
- Memory grows steadily and never drops
- Memory remains high after traffic decreases

Each pattern points to a different root cause. Match the behavior you observe and follow the corresponding steps.

## {{% param "PRODUCT_NAME" %}} is `OOMKilled`

Kubernetes terminates a container with `OOMKilled` when it exceeds its memory limit. This usually happens because the Pod limit is too low, `GOMEMLIMIT` isn't configured, write-ahead log (WAL) replay consumes additional memory at startup, or queues grow under `backpressure`.

Start by validating resource configuration before assuming a leak.

1. Inspect the Pod memory configuration.

   ```bash
   kubectl describe pod <pod-name>
   ```

   Verify that memory requests and limits exist. If no limit is defined, set one. If the limit is close to observed usage, increase it.

   Set the limit high enough to absorb WAL replay and temporary queue growth. In most environments, this means at least two to four times steady-state usage.

1. Configure `GOMEMLIMIT`.

   Set `GOMEMLIMIT` slightly below the container memory limit.

   Example for a 2Gi limit:

   ```yaml
   env:
     - name: GOMEMLIMIT
       value: "1800MiB"
   ```

   Redeploy and observe memory behavior. Without `GOMEMLIMIT`, the Go runtime may expand memory until Kubernetes terminates the container.

1. Check whether WAL replay triggers the spike.

   If memory jumps immediately after startup, inspect the WAL directory size. Large WAL segments increase memory usage during replay.

   If replay causes out of memory (OOM) errors:
   - Increase the memory limit.
   - Reduce WAL retention.
   - Avoid repeated restarts.

1. Capture a heap profile if OOM continues.

   Enable the debug server and collect:

   ```bash
   curl http://localhost:12345/debug/pprof/heap > heap.pb.gz
   ```

   Analyze:

   ```bash
   go tool pprof heap.pb.gz
   ```

   Identify the component retaining memory.

## Memory spikes after restart

Memory often increases sharply after {{< param "PRODUCT_NAME" >}} starts because it replays the WAL. Replay loads segments into memory and processes backlogged data. The spike should be temporary.

If memory remains elevated or triggers restarts, review WAL size and limits.

1. Inspect WAL size.

   Check the size of the {{< param "PRODUCT_NAME" >}} data directory. Large WAL segments require additional memory during replay.

1. Observe memory after replay completes.

   Memory should decrease once replay finishes and queues drain. If memory stabilizes at a lower level, replay caused the spike.

1. Reduce replay pressure if needed.

   - Increase Pod memory limit.
   - Reduce WAL retention.
   - Ensure persistent storage exists for WAL and positions data.

1. Avoid restart loops.

   Frequent restarts increase replay cost and can create repeated spikes.

## Memory grows steadily over time

Gradual memory growth usually indicates `backpressure`, sustained load increase, or a component retaining objects longer than expected. In some cases, this behavior indicates a leak.

Determine whether traffic, downstream latency, or retained allocations cause the growth.

1. Check downstream latency.

   Inspect remote write, log ingestion, or trace exporter latency. Slow downstream systems cause queues to grow and memory to increase.

1. Confirm whether traffic volume increased.

   Compare ingestion rate to historical baselines. Increased load results in higher steady-state memory.

1. Capture two heap profiles several minutes apart.

   ```bash
   curl http://localhost:12345/debug/pprof/heap > heap1.pb.gz
   sleep 300
   curl http://localhost:12345/debug/pprof/heap > heap2.pb.gz
   ```

   Compare:

   ```bash
   go tool pprof heap1.pb.gz
   go tool pprof heap2.pb.gz
   ```

   Look for retained allocations that continue to grow.

1. Inspect queue-related allocations.

   If queues grow and don't drain, investigate downstream systems before adjusting {{< param "PRODUCT_NAME" >}}.

## Memory remains high after traffic drops

Memory should decrease after ingestion slows and queues drain. If memory remains high despite lower traffic, objects may remain retained in memory.

Validate that the workload actually decreased, then inspect retained allocations.

1. Confirm ingestion rate decreased.

   Verify that logs, metrics, or traces decreased at the source.

1. Capture a heap profile.

   Identify retained objects and the component responsible.

1. Look for:

   - Exporters holding buffered data
   - Queues that don't shrink
   - Unbounded label or cardinality growth

1. If retained allocations continue to grow with stable traffic, treat the behavior as a potential leak and collect:

   - Support bundle
   - Heap profile
   - {{< param "PRODUCT_NAME" >}} configuration
   - Pod specification

   Provide these artifacts when opening a support case.

## Configure Kubernetes correctly

Many memory incidents originate from resource configuration rather than defects. Incorrect limits, missing persistent storage, or unsuitable workload types increase replay cost and memory pressure.

Define memory requests and limits for every {{< param "PRODUCT_NAME" >}} Pod. Don't rely on defaults.

Mount persistent storage for:

- WAL
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

Use a DaemonSet for node-local log collection. Use a StatefulSet when stable identity or persistent storage per replica is required.

## Tune Go runtime settings

{{< param "PRODUCT_NAME" >}} relies on the Go garbage collector. Without limits, the runtime expands memory aggressively under load.

Set `GOMEMLIMIT` in production environments to align Go memory with container limits.

Adjust `GOGC` only after analyzing heap profiles. Lower values reduce peak memory at the cost of CPU.

Avoid changing multiple runtime parameters at the same time without measuring impact.

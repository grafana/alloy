---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/memory/kubernetes/
description: Learn how to troubleshoot Kubernetes-specific memory issues in Grafana Alloy
title: Kubernetes memory issues
menuTitle: Kubernetes
weight: 100
---

# Kubernetes memory issues

When a container exceeds its memory limit, Kubernetes terminates it with an OOM error, typically showing `OOMKilled` in the Pod status.

Many memory incidents originate from resource configuration rather than defects.
Incorrect limits, missing persistent storage, or unsuitable workload types increase replay cost and memory pressure.

## {{% param "PRODUCT_NAME" %}} exceeds memory limits

Common causes include:

- Pod memory limit is too low
- [`GOMEMLIMIT`][env-vars] isn't configured
- WAL replay consumes additional memory at startup
- Internal queues grow when remote endpoints respond slowly or can't accept data fast enough

### Validate resource configuration

1. Inspect the Pod memory configuration.

   ```bash
   kubectl describe pod <POD_NAME>
   ```

   Verify that you defined both memory requests and limits.
   If you didn't define a limit, set one.
   If the limit is close to observed usage, increase it.

   Set the limit high enough to absorb WAL replay and temporary queue growth.
   In most environments, this means at least two to four times steady-state usage.
   Refer to [Estimate resource usage][estimate-resource-usage] for baseline guidance.

1. Check whether WAL replay triggers the spike.

   If memory jumps immediately after startup, inspect the WAL directory size.
   Large WAL segments increase memory usage during replay because {{< param "PRODUCT_NAME" >}} must load and process buffered telemetry before forwarding it.

   If the container memory limit is too low, the Pod may restart before replay completes.
   This creates a restart loop where each Pod attempts to replay the same WAL.

   If replay causes OOM errors:

   - Increase the memory limit.
   - Reduce WAL retention.
     Refer to [Prometheus component memory issues][prometheus] for WAL configuration details.
   - Ensure you have [persistent storage](#configure-persistent-storage) so the WAL persists across restarts and doesn't grow unbounded.

1. Verify whether WAL replay causes restarts.

   If {{< param "PRODUCT_NAME" >}} repeatedly restarts shortly after startup:

   1. Check Pod logs to confirm whether WAL replay begins before the container terminates.
   1. Check the WAL directory size in the persistent storage volume.
   1. Confirm the container memory limit can absorb temporary replay spikes.

   Increase the Pod memory limit or reduce WAL retention to resolve replay-related restart loops.

1. Capture a heap profile if OOM errors continue.

   Collect heap and goroutine profiles to identify what consumes memory.
   Refer to [Profile resource consumption][profile] for more information.

If profiling suggests a memory leak, refer to [Report a potential memory leak][report-leak].

## Configure persistent storage

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
Without persistent storage, {{< param "PRODUCT_NAME" >}} loses buffered data on restart and must replay the entire WAL from scratch each time.
Refer to [Data durability][data-durability] for more information.
{{< /admonition >}}

## Choose the correct workload type

Use a DaemonSet when collecting logs locally on each node.
Use a StatefulSet when you need stable identity or persistent storage per replica.
Refer to [Deploy {{< param "FULL_PRODUCT_NAME" >}}][deploy] for more information.

## Avoid restart loops

Frequent restarts increase replay cost and can create repeated memory spikes.
If {{< param "PRODUCT_NAME" >}} keeps restarting:

- Increase the memory limit to allow WAL replay to complete.
- Check for probe failures that trigger premature restarts.
- Review logs for errors that cause crashes before WAL replay finishes.

[estimate-resource-usage]: ../../../set-up/estimate-resource-usage/
[env-vars]: ../../../reference/cli/environment-variables/#gomemlimit
[profile]: ../../profile/
[data-durability]: ../../../introduction/requirements/#data-durability
[deploy]: ../../../set-up/deploy/
[prometheus]: ../prometheus/
[report-leak]: ../#report-a-potential-memory-leak

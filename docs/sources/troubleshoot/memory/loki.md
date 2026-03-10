---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/memory/loki/
description: Learn how to troubleshoot Loki component memory issues in Grafana Alloy
title: Loki component memory issues
menuTitle: Loki
weight: 300
---

# Loki component memory issues

Loki components in {{< param "PRODUCT_NAME" >}} handle log ingestion and forwarding.
Memory issues with these components usually occur when back pressure builds up or when the positions file isn't persisted correctly across restarts.

## Back pressure from HTTP sources

Sustained back pressure on HTTP source components like [`loki.source.api`][loki-source-api] and [`loki.source.awsfirehose`][loki-source-awsfirehose] can appear as a memory leak.
This occurs when {{< param "PRODUCT_NAME" >}} receives more HTTP log requests than it can process or forward to log destinations.
Short bursts of traffic can cause temporary memory spikes while buffers absorb incoming log data.
Memory typically stabilizes once ingestion rates return to normal and {{< param "PRODUCT_NAME" >}} forwards the buffered logs.

If you're unsure whether back pressure causes the memory growth, refer to [Diagnose back pressure and queue buildup][memory-backpressure] in the memory troubleshooting overview.

{{< admonition type="note" >}}
Back pressure issues aren't memory leaks.
Memory grows because incoming log data accumulates in memory buffers faster than {{< param "PRODUCT_NAME" >}} can process or forward it.
{{< /admonition >}}

### Diagnose back pressure

1. Check log ingestion latency.

   Inspect the latency of log destinations.
   When endpoints respond slowly, components buffer data in memory.

1. Check for repeated ingestion failures.

   Inspect {{< param "PRODUCT_NAME" >}} logs for HTTP errors or retries when sending logs to Loki endpoints.
   Repeated failures or retry loops can cause queues to grow and increase memory usage.

1. Confirm whether traffic volume increased.

   Compare log ingestion rate to historical baselines.
   Increased load results in higher steady-state memory.
   Refer to [Estimate resource usage][estimate-resource-usage] for more information.

1. Inspect component queues.

   Review internal queue metrics to determine whether log data is accumulating faster than {{< param "PRODUCT_NAME" >}} can forward it.
   Refer to [Monitor components][monitor-components] for more information.

1. Capture heap profiles.

   Collect two profiles several minutes apart and compare them to identify growing allocations.
   Refer to [Profile resource consumption][profile] for more information.

### Resolve back pressure

- Scale {{< param "PRODUCT_NAME" >}} horizontally to handle higher log ingestion rates.
- Increase memory limits to accommodate the buffer.
  Refer to [Kubernetes memory issues][kubernetes] for resource configuration.
- Investigate and resolve destination latency issues.
- Consider rate limiting at the source if traffic exceeds capacity.
- Verify whether component queues continue to grow.

  Inspect queue-related metrics for the Loki components in {{< param "PRODUCT_NAME" >}}.
  If queue depth or buffered log counts continue to increase over time, {{< param "PRODUCT_NAME" >}} is receiving logs faster than it can forward them.

## Positions file persistence

The positions file tracks the read position in log files.
Without persistent storage, {{< param "PRODUCT_NAME" >}} loses position data on restart and may re-read log files from the beginning, which increases ingestion load and memory usage while it processes the logs again.

Ensure [persistent storage][kubernetes-storage] exists for the positions file to prevent duplicate log processing and unnecessary memory usage.

## Positions file and file descriptor limits

In some environments, exhausted file descriptor (FD) limits or storage permission issues cause errors when writing the positions file.
When {{< param "PRODUCT_NAME" >}} can't reliably write the positions file, it may reread log files on restart or during error recovery, which can temporarily increase memory and CPU usage while the system processes duplicate log data.

### Diagnose file descriptor exhaustion

Look for errors related to the positions file in the component logs. Common examples include messages such as:

- `error writing positions file`
- `too many open files`
- I/O or permission errors when accessing the positions file

If these errors appear, check the number of open file descriptors used by the {{< param "PRODUCT_NAME" >}} process:

```bash
ls /proc/<PID>/fd | wc -l
```

You can also inspect the process using:

```bash
lsof -p <PID>
```

Then verify the configured file descriptor limit inside the container or process environment:

```bash
ulimit -n
```

### Resolve FD-related positions file issues

If the process is hitting its file descriptor limit:

1. Increase the file descriptor limit for the container or host environment.
1. Restart the {{< param "PRODUCT_NAME" >}} process so it can reopen files under the updated limits.

If storage or permission problems prevent writing the positions file:

1. Verify that you can write to the positions file location.
1. Confirm that you mounted the directory correctly and that it persists across restarts.
1. Fix permissions or storage configuration and restart {{< param "PRODUCT_NAME" >}}.

If the process is tailing a very large number of log files simultaneously, consider:

- Reducing the number of files handled by a single {{< param "PRODUCT_NAME" >}} instance
- Sharding workloads across multiple {{< param "PRODUCT_NAME" >}} instances

### Monitor after raising limits

Higher file descriptor limits allow {{< param "PRODUCT_NAME" >}} to open more files concurrently, which can also increase memory usage due to additional buffers and internal state.
When adjusting FD limits, monitor both open file counts and memory usage to ensure the system remains stable.

## Memory grows steadily over time

Gradual memory growth can indicate that logs are arriving faster than {{< param "PRODUCT_NAME" >}} can forward them.

Common causes include:

- Log destinations that respond slowly or reject requests
- High log volume causing large internal buffers
- Processing components performing expensive log transformations

### Diagnose the cause

1. Check destination latency.

   Inspect log ingestion latency to log destinations.
   When destinations respond slowly, components buffer data in memory.

1. Confirm whether log volume increased.

   Compare ingestion rate to historical baselines.
   Increased load results in higher steady-state memory.

1. Inspect component queues.

   Review internal queue metrics to determine whether log data is accumulating faster than {{< param "PRODUCT_NAME" >}} can forward it.
   Refer to [Monitor components][monitor-components] for more information.

1. Capture heap profiles.

   Collect two profiles several minutes apart and compare them to identify growing allocations.
   Refer to [Profile resource consumption][profile] for more information.

### Initial log catch-up after restart

When {{< param "PRODUCT_NAME" >}} starts or resumes reading log files, it may need to process a backlog of log entries that accumulated while it wasn't running.

If large log files accumulated while {{< param "PRODUCT_NAME" >}} wasn't running, the system may temporarily ingest a large number of log lines while catching up to the end of each file.

During this catch-up phase:

- CPU usage may increase
- Log ingestion rates may spike
- Memory usage may temporarily increase while {{< param "PRODUCT_NAME" >}} buffers and processes logs

This behavior is normal and usually stabilizes once {{< param "PRODUCT_NAME" >}} reaches the current end of the log files.

### Diagnose catch-up behavior

1. Check whether {{< param "PRODUCT_NAME" >}} recently restarted.

1. Inspect ingestion metrics to determine whether log volume is temporarily higher than normal.

1. Verify that ingestion rates decrease after {{< param "PRODUCT_NAME" >}} processes the backlog.

If ingestion rates return to normal and memory usage stabilizes, the behavior likely reflects temporary catch-up processing rather than a memory leak.

If queue metrics show that buffered log data is draining over time and memory usage gradually decreases, the behavior likely reflects temporary backlog processing rather than a memory leak.
If memory continues to grow with stable traffic and healthy endpoints, refer to [Report a potential memory leak][report-leak].

[estimate-resource-usage]: ../../../set-up/estimate-resource-usage/
[loki-source-api]: ../../../reference/components/loki/loki.source.api/
[loki-source-awsfirehose]: ../../../reference/components/loki/loki.source.awsfirehose/
[monitor-components]: ../../component_metrics/
[profile]: ../../profile/
[kubernetes]: ../kubernetes/
[kubernetes-storage]: ../kubernetes/#configure-persistent-storage
[report-leak]: ../#report-a-potential-memory-leak
[memory-backpressure]: ../#diagnose-back-pressure-and-queue-buildup

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
This occurs when {{< param "PRODUCT_NAME" >}} receives more HTTP log requests than it can process or forward to downstream destinations.

{{< admonition type="note" >}}
Back pressure issues aren't memory leaks.
Memory grows because incoming log data accumulates in memory buffers faster than {{< param "PRODUCT_NAME" >}} can process or forward it.
{{< /admonition >}}

### Diagnose back pressure

1. Check log ingestion latency.

   Inspect the latency of downstream log destinations.
   When endpoints respond slowly, components buffer data in memory.

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
- Investigate and resolve downstream latency issues.
- Consider rate limiting at the source if traffic exceeds capacity.

## Positions file persistence

The positions file tracks the read position in log files.
Without persistent storage, {{< param "PRODUCT_NAME" >}} loses position data on restart and may re-read log files from the beginning, which increases ingestion load and memory usage while it processes the logs again.

Ensure [persistent storage][kubernetes-storage] exists for the positions file to prevent duplicate log processing and unnecessary memory usage.

## Memory grows steadily over time

Gradual memory growth can indicate that logs are arriving faster than {{< param "PRODUCT_NAME" >}} can forward them.

Common causes include:

- Log destinations that respond slowly or reject requests
- High log volume causing large internal buffers
- Processing components performing expensive log transformations

### Diagnose the cause

1. Check destination latency.

   Inspect log ingestion latency to downstream systems.
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

If memory continues to grow with stable traffic and healthy endpoints, refer to [Report a potential memory leak][report-leak].

[estimate-resource-usage]: ../../../set-up/estimate-resource-usage/
[loki-source-api]: ../../../reference/components/loki/loki.source.api/
[loki-source-awsfirehose]: ../../../reference/components/loki/loki.source.awsfirehose/
[monitor-components]: ../../component_metrics/
[profile]: ../../profile/
[kubernetes]: ../kubernetes/
[kubernetes-storage]: ../kubernetes/#configure-persistent-storage
[report-leak]: ../#report-a-potential-memory-leak

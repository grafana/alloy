---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/memory/loki/
description: Learn how to troubleshoot Loki component memory issues in Grafana Alloy
title: Loki component memory issues
menuTitle: Loki
weight: 300
---

# Loki component memory issues

Loki components in {{< param "PRODUCT_NAME" >}} handle log ingestion and forwarding.
Memory issues with these components often relate to back pressure from HTTP sources or positions file persistence.

## Back pressure from HTTP sources

Sustained back pressure on HTTP source components like `loki.source.api` and `loki.source.firehose` can appear as a memory leak.
This occurs when {{< param "PRODUCT_NAME" >}} receives more HTTP requests than it can process.

{{< admonition type="note" >}}
Back pressure issues aren't memory leaks.
Memory grows because incoming data accumulates faster than it can be processed or forwarded.
{{< /admonition >}}

### Diagnose back pressure

1. Check log ingestion latency.

   Inspect the latency of downstream log destinations.
   When endpoints respond slowly, components buffer data in memory.

1. Confirm whether traffic volume increased.

   Compare ingestion rate to historical baselines.
   Increased load results in higher steady-state memory.
   Refer to [Estimate resource usage][estimate-resource-usage] for more information.

1. Capture heap profiles.

   Collect two profiles several minutes apart and compare them to identify growing allocations.
   Refer to [Profile resource consumption][profile] for more information.

### Resolve back pressure

- Scale {{< param "PRODUCT_NAME" >}} horizontally to handle more requests.
- Increase memory limits to accommodate the buffer.
  Refer to [Kubernetes memory issues][kubernetes] for resource configuration.
- Investigate and resolve downstream latency issues.
- Consider rate limiting at the source if traffic exceeds capacity.

## Positions file persistence

The positions file tracks the read position in log files.
Without persistent storage, {{< param "PRODUCT_NAME" >}} loses position data on restart and may re-read log files from the beginning.

Ensure [persistent storage][kubernetes-storage] exists for the positions file to prevent duplicate log processing and unnecessary memory usage.

## Memory grows steadily over time

Gradual memory growth can indicate that logs are arriving faster than {{< param "PRODUCT_NAME" >}} can forward them.

Common causes include:

- Log destinations that respond slowly or reject requests
- High log volume causing large internal buffers
- Processing components with expensive operations

### Diagnose the cause

1. Check destination latency.

   Inspect log ingestion latency to downstream systems.
   When destinations respond slowly, components buffer data in memory.

1. Confirm whether log volume increased.

   Compare ingestion rate to historical baselines.
   Increased load results in higher steady-state memory.

1. Capture heap profiles.

   Collect two profiles several minutes apart and compare them to identify growing allocations.
   Refer to [Profile resource consumption][profile] for more information.

If memory continues to grow with stable traffic and healthy endpoints, refer to [Report a potential memory leak][report-leak].

[estimate-resource-usage]: ../../../set-up/estimate-resource-usage/
[profile]: ../../profile/
[kubernetes]: ../kubernetes/
[kubernetes-storage]: ../kubernetes/#configure-persistent-storage
[report-leak]: ../#report-a-potential-memory-leak

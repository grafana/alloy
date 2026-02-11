---
canonical: https://grafana.com/docs/alloy/latest/telemetry-flow/read-configurations/
description: Learn how to read Grafana Alloy configurations by tracing telemetry paths
menuTitle: Read configurations
title: Read Grafana Alloy configurations as data flow
weight: 400
---

# Read {{% param "FULL_PRODUCT_NAME" %}} configurations as data flow

A configuration defines how telemetry moves through connected components.

To understand a configuration, trace the path telemetry follows.

## Start at receivers

Identify where telemetry enters.
Determine what signal type each receiver handles.
Locate its downstream connections.

## Follow the path

From each receiver, trace the connections to downstream components.

Connections determine execution order, not the order components appear in the file.

If telemetry is missing in a backend, the break usually exists somewhere along this path.

## Identify processors

Processors are the only place where telemetry can be:

- Modified
- Filtered
- Dropped
- Routed

If no processor appears between a receiver and exporter, telemetry passes through unchanged.

## Confirm exporters

Exporters define where telemetry leaves the system.

If a path doesn't end at an exporter, telemetry doesn't leave {{< param "PRODUCT_NAME" >}}.

Following these paths reveals exactly how {{< param "PRODUCT_NAME" >}} behaves at runtime.

## Next steps

- [Get started](../../get-started/) - Learn the fundamentals of {{< param "PRODUCT_NAME" >}} configuration syntax and components.
- [Troubleshoot](../../troubleshoot/) - Find solutions to common issues and debugging techniques.

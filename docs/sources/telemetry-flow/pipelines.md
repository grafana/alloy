---
canonical: https://grafana.com/docs/alloy/latest/telemetry-flow/pipelines/
description: Learn how telemetry flows through Grafana Alloy from ingestion to export
menuTitle: Pipelines
title: Telemetry pipelines in Grafana Alloy
weight: 200
---

# Telemetry pipelines in {{% param "FULL_PRODUCT_NAME" %}}

A telemetry pipeline is a connected path that moves telemetry from ingestion to export.

Each pipeline begins with a receiver, may include processors, and ends with one or more exporters.

## Ingestion

Receivers accept telemetry from external systems such as:

- Applications
- Infrastructure endpoints
- Log sources
- Other collectors

Receivers decode and normalize incoming data so it can move through {{< param "PRODUCT_NAME" >}}.

If a receiver has no downstream connection, its telemetry stops there.

## Processing

Processors operate on telemetry after ingestion and before export.

Processors can:

- Modify fields or labels
- Filter or drop telemetry
- Route telemetry to different downstream components
- Sample traces

Processing only happens when you include a processor in the telemetry path.

## Export

Exporters send telemetry from {{< param "PRODUCT_NAME" >}} to external systems.

A pipeline can include:

- A single exporter
- Multiple exporters
- No exporters

If telemetry doesn't reach an exporter, it doesn't leave {{< param "PRODUCT_NAME" >}}.

## Multiple pipelines

A configuration may contain multiple pipelines for different signals.

For example:

- Metrics follow one path.
- Logs follow another.
- Traces follow a third.

These pipelines can remain independent or share components.

## Next steps

- [Where telemetry is modified](../modify-telemetry/) - Learn where and how telemetry changes.
- [Read configurations as data flow](../read-configurations/) - Interpret configurations by tracing telemetry paths.

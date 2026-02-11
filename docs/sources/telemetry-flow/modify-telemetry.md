---
canonical: https://grafana.com/docs/alloy/latest/telemetry-flow/modify-telemetry/
description: Learn where and how Grafana Alloy modifies telemetry
menuTitle: Modify telemetry
title: Where Grafana Alloy modifies telemetry
weight: 300
---

# Where {{% param "FULL_PRODUCT_NAME" %}} modifies telemetry

Processors are the only place where telemetry changes.

Receivers ingest and normalize data.
Exporters deliver data.
Any semantic modification occurs between those two stages.

## Modification happens inside processors

A typical path looks like this:

Receiver → Processor → Exporter

Processors can:

- Rewrite labels
- Filter or drop telemetry
- Enrich data
- Sample traces
- Route telemetry to different destinations

If a processor isn't connected in a path, it has no effect on that telemetry.

## No automatic transformation

{{< param "PRODUCT_NAME" >}} doesn't automatically modify telemetry.

It doesn't:

- Redact sensitive data by default
- Reduce metric cardinality automatically
- Drop telemetry without configuration
- Sample traces without an explicit processor

You must define all transformation logic in the configuration.

Understanding where modification occurs makes it easier to design pipelines that control what gets sent to downstream systems.

## Next steps

- [Read configurations as data flow](../read-configurations/) - Interpret configurations by tracing telemetry paths.

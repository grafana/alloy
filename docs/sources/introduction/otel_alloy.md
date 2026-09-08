---
canonical: https://grafana.com/docs/alloy/latest/introduction/otel_alloy/
aliases:
  - ../opentelemetry/ # /docs/alloy/latest/opentelemetry/
description: Learn about the OpenTelemetry Engine, a bundled OpenTelemetry Collector distribution embedded within Grafana Alloy
menuTitle: OpenTelemetry in Alloy
title: OpenTelemetry in Alloy
weight: 230
---

# OpenTelemetry in {{% param "PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} combines the Prometheus-native, production-grade collection features of {{< param "PRODUCT_NAME" >}} with the broad ecosystem and standards of OpenTelemetry.
The {{< param "FULL_OTEL_ENGINE" >}} is an OpenTelemetry Collector distribution embedded within {{< param "PRODUCT_NAME" >}}.
It lets you run {{< param "PRODUCT_NAME" >}} with the OpenTelemetry Collector while retaining access to {{< param "PRODUCT_NAME" >}} features and integrations through the {{< param "PRODUCT_NAME" >}} Engine extension.

{{< docs/shared lookup="stability/experimental_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Why the {{% param "OTEL_ENGINE" %}} exists

Standard OpenTelemetry Collector pipelines use YAML configuration, but {{< param "PRODUCT_NAME" >}} components require translating that configuration into {{< param "PRODUCT_NAME" >}} syntax.
The {{< param "OTEL_ENGINE" >}} runs the Collector runtime directly, so you can use Collector YAML configurations without translation.

The {{< param "OTEL_ENGINE" >}} addresses this by running the upstream Collector runtime directly from the {{< param "PRODUCT_NAME" >}} executable.
You can bring existing Collector configurations to {{< param "PRODUCT_NAME" >}}, use familiar Collector tooling, and choose from the [components bundled with the `otel` command](../../reference/cli/otel/#included-components).

The {{< param "OTEL_ENGINE" >}} also gives Grafana a standards-native foundation for extending {{< param "PRODUCT_NAME" >}}.
Grafana is committed to providing a first-class OpenTelemetry collection experience as this experimental engine matures.

## How the engines fit together

Both engines are built into the same {{< param "PRODUCT_NAME" >}} binary: `alloy run` starts the {{< param "DEFAULT_ENGINE" >}}, and `alloy otel` starts the {{< param "OTEL_ENGINE" >}}.
The optional {{< param "PRODUCT_NAME" >}} Engine extension lets you run a {{< param "DEFAULT_ENGINE" >}} pipeline inside the {{< param "OTEL_ENGINE" >}}, in the same process.

The following diagram shows how the engines and the extension fit inside the {{< param "PRODUCT_NAME" >}} executable:

{{< mermaid >}}
---
config:
  flowchart:
    subGraphTitleMargin:
      top: 10
      bottom: 10
    rankSpacing: 10
---

flowchart TD
    subgraph EXEC["Grafana Alloy executable"]
        direction TB
        run(["'alloy run' command"])
        otel(["'alloy otel' command"])
        DE["Default Engine<br/><small>Traditional Alloy pipeline</small>"]
        run --> DE

        subgraph OE["OTel Engine"]
            direction TB
            subgraph EXT["alloyengine extension <small>(optional)</small>"]
                DEP["Default Engine<br/><small>Traditional Alloy pipeline</small>"]
            end
        end
        otel --> OE
    end

    style EXEC fill:#fdefe5,stroke:#000000,color:#000000,rx:10,ry:10
    style OE fill:#cce5ff,stroke:#000000,color:#000000,rx:10,ry:10
    style EXT fill:#cce5ff,stroke:#000000,color:#000000,stroke-dasharray: 4 3,rx:10,ry:10
    style DE fill:#ff8833,stroke:#000000,color:#000000,rx:10,ry:10
    style DEP fill:#ff8833,stroke:#000000,color:#000000,rx:10,ry:10
    style run fill:#ffffff,stroke:#000000,color:#000000,rx:10,ry:10
    style otel fill:#ffffff,stroke:#000000,color:#000000,rx:10,ry:10
{{< /mermaid >}}

## Choose an engine

{{< param "PRODUCT_NAME" >}} supports two runtime engines and an extension.
Choose the engine that best matches your existing configuration and collection workload.

- **{{< param "DEFAULT_ENGINE" >}}**: The standard way to run {{< param "PRODUCT_NAME" >}}.
  It uses [{{< param "PRODUCT_NAME" >}} configuration syntax](../../get-started/syntax/) and {{< param "PRODUCT_NAME" >}} components.
  It remains the stable, most polished experience for getting the most from Grafana Cloud, with [backward compatibility](../backward-compatibility/) guarantees, a built-in UI, live debugging, support bundles, clustering, and broad Grafana integrations.
- **{{< param "OTEL_ENGINE" >}}**: The upstream OpenTelemetry Collector runtime embedded in {{< param "PRODUCT_NAME" >}}.
  It uses [Collector YAML configuration](https://opentelemetry.io/docs/collector/configuration/) and standard Collector command-line arguments.
  It provides a direct path for OpenTelemetry-native pipelines and existing Collector configurations.
- **{{< param "PRODUCT_NAME" >}} Engine extension**: An OpenTelemetry Collector extension that starts a {{< param "DEFAULT_ENGINE" >}} pipeline alongside the {{< param "OTEL_ENGINE" >}}.
  The two pipelines run in the same process, but they don't interact directly.

### Use the {{% param "OTEL_ENGINE" %}} for OpenTelemetry-native pipelines

The {{< param "OTEL_ENGINE" >}} is a good fit when you:

- Already run an OpenTelemetry Collector and want to reuse its YAML configuration and operational model.
- Prefer to use standard Collector configuration and command-line arguments.
- Collect push-based, OpenTelemetry-native signals through OTLP.
- Use Grafana Application Observability and don't need {{< param "DEFAULT_ENGINE" >}} components in the same pipeline.

Grafana Application Observability uses OpenTelemetry-native telemetry.
To build a compatible Collector pipeline, refer to [Set up OpenTelemetry Collector for Application Observability](https://grafana.com/docs/opentelemetry/collector/opentelemetry-collector/).

### Use the {{% param "DEFAULT_ENGINE" %}} for the full {{% param "PRODUCT_NAME" %}} experience

The {{< param "DEFAULT_ENGINE" >}} is the recommended choice when you:

- Want the stable and most complete {{< param "PRODUCT_NAME" >}} experience.
- Collect infrastructure telemetry with Prometheus exporters and pull-based scrapes.
- Need {{< param "PRODUCT_NAME" >}} features such as clustering, the built-in UI, live debugging, support bundles, or configuration reloads.
- Use Grafana integrations for Kubernetes monitoring, Database Observability, eBPF, logs, or profiles.

The {{< param "DEFAULT_ENGINE" >}} is optimized for Prometheus pipelines and label semantics.
It also provides Grafana-specific collection features that don't yet have OpenTelemetry-native equivalents.

### Run both engines when you need both component sets

Use the `alloyengine` extension when one process needs both standard Collector components and {{< param "DEFAULT_ENGINE" >}} components.
This approach can simplify small deployments.

For large workloads, run the engines in separate processes so you can scale and troubleshoot them independently.
Push-based OTLP gateways and pull-based Prometheus scrapers have different load and scaling characteristics.

## Manage the {{% param "OTEL_ENGINE" %}} with Fleet Management

The {{< param "OTEL_ENGINE" >}} works with the OpenTelemetry Collector support in Grafana Fleet Management.
The {{< param "PRODUCT_NAME" >}} container provides an `otelcol`-compatible entry point, and the engine includes the components needed to run with the Open Agent Management Protocol (OpAMP) Supervisor.
To monitor and remotely configure an {{< param "OTEL_ENGINE" >}} deployment, follow the [Fleet Management setup for the OpenTelemetry Collector](https://grafana.com/docs/grafana-cloud/send-data/fleet-management/get-started/opentelemetry-collector/).

## How the engines evolve

Grafana continues to improve the {{< param "OTEL_ENGINE" >}} and its integration with Grafana products.
The goal is a first-class collection experience for users who choose standard OpenTelemetry Collector workflows.

{{< param "DEFAULT_ENGINE" >}} continues to be in active development.
It remains the default, stable engine, and Grafana continues to add features to it.
The two engines can evolve toward the same outcomes without requiring every feature to have an identical implementation.

## Next steps

- [Set up the {{< param "OTEL_ENGINE" >}}](../../set-up/otel_engine/).
- [Explore the `otel` command and its included components](../../reference/cli/otel/).

---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/
aliases:
  - ../opentelemetry/get-started/ # /docs/alloy/latest/opentelemetry/get-started/
description: Learn how to run the Alloy OpenTelemetry Engine with the CLI, the OpenTelemetry Collector Helm chart, a service installation, or a custom build
menuTitle: OpenTelemetry Engine
review_date: 2026-10-06
title: The Alloy OpenTelemetry Engine
weight: 390
---

# The {{% param "FULL_OTEL_ENGINE" %}}

The {{< param "OTEL_ENGINE" >}} is an OpenTelemetry Collector distribution embedded in {{< param "PRODUCT_NAME" >}}.
It runs standard OpenTelemetry Collector YAML configuration without translating it to {{< param "PRODUCT_NAME" >}} syntax.

You can run it with the CLI, the OpenTelemetry Collector Helm chart, or a service installation.
To run a {{< param "DEFAULT_ENGINE" >}} pipeline inside the {{< param "OTEL_ENGINE" >}}, add the {{< param "PRODUCT_NAME" >}} Engine extension to any of these.
You can also build your own binary with the OpenTelemetry Collector Builder.

To manage the {{< param "OTEL_ENGINE" >}} configuration remotely, use the [`otel-supervisor`][OTelSupervisorCommand] command.
It runs the {{< param "OTEL_ENGINE" >}} under an embedded OpAMP supervisor that receives the configuration from [Grafana Fleet Management][FleetManagement].

To learn when to use each engine, refer to [OpenTelemetry in {{< param "PRODUCT_NAME" >}}][OTelAlloy].

{{< docs/shared lookup="stability/public_preview_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< section >}}

## Next steps

- [`otel` command reference][OTelCommand]
- [`otel-supervisor` command reference][OTelSupervisorCommand]

[FleetManagement]: https://grafana.com/docs/grafana-cloud/send-data/fleet-management/
[OTelAlloy]: ../../introduction/otel_alloy/
[OTelCommand]: ../../reference/cli/otel/
[OTelSupervisorCommand]: ../../reference/cli/otel-supervisor/

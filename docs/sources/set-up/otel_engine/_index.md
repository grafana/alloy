---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/
aliases:
  - ../opentelemetry/get-started/ # /docs/alloy/latest/opentelemetry/get-started/
description: Learn how to run the Alloy OpenTelemetry Engine with the CLI, the OpenTelemetry Collector Helm chart, a service installation, or a custom build
menuTitle: OpenTelemetry Engine
review_date: 2026-09-23
title: The Alloy OpenTelemetry Engine
weight: 390
---

# The {{% param "FULL_OTEL_ENGINE" %}}

The {{< param "OTEL_ENGINE" >}} is an OpenTelemetry Collector distribution embedded in {{< param "PRODUCT_NAME" >}}.
It runs standard OpenTelemetry Collector YAML configuration without translating it to {{< param "PRODUCT_NAME" >}} syntax.

You can run it with the CLI, the OpenTelemetry Collector Helm chart, or a service installation.
You can also build your own binary with the OpenTelemetry Collector Builder.

To run a {{< param "DEFAULT_ENGINE" >}} pipeline inside the {{< param "OTEL_ENGINE" >}}, add the {{< param "PRODUCT_NAME" >}} Engine extension to any of these.

To learn when to use each engine, refer to [OpenTelemetry in {{< param "PRODUCT_NAME" >}}][OTelAlloy].

{{< docs/shared lookup="stability/experimental_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< section >}}

## Next steps

- [`otel` command reference][OTelCommand]

[OTelAlloy]: ../../introduction/otel_alloy/
[OTelCommand]: ../../reference/cli/otel/

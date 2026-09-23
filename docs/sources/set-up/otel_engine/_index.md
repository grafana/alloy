---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/
aliases:
  - ../opentelemetry/get-started/ # /docs/alloy/latest/opentelemetry/get-started/
description: Learn how to run the OpenTelemetry Engine with the CLI, the Alloy Engine extension, the OpenTelemetry Collector Helm chart, or a service installation
menuTitle: OpenTelemetry Engine
title: The OpenTelemetry Engine
weight: 390
---

# The {{% param "FULL_OTEL_ENGINE" %}}

You can run the {{< param "OTEL_ENGINE" >}} with the CLI, the {{< param "PRODUCT_NAME" >}} Engine extension, the OpenTelemetry Collector Helm chart, or a service installation.
{{< param "PRODUCT_NAME" >}} includes the tools you need, so there are no other prerequisites.
To learn when to use each engine, refer to [OpenTelemetry in {{< param "PRODUCT_NAME" >}}][OTelAlloy].

{{< docs/shared lookup="stability/experimental_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< section >}}

## Next steps

- [OpenTelemetry in {{< param "PRODUCT_NAME" >}}][OTelAlloy]
- [`otel` command reference][OTelCommand]

[OTelAlloy]: ../../introduction/otel_alloy/
[OTelCommand]: ../../reference/cli/otel/

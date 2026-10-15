---
canonical: https://grafana.com/docs/alloy/latest/scenarios/opentelemetry-engine/
description: Runnable scenarios that use the Alloy OpenTelemetry Engine to run OpenTelemetry Collector configurations
review_date: 2026-10-15
menuTitle: OpenTelemetry Engine
title: Alloy OpenTelemetry Engine scenarios
weight: 800
---

# {{% param "FULL_OTEL_ENGINE" %}} scenarios

{{< docs/shared lookup="stability/experimental_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

These scenarios run upstream OpenTelemetry Collector YAML directly with the {{< param "OTEL_ENGINE" >}}, instead of converting it to {{< param "PRODUCT_NAME" >}} configuration syntax.
Each one pairs an OpenTelemetry YAML configuration with a minimal {{< param "PRODUCT_NAME" >}} configuration that enables the user interface.

Use them when you're moving a Collector configuration to {{< param "PRODUCT_NAME" >}} and want to keep the YAML.
They also provide worked examples of processors such as `transform`, `filter`, and `redaction`.
Refer to [The OpenTelemetry Engine][otel-engine] for how to run one, and to [`alloy otel`][otel-cli] for the command-line interface.

{{< docs/shared lookup="scenarios/opentelemetry-engine.md" source="alloy" version="<ALLOY_VERSION>" >}}

[otel-engine]: ../../set-up/otel_engine/
[otel-cli]: ../../reference/cli/otel/

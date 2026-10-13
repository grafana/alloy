---
canonical: https://grafana.com/docs/alloy/latest/scenarios/traces/
description: Runnable scenarios that receive, sample, and forward traces with Grafana Alloy
menuTitle: Traces
title: Traces scenarios
weight: 300
---

# Traces scenarios

These scenarios receive spans over OpenTelemetry, Jaeger, or Zipkin protocols and forward them to Tempo.

The simplest ones run [`otelcol.receiver.otlp`][otelcol.receiver.otlp] into [`otelcol.processor.batch`][otelcol.processor.batch] and out through [`otelcol.exporter.otlp`][otelcol.exporter.otlp].
Others show what you can derive from a trace stream once it flows through {{< param "PRODUCT_NAME" >}}: request rate and error rate metrics from spans, service graphs, and tail sampling that keeps the slow and failed traces while dropping the rest.

{{< docs/shared lookup="scenarios/traces.md" source="alloy" version="<ALLOY_VERSION>" >}}

[otelcol.receiver.otlp]: ../../reference/components/otelcol/otelcol.receiver.otlp/
[otelcol.processor.batch]: ../../reference/components/otelcol/otelcol.processor.batch/
[otelcol.exporter.otlp]: ../../reference/components/otelcol/otelcol.exporter.otlp/

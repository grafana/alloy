---
canonical: https://grafana.com/docs/alloy/latest/scenarios/traces/
description: Runnable scenarios that receive, sample, and forward traces with Grafana Alloy
review_date: 2026-10-15
menuTitle: Traces
title: Traces scenarios
weight: 300
---

# Traces scenarios

These scenarios receive spans over OpenTelemetry, Jaeger, or Zipkin protocols and forward them to Tempo.

The simplest ones run [`otelcol.receiver.otlp`][otelcol.receiver.otlp] into [`otelcol.processor.batch`][otelcol.processor.batch] and out through [`otelcol.exporter.otlp`][otelcol.exporter.otlp].
Others show what a trace stream gives you once it flows through {{< param "PRODUCT_NAME" >}}.
You can derive request and error rate metrics from spans, build service graphs, or tail sample to keep only slow and failed traces.

{{< docs/shared lookup="scenarios/traces.md" source="alloy" version="<ALLOY_VERSION>" >}}

[otelcol.receiver.otlp]: ../../reference/components/otelcol/otelcol.receiver.otlp/
[otelcol.processor.batch]: ../../reference/components/otelcol/otelcol.processor.batch/
[otelcol.exporter.otlp]: ../../reference/components/otelcol/otelcol.exporter.otlp/

---
canonical: https://grafana.com/docs/alloy/latest/set-up/deploy/
aliases:
  - ../get-started/deploy/ # /docs/alloy/latest/get-started/deploy/
description: Learn about possible deployment topologies for Grafana Alloy
menuTitle: Deploy
title: Deploy Grafana Alloy
weight: 900
---

{{< docs/shared source="alloy" lookup="/deploy-alloy.md" version="<ALLOY_VERSION>" >}}

## Processing different types of telemetry in different {{< param "PRODUCT_NAME" >}} instances

If the load on {{< param "PRODUCT_NAME" >}} is small, you can process all necessary telemetry signals in the same {{< param "PRODUCT_NAME" >}} process.
For example, a single {{< param "PRODUCT_NAME" >}} deployment can process all of the incoming metrics, logs, traces, and profiles.

However, if the load on {{< param "PRODUCT_NAME" >}} is big, it may be beneficial to process different telemetry signals in different deployments of {{< param "PRODUCT_NAME" >}}.

This provides better stability due to the isolation between processes.
For example, an overloaded {{< param "PRODUCT_NAME" >}} instance processing traces won't impact an {{< param "PRODUCT_NAME" >}} instance processing metrics.
Different types of signal collection require different methods for scaling:

* "Pull" components such as `prometheus.scrape` and `pyroscope.scrape` are scaled using hashmod sharing or clustering.
* "Push" components such as `otelcol.receiver.otlp` are scaled by placing a load balancer in front of the components.

### Traces

Scaling {{< param "PRODUCT_NAME" >}} instances for tracing is very similar to [scaling OpenTelemetry Collector][scaling-collector] instances.
This similarity is because most {{< param "PRODUCT_NAME" >}} components used for tracing are based on components from the OTel Collector.

[scaling-collector]: https://opentelemetry.io/docs/collector/scaling/

#### When to scale

To decide whether scaling is necessary, check metrics such as:
* `receiver_refused_spans_ratio_total` from receivers such as `otelcol.receiver.otlp`.
* `processor_refused_spans_ratio_total` from processors such as `otelcol.processor.batch`.
* `exporter_send_failed_spans_ratio_total` from exporters such as `otelcol.exporter.otlp` and `otelcol.exporter.loadbalancing`.

#### Stateful and stateless components

In the context of tracing, a "stateful component" is a component that needs to aggregate certain spans to work correctly.
A "stateless {{< param "PRODUCT_NAME" >}}" is an {{< param "PRODUCT_NAME" >}} instance which doesn't contain stateful components.

Scaling stateful {{< param "PRODUCT_NAME" >}} instances is more difficult, because spans must be forwarded to a specific {{< param "PRODUCT_NAME" >}} instance according to a span property such as trace ID or a `service.name` attribute.
You can forward spans with `otelcol.exporter.loadbalancing`.

Examples of stateful components:

* `otelcol.processor.tail_sampling`
* `otelcol.connector.spanmetrics`
* `otelcol.connector.servicegraph`

<!-- TODO: link to the otelcol.exporter.loadbalancing docs for more info -->

A "stateless component" doesn't need to aggregate specific spans to work correctly.
It can work correctly even if it only has some of the spans of a trace.

A stateless {{< param "PRODUCT_NAME" >}} instance can be scaled without using `otelcol.exporter.loadbalancing`.
For example, you could use an off-the-shelf load balancer to do a round-robin load balancing.

Examples of stateless components:
* `otelcol.processor.probabilistic_sampler`
* `otelcol.processor.transform`
* `otelcol.processor.attributes`
* `otelcol.processor.span`

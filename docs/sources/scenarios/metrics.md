---
canonical: https://grafana.com/docs/alloy/latest/scenarios/metrics/
description: Runnable scenarios that collect and forward metrics with Grafana Alloy
review_date: 2026-10-15
menuTitle: Metrics
title: Metrics scenarios
weight: 100
---

# Metrics scenarios

These scenarios collect metrics and forward them to a Prometheus-compatible backend.

They cover both directions {{< param "PRODUCT_NAME" >}} works in.
[`prometheus.scrape`][prometheus.scrape] pulls a `/metrics` endpoint on an interval, and an application can push OpenTelemetry or StatsD data instead.
Most scenarios end at [`prometheus.remote_write`][prometheus.remote_write], and two show [`prometheus.relabel`][prometheus.relabel] rewriting labels or dropping series before they reach storage.

{{< docs/shared lookup="scenarios/metrics.md" source="alloy" version="<ALLOY_VERSION>" >}}

[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/
[prometheus.remote_write]: ../../reference/components/prometheus/prometheus.remote_write/
[prometheus.relabel]: ../../reference/components/prometheus/prometheus.relabel/

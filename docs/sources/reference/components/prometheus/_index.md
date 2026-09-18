---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/
description: Learn about the prometheus components in Grafana Alloy
review_date: 2026-09-14
labels:
  products:
    - oss
title: prometheus
weight: 100
---

# `prometheus`

The `prometheus` components collect, process, and forward Prometheus-compatible metrics.
They support multiple collection methods, including HTTP scraping, remote-write ingestion, and built-in exporters, and let you relabel, enrich, and route metrics before writing them to storage.

Use a `prometheus` component when you want to collect metrics from your applications and infrastructure, and send them to Prometheus, Grafana Mimir, or another compatible endpoint for analysis.

{{< section >}}

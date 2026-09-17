---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/
description: Learn about the pyroscope components in Grafana Alloy
review_date: 2026-09-11
labels:
  products:
    - oss
title: pyroscope
weight: 100
---

# `pyroscope`

The `pyroscope` components collect, process, and forward continuous profiling data to a Pyroscope-compatible backend. They support multiple collection methods, including eBPF-based profiling, Java process profiling, and HTTP scraping, and let you enrich, relabel, and route profiles before writing them to storage.

Use a `pyroscope` component when you want to collect performance profiles from your applications and infrastructure, and send them to Grafana Pyroscope or another compatible endpoint for analysis.

{{< section >}}

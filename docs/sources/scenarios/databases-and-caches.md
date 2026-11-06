---
canonical: https://grafana.com/docs/alloy/latest/scenarios/databases-and-caches/
description: Runnable scenarios that monitor databases and in-memory caches with Grafana Alloy
review_date: 2026-10-15
menuTitle: Databases and caches
title: Database and cache scenarios
weight: 500
---

# Database and cache scenarios

These scenarios monitor a database or cache server and the workload running against it.

Each one starts a real server, and some add a load generator so there's traffic to measure from the first minute.
They're built on the exporter component for each engine, such as [`prometheus.exporter.mysql`][prometheus.exporter.mysql], [`prometheus.exporter.postgres`][prometheus.exporter.postgres], or [`prometheus.exporter.redis`][prometheus.exporter.redis], collected by [`prometheus.scrape`][prometheus.scrape].

{{< docs/shared lookup="scenarios/databases-and-caches.md" source="alloy" version="<ALLOY_VERSION>" >}}

[prometheus.exporter.mysql]: ../../reference/components/prometheus/prometheus.exporter.mysql/
[prometheus.exporter.postgres]: ../../reference/components/prometheus/prometheus.exporter.postgres/
[prometheus.exporter.redis]: ../../reference/components/prometheus/prometheus.exporter.redis/
[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/

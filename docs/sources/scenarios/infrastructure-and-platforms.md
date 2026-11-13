---
canonical: https://grafana.com/docs/alloy/latest/scenarios/infrastructure-and-platforms/
description: Runnable scenarios that monitor hosts, containers, network devices, cloud providers, and Kubernetes with Grafana Alloy
review_date: 2026-10-15
menuTitle: Infrastructure and platforms
title: Infrastructure and platform scenarios
weight: 600
---

# Infrastructure and platform scenarios

These scenarios monitor the layer your applications run on: Linux and Windows hosts, Docker containers, network devices, cloud provider APIs, and Kubernetes clusters.

Most pair a platform exporter such as [`prometheus.exporter.unix`][prometheus.exporter.unix] or [`prometheus.exporter.windows`][prometheus.exporter.windows] with log collection from the same machine.
A single scenario then produces both metrics and logs.

Others collect metrics alone, such as the scenario built on [`prometheus.exporter.snmp`][prometheus.exporter.snmp].
The Kubernetes scenarios mostly rely on Helm charts, though one scrapes kube-state-metrics and cAdvisor directly with [`discovery.kubernetes`][discovery.kubernetes].

{{< docs/shared lookup="scenarios/infrastructure-and-platforms.md" source="alloy" version="<ALLOY_VERSION>" >}}

[prometheus.exporter.unix]: ../../reference/components/prometheus/prometheus.exporter.unix/
[prometheus.exporter.windows]: ../../reference/components/prometheus/prometheus.exporter.windows/
[prometheus.exporter.snmp]: ../../reference/components/prometheus/prometheus.exporter.snmp/
[discovery.kubernetes]: ../../reference/components/discovery/discovery.kubernetes/

---
canonical: https://grafana.com/docs/alloy/latest/scenarios/infrastructure-and-platforms/
description: Runnable scenarios that monitor hosts, containers, network devices, cloud providers, and Kubernetes with Grafana Alloy
menuTitle: Infrastructure and platforms
title: Infrastructure and platform scenarios
weight: 600
---

# Infrastructure and platform scenarios

These scenarios monitor the layer your applications run on: Linux and Windows hosts, Docker containers, network devices, cloud provider APIs, and Kubernetes clusters.

Most pair a platform exporter such as [`prometheus.exporter.unix`][prometheus.exporter.unix], [`prometheus.exporter.windows`][prometheus.exporter.windows], or [`prometheus.exporter.snmp`][prometheus.exporter.snmp] with log collection from the same machine, so a single scenario produces both metrics and logs.
The Kubernetes scenarios use [`discovery.kubernetes`][discovery.kubernetes] to find targets as workloads come and go.

{{< docs/shared lookup="scenarios/infrastructure-and-platforms.md" source="alloy" version="<ALLOY_VERSION>" >}}

[prometheus.exporter.unix]: ../../reference/components/prometheus/prometheus.exporter.unix/
[prometheus.exporter.windows]: ../../reference/components/prometheus/prometheus.exporter.windows/
[prometheus.exporter.snmp]: ../../reference/components/prometheus/prometheus.exporter.snmp/
[discovery.kubernetes]: ../../reference/components/discovery/discovery.kubernetes/

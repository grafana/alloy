---
canonical: https://grafana.com/docs/alloy/latest/introduction/estimate-resource-usage/
aliases:
  - ../tasks/estimate-resource-usage/ # /docs/alloy/latest/tasks/estimate-resource-usage/
description: Estimate expected Grafana Alloy resource usage
headless: true
title: Estimate Grafana Alloy resource usage
menuTitle: Estimate resource usage
weight: 300
---

# Estimate {{% param "FULL_PRODUCT_NAME" %}} resource usage

This page provides guidance for expected resource usage of {{< param "PRODUCT_NAME" >}} for each telemetry type, based on operational experience of some of the {{< param "PRODUCT_NAME" >}} maintainers.

{{< admonition type="note" >}}
The resource usage depends on the workload, hardware, and the configuration used.
The information on this page is a good starting point for most users, but your actual usage may be different.
{{< /admonition >}}

## Prometheus metrics

The Prometheus metrics resource usage depends mainly on the number of active series that need to be scraped and the scrape interval.

As a rule of thumb, **per each 1 million active series** and with the default scrape interval, you can expect to use approximately:

* 0.4 CPU cores
* 11 GiB of memory
* 1.5 MiB/s of total network bandwidth, send and receive

These recommendations are based on deployments that use [clustering][], but they will broadly apply to other deployment modes.
Refer to [Deploy {{< param "FULL_PRODUCT_NAME" >}}][deploy] for more information on how to deploy {{< param "PRODUCT_NAME" >}}.

## Loki logs

Loki logs resource usage depends mainly on the volume of logs ingested.

As a rule of thumb, **per each 1 MiB/second of logs ingested**, you can expect to use approximately:

* 1 CPU core
* 120 MiB of memory

These recommendations are based on Kubernetes DaemonSet deployments on clusters with relatively small number of nodes and high logs volume on each.
The resource usage can be higher per each 1 MiB/second of logs if you have a large number of small nodes due to the constant overhead of running the {{< param "PRODUCT_NAME" >}} on each node.

Additionally, factors such as number of labels, number of files and average log line length may all play a role in the resource usage.

## Pyroscope profiles

Pyroscope profiles resource usage depends mainly on the volume of profiles.

As a rule of thumb, **per each 100 profiles/second**, you can expect to use approximately:

* 1 CPU core
* 10 GiB of memory

Factors such as size of each profile and frequency of fetching them also play a role in the overall resource usage.

[deploy]: ../../set-up/deploy/
[clustering]: ../../get-started/clustering/

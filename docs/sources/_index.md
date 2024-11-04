---
canonical: https://grafana.com/docs/alloy/latest/
title: Grafana Alloy
description: Grafana Alloy is a a vendor-neutral distribution of the OTel Collector
weight: 350
cascade:
  ALLOY_RELEASE: v1.5.0
  OTEL_VERSION: v0.105.0
  PROM_WIN_EXP_VERSION: v0.27.3
  SNMP_VERSION: v0.26.0
  FULL_PRODUCT_NAME: Grafana Alloy
  PRODUCT_NAME: Alloy
hero:
  title: Grafana Alloy
  level: 1
  image: /media/docs/alloy/alloy_icon.png
  width: 110
  height: 110
  description: >-
    Grafana Alloy is a vendor-neutral distribution of the OpenTelemetry (OTel) Collector. With Alloy, you can instrument your app or infrastrastructure to collect, process, and forward telemetry data to the data source of your choice. 
cards:
  title_class: pt-0 lh-1
  items:
    - title: Install Alloy
      href: ./get-started/install/
      description: Learn how to install and uninstall Alloy on Docker, Kubernetes, Linux, macOS, or Windows.
    - title: Run Alloy
      href: ./get-started/run/
      description: Learn how to start, restart, and stop Alloy after you have installed it.
    - title: Configure Alloy
      href: ./tasks/configure/
      description: Learn how to configure Alloy on Kubernetes, Linux, macOS, or Windows.
    - title: Migrate to Alloy
      href: ./tasks/migrate/
      description: Learn how to migrate to Alloy from Grafana Agent Operator, Prometheus, Promtail, Grafana Agent Static, or Grafana Agent Flow.
    - title: Collect OpenTelemetry data
      href: ./tasks/collect-opentelemetry-data/
      description: You can configure Alloy to collect OpenTelemetry-compatible data and forward it to any OpenTelemetry-compatible endpoint. Learn how to configure OpenTelemetry data delivery, configure batching, and receive OpenTelemetry data over OTLP.
    - title: Collect and forward Prometheus metrics
      href: ./tasks/collect-prometheus-metrics/
      description: You can configure Alloy to collect Prometheus metrics and forward them to any Prometheus-compatible database. Learn how to configure metrics delivery and collect metrics from Kubernetes Pods.
    - title: Concepts
      href: ./concepts/
      description: Learn about components, modules, clustering, and the Alloy configuration syntax.
    - title: Reference
      href: ./reference/
      description: Read the reference documentation about the command line tools, configuration blocks, components, and standard library.
---

{{< docs/hero-simple key="hero" >}}

---

# Overview

Getting the relevant telemetry data (i.e. metrics, logs, and traces) for analysis is an indispensable part of understanding the health of your system. 

Think of {{< param "PRODUCT_NAME" >}} as a Swiss army knife for collecting, processing, and forwarding telemetry data to the data source of your choosing. 

{{< param "PRODUCT_NAME" >}} has the following features to help you customize, scale, secure, and troubleshoot your data pipeline.
1. Custom components
1. GitOps compatibility
1. Clustering support
1. Security
1. Debugging utilities

Check out the {{< param "PRODUCT_NAME" >}} [Introduction] page for more information on these and other key features.

{{< param "PRODUCT_NAME" >}} is flexible, and you can easily configure it to fit your needs in on-prem, cloud-only, or a mix of both.

Getting started with Alloy consists of 3 major steps:
1. Install {{< param "PRODUCT_NAME" >}} 
1. Configure {{< param "PRODUCT_NAME" >}} 
1. Collect and forward telemetry data to the data source of choice

In addition, you can use Grafana dashboard to visualize the data collected from app or infrastructure.

For a quick overview of this process, check out the following tutorials.
* [Use Grafana Alloy to send logs to Loki](https://grafana.com/docs/alloy/latest/tutorials/send-logs-to-loki/)
* [Use Grafana Alloy to send metrics to Prometheus](https://grafana.com/docs/alloy/latest/tutorials/send-metrics-to-prometheus/)

## Explore

{{< card-grid key="cards" type="simple" >}}

[OTel]: https://opentelemetry.io/ecosystem/distributions/
[Prometheus]: https://prometheus.io/
[Pyroscope]: https://grafana.com/docs/pyroscope/
[Loki]: https://grafana.com/docs/loki/
[Mimir]: https://grafana.com/docs/mimir/
[Promtail]: https://grafana.com/docs/loki/latest/send-data/promtail/
[Introduction]: ./introduction/

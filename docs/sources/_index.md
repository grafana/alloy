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
    Grafana Alloy is a vendor-neutral distribution of the OpenTelemetry (OTel) Collector. With Alloy, you can instrument your application or infrastructure to collect, process, and forward telemetry data to the observability backend of your choice.
cards: 
  title_class: pt-0 lh-1
  items:
    - title: Introduction
      href:  ./introduction/
      description: Discover more about the key features and benefits of Alloy.
    - title: Concepts
      href: ./get-started/
      description: Learn about components, modules, clustering, and the Alloy configuration syntax.
    - title: Install Alloy
      href: ./set-up/install/
      description: Learn how to install and uninstall Alloy on Docker, Kubernetes, Linux, macOS, or Windows.
    - title: Configure Alloy
      href: ./configure/
      description: Learn how to configure Alloy on Kubernetes, Linux, macOS, or Windows.
    - title: Run Alloy
      href: ./set-up/run/
      description: Learn how to start, restart, and stop Alloy after you have installed it.
    - title: Migrate to Alloy
      href: ./set-up/migrate/
      description: Learn how to migrate to Alloy from Grafana Agent Operator, Prometheus, Promtail, Grafana Agent Static, or Grafana Agent Flow.
    - title: Use Alloy to send logs to Loki
      href: ./tutorials/send-logs-to-loki/
      description: Learn how to use Grafana Alloy to send logs to Loki.
    - title: Use Alloy to send metrics to Prometheus
      href: ./tutorials/send-metrics-to-prometheus/
      description: Learn how to use Grafana Alloy to send metrics to Prometheus.
    - title: Reference
      href: ./reference/
      description: Read the reference documentation about the command line tools, configuration blocks, components, and standard library.
---

{{< docs/hero-simple key="hero" >}}

---

# Overview

Collecting the relevant telemetry data, such as metrics, logs, and traces, for analysis is an indispensable part of understanding the health of your system.

{{< param "PRODUCT_NAME" >}} is more than just a collector. You can use {{< param "PRODUCT_NAME" >}} to collect, process, and forward telemetry data to the observability backend of your choosing.

{{< param "PRODUCT_NAME" >}} has the following features to help you customize, scale, secure, and troubleshoot your data pipeline.

* Custom components
* GitOps compatibility
* Clustering support
* Security
* Debugging utilities

{{< param "PRODUCT_NAME" >}} is flexible, and you can easily configure it to fit your needs in on-premises, cloud, or a mix of on-premises and cloud. .

## Explore

{{< card-grid key="cards" type="simple" >}}

[OTel]: https://opentelemetry.io/ecosystem/distributions/
[Prometheus]: https://prometheus.io/
[Pyroscope]: https://grafana.com/docs/pyroscope/
[Loki]: https://grafana.com/docs/loki/
[Mimir]: https://grafana.com/docs/mimir/
[Promtail]: https://grafana.com/docs/loki/latest/send-data/promtail/

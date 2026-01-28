---
canonical: https://grafana.com/docs/alloy/latest/
title: Grafana Alloy
description: Grafana Alloy is a vendor-neutral distribution of the OTel Collector
weight: 350
cascade:
  ALLOY_RELEASE: v1.12.2 # x-release-please-version
  OTEL_VERSION: v0.142.0
  PROM_WIN_EXP_VERSION: v0.31.3
  SNMP_VERSION: v0.29.0
  BEYLA_VERSION: v2.8.5
  FULL_PRODUCT_NAME: Grafana Alloy
  PRODUCT_NAME: Alloy
hero:
  title: Grafana Alloy
  level: 1
  image: /media/docs/alloy/alloy_icon.png
  width: 110
  height: 110
  description: >-
    Grafana Alloy combines the strengths of the leading collectors into one place. Whether observing applications, infrastructure, or both, Grafana Alloy can collect, process, and export telemetry signals to scale and future-proof your observability approach.
cards:
  title_class: pt-0 lh-1
  items:
    - title: Introduction to Alloy
      href: ./introduction/
      description: Learn about what Alloy can do for you.
    - title: Install Alloy
      href: ./set-up/install/
      description: Learn how to install and uninstall Alloy on Docker, Kubernetes, Linux, macOS, or Windows.
    - title: Run Alloy
      href: ./set-up/run/
      description: Learn how to start, restart, and stop Alloy after you have installed it.
    - title: Configure Alloy
      href: ./configure/
      description: Learn how to configure Alloy on Kubernetes, Linux, macOS, or Windows.
    - title: Migrate to Alloy
      href: ./set-up/migrate/
      description: Learn how to migrate to Alloy from Grafana Agent Operator, Prometheus, Promtail, Grafana Agent Static, or Grafana Agent Flow.
    - title: Collect OpenTelemetry data
      href: ./collect/opentelemetry-data/
      description: Learn how to configure OpenTelemetry data delivery, configure batching, and receive OpenTelemetry data over OTLP.
    - title: Concepts
      href: ./get-started/
      description: Learn about components, modules, clustering, and the Alloy configuration syntax.
    - title: Reference
      href: ./reference/
      description: Read the reference documentation about the command line tools, configuration blocks, components, and standard library.
---

{{< docs/hero-simple key="hero" >}}

---

# Overview

{{< figure src="/media/docs/alloy/alloy_diagram_v2.svg" alt="Alloy flow diagram" >}}

**Collect all your telemetry with one product**

Choosing the right tools to collect, process, and export telemetry data can be a confusing and costly experience.
The broad range of telemetry you need to process and the collectors you choose can vary widely depending on your observability goals.
In addition, you face the challenge of addressing the constantly evolving needs of your observability strategy.
For example, you may initially only need application observability, but you then discover that you must add infrastructure observability.
Many organizations manage and configure multiple collectors to address these challenges, introducing more complexity and potential errors in their obervability strategy.

**All signals, whether application, infrastructure, or both**

{{< param "FULL_PRODUCT_NAME" >}} has native pipelines for leading telemetry signals, such as Prometheus and OpenTelemetry, and databases such as Loki and Pyroscope.
This permits logs, metrics, traces, and even mature support for profiling.

**Enterprise strength observability**

{{< param "FULL_PRODUCT_NAME" >}} improves reliability and provides advanced features for Enterprise needs, such as clusters of fleets and balancing workloads.
Grafana [Fleet Management](https://grafana.com/docs/grafana-cloud/send-data/fleet-management/) helps you manage multiple {{< param "FULL_PRODUCT_NAME" >}} deployments at scale.

## Explore

{{< card-grid key="cards" type="simple" >}}

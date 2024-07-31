---
canonical: https://grafana.com/docs/alloy/latest/
title: Grafana Alloy
description: Grafana Alloy is a a vendor-neutral distribution of the OTel Collector
weight: 350
cascade:
  ALLOY_RELEASE: v1.3.0
  OTEL_VERSION: v0.105.0
  FULL_PRODUCT_NAME: Grafana Alloy
  PRODUCT_NAME: Alloy
hero:
  title: Grafana Alloy
  level: 1
  image: /media/docs/alloy/alloy_icon.png
  width: 110
  height: 110
  description: >-
    Grafana Alloy is a vendor-neutral distribution of the OpenTelemetry (OTel) Collector. Alloy uniquely combines the very best OSS observability signals in the community.
cards:
  title_class: pt-0 lh-1
  items:
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
      description: You can configure Alloy to collect OpenTelemetry-compatible data and forward it to any OpenTelemetry-compatible endpoint. Learn how to configure OpenTelemetry data delivery, configure batching, and receive OpenTelemetry data over OTLP.
    - title: Collect and forward Prometheus metrics
      href: ./collect/prometheus-metrics/
      description: You can configure Alloy to collect Prometheus metrics and forward them to any Prometheus-compatible database. Learn how to configure metrics delivery and collect metrics from Kubernetes Pods.
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

{{< param "PRODUCT_NAME" >}} offers native pipelines for [OTel][], [Prometheus][], [Pyroscope][], [Loki][], and many other metrics, logs, traces, and profile tools.
In addition, you can use {{< param "PRODUCT_NAME" >}} pipelines to do different tasks, such as configure alert rules in Loki and [Mimir][].
{{< param "PRODUCT_NAME" >}} is fully compatible with the OTel Collector, Prometheus Agent, and [Promtail][].
You can use {{< param "PRODUCT_NAME" >}} as an alternative to either of these solutions or combine it into a hybrid system of multiple collectors and agents.
You can deploy {{< param "PRODUCT_NAME" >}} anywhere within your IT infrastructure and pair it with your Grafana LGTM stack, a telemetry backend from Grafana Cloud, or any other compatible backend from any other vendor.
{{< param "PRODUCT_NAME" >}} is flexible, and you can easily configure it to fit your needs in on-prem, cloud-only, or a mix of both.

{{< admonition type="tip" >}}
{{< param "PRODUCT_NAME" >}} uses the same components, code, and concepts that were first introduced in Grafana Agent Flow.
{{< /admonition >}}

## What can {{% param "PRODUCT_NAME" %}} do?

{{< param "PRODUCT_NAME" >}} is more than just observability signals like metrics, logs, and traces. It provides many features that help you quickly find and process your data in complex environments.
Some of these features include custom components, GitOps compatibility, clustering support, security, and debugging utilities. Refer to the {{< param "PRODUCT_NAME" >}} [Introduction] for more information on these and other key features.

## Explore

{{< card-grid key="cards" type="simple" >}}

[OTel]: https://opentelemetry.io/ecosystem/distributions/
[Prometheus]: https://prometheus.io/
[Pyroscope]: https://grafana.com/docs/pyroscope/
[Loki]: https://grafana.com/docs/loki/
[Mimir]: https://grafana.com/docs/mimir/
[Promtail]: https://grafana.com/docs/loki/latest/send-data/promtail/
[Introduction]: ./introduction/

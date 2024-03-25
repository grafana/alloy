---
canonical: https://grafana.com/docs/alloy/latest/
title: Grafana Alloy
description: Grafana Alloy is a flexible, performant, vendor-neutral, telemetry collector
weight: 350
cascade:
  ALLOY_RELEASE: v1.0.0
  OTEL_VERSION: v0.87.0
  PRODUCT_NAME: Grafana Alloy
  PRODUCT_ROOT_NAME: Alloy
  _build:
    list: false
  noindex: true
---

# {{% param "PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} is a vendor-agnostic distribution of the [OpenTelemetry][] (OTel) Collector.
{{< param "PRODUCT_ROOT_NAME" >}} uniquely combines the very best OSS observability signals in the community, and it offers native pipelines for both OTel and [Prometheus][] telemetry formats, supporting metrics, logs, traces, and profiles.
{{< param "PRODUCT_ROOT_NAME" >}} is fully compatible with the OTel Collector and the Prometheus Agent.
You can use {{< param "PRODUCT_ROOT_NAME" >}} as an alternative to either of these solutions or combined into a hybrid system of multiple collectors and agents.
You can deploy {{< param "PRODUCT_ROOT_NAME" >}} anywhere within your IT infrastructure and you can pair it with a telemetry backend from Grafana Cloud or any other compatible backend from any other vendor.
{{< param "PRODUCT_ROOT_NAME" >}} is flexible, and you can easily configure it to fit your needs in on-prem, cloud-only, or a mix of both.

## What can {{% param "PRODUCT_ROOT_NAME" %}} do?

{{< param "PRODUCT_ROOT_NAME" >}} is more than just signals. It provides many features that help you quickly find and process your data in complex environments.
Some of these features include:

* **Modules:** {{< param "PRODUCT_ROOT_NAME" >}} uses modules to help you quickly build production-ready pipelines.
  Modules break down large configuration files into single, easy-to-understand modules that are just a few lines long.
  You can use pre-built community modules or modules packaged by Grafana, or create your own custom modules.
* **GitOps compatibility:** {{< param "PRODUCT_ROOT_NAME" >}} uses frameworks to pull configurations from Git, S3, HTTP endpoints, and just about any other source.
* **Clustering support:** {{< param "PRODUCT_ROOT_NAME" >}} has native clustering support.
  Clustering helps distribute the workload and ensures you have high availability.
  You can quickly create horizontally scalable deployments with minimal resource and operational overhead.
* **Security:** {{< param "PRODUCT_ROOT_NAME" >}} helps you manage authentication credentials and connect to HashiCorp Vaults to retrieve secrets.
* **Debugging utilities:** {{< param "PRODUCT_ROOT_NAME" >}} provides troubleshooting support and an embedded [user interface][UI] to help you identify and resolve configuration problems.

## Supported platforms

* Linux

  * Minimum version: kernel 2.6.32 or later
  * Architectures: AMD64, ARM64

* Windows

  * Minimum version: Windows Server 2016 or later, or Windows 10 or later.
  * Architectures: AMD64

* macOS

  * Minimum version: macOS 10.13 or later
  * Architectures: AMD64 (Intel), ARM64 (Apple Silicon)

* FreeBSD

  * Minimum version: FreeBSD 10 or later
  * Architectures: AMD64

[OpenTelemetry]: https://opentelemetry.io/ecosystem/distributions/
[Prometheus]: https://prometheus.io
[Loki]: https://github.com/grafana/loki
[UI]: ./tasks/debug/#grafana-alloy-ui

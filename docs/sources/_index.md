---
canonical: https://grafana.com/docs/alloy/latest/
title: Grafana Alloy
description: Grafana Alloy is a flexible, performant, vendor-neutral, telemetry collector
weight: 350
cascade:
  ALLOY_RELEASE: $ALLOY_VERSION
  OTEL_VERSION: v0.87.0
  PRODUCT_NAME: Grafana Alloy
  PRODUCT_ROOT_NAME: Alloy
---

# {{% param "PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} is a distribution of the [OpenTelemetry][] (OTel) Collector, and it is fully compatible with the Open Telemetry Protocol (OTLP). It offers native pipelines for both OTel and Prometheus telemetry, supporting metrics, logs, traces, and profiles.

{{< param "PRODUCT_NAME" >}} is designed to be much more than just a simple telemetry collector. Some of {{< param "PRODUCT_NAME" >}}'s features include:

* Modules to help you quickly build production-ready pipelines.
* Frameworks that can pull configurations from Git, S3, HTTP endpoints, and just about any other source.
* Clustering support to help you create horizontally scalable deployments with minimal resource and operational overhead.
* Security features to help you manage authentication credentials and connect to HashiCorp Vaults.
* Debugging support and an embedded user interface to help you troubleshoot configuration problems.

{{< param "PRODUCT_NAME" >}} is vendor-agnostic and fully compatible with the OTel collector or [Prometheus][] Agent. You can deploy {{< param "PRODUCT_NAME" >}} anywhere within your IT infrastructure and you can pair it with Grafana Cloud or a backend from any other vendor. {{< param "PRODUCT_NAME" >}} is flexible and can easily meet your needs in on-prem, cloud-only, or a mix of both.



{{< param "PRODUCT_NAME" >}} is based around **components**. Components are wired together to form programmable observability **pipelines** for telemetry collection, processing, and delivery.

{{< param "PRODUCT_NAME" >}} can collect, transform, and send data to:

* The [Prometheus][] ecosystem
* The [OpenTelemetry][] ecosystem
* The Grafana open source ecosystem ([Loki][], [Grafana][], [Tempo][], [Mimir][], [Pyroscope][])

## Why use {{< param "PRODUCT_NAME" >}}?

* **Vendor-neutral**: Fully compatible with the Prometheus, OpenTelemetry, and Grafana open source ecosystems.
* **Every signal**: Collect telemetry data for metrics, logs, traces, and continuous profiles.
* **Scalable**: Deploy on any number of machines to collect millions of active series and terabytes of logs.
* **Battle-tested**: {{< param "PRODUCT_NAME" >}} extends the existing battle-tested code from the Prometheus and OpenTelemetry Collector projects.
* **Powerful**: Write programmable pipelines with ease, and debug them using a [built-in UI][UI].
* **Batteries included**: Integrate with systems like MySQL, Kubernetes, and Apache to get telemetry that's immediately useful.

<!--
## Getting started

* Choose a [variant][variants] of {{< param "PRODUCT_NAME" >}} to run.
* Refer to the documentation for the variant to use:
  * [Static mode][]
  * [Static mode Kubernetes operator][]
  * [Flow mode][]

[variants]: ./about/
[Static mode]: https://grafana.com/docs/agent/static/
[Static mode Kubernetes operator]: https://grafana.com/docs/agent/operator/
[Flow mode]: https://grafana.com/docs/agent/flow/

-->

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

## Release cadence

A new minor release is planned every six weeks for the entire {{< param "PRODUCT_NAME" >}}.

The release cadence is best-effort: if necessary, releases may be performed
outside of this cadence, or a scheduled release date can be moved forwards or
backwards.

Minor releases published on cadence include updating dependencies for upstream
OpenTelemetry Collector code if new versions are available. Minor releases
published outside of the release cadence may not include these dependency
updates.

Patch and security releases may be created at any time.

[Terraform]: https://terraform.io
[Prometheus]: https://prometheus.io
[OpenTelemetry]: https://opentelemetry.io
[Loki]: https://github.com/grafana/loki
[Grafana]: https://github.com/grafana/grafana
[Tempo]: https://github.com/grafana/tempo
[Mimir]: https://github.com/grafana/mimir
[Pyroscope]: https://github.com/grafana/pyroscope
[UI]: ./tasks/debug/#grafana-alloy-ui

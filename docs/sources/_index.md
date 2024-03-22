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

{{< param "PRODUCT_NAME" >}} is an OpenTelemetry Collector distribution with configuration inspired by [Terraform][].
It is designed to be flexible, performant, and compatible with multiple ecosystems such as Prometheus and OpenTelemetry.

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

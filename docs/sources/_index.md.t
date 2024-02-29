---
aliases:
- ./docs/alloy/
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

{{< param "PRODUCT_NAME" >}} is a vendor-neutral, batteries-included telemetry collector with
configuration inspired by [Terraform][]. It is designed to be flexible,
performant, and compatible with multiple ecosystems such as Prometheus and
OpenTelemetry.

{{< param "PRODUCT_NAME" >}} is based around **components**. Components are wired together to
form programmable observability **pipelines** for telemetry collection,
processing, and delivery.

{{< admonition type="note" >}}
This page focuses mainly on [Flow mode](https://grafana.com/docs/alloy/<ALLOY_VERSION>/), the Terraform-inspired variant of {{< param "PRODUCT_NAME" >}}.

For information on other variants of {{< param "PRODUCT_NAME" >}}, refer to [Introduction to {{< param "PRODUCT_NAME" >}}]({{< relref "./about.md" >}}).
{{< /admonition >}}

{{< param "PRODUCT_NAME" >}} can collect, transform, and send data to:

* The [Prometheus][] ecosystem
* The [OpenTelemetry][] ecosystem
* The Grafana open source ecosystem ([Loki][], [Grafana][], [Tempo][], [Mimir][], [Pyroscope][])

[Terraform]: https://terraform.io
[Prometheus]: https://prometheus.io
[OpenTelemetry]: https://opentelemetry.io
[Loki]: https://github.com/grafana/loki
[Grafana]: https://github.com/grafana/grafana
[Tempo]: https://github.com/grafana/tempo
[Mimir]: https://github.com/grafana/mimir
[Pyroscope]: https://github.com/grafana/pyroscope

## Why use {{% param "PRODUCT_NAME" %}}?

* **Vendor-neutral**: Fully compatible with the Prometheus, OpenTelemetry, and
  Grafana open source ecosystems.
* **Every signal**: Collect telemetry data for metrics, logs, traces, and
  continuous profiles.
* **Scalable**: Deploy on any number of machines to collect millions of active
  series and terabytes of logs.
* **Battle-tested**: {{< param "PRODUCT_NAME" >}} extends the existing battle-tested code from
  the Prometheus and OpenTelemetry Collector projects.
* **Powerful**: Write programmable pipelines with ease, and debug them using a
  [built-in UI][UI].
* **Batteries included**: Integrate with systems like MySQL, Kubernetes, and
  Apache to get telemetry that's immediately useful.

## Getting started

* Choose a [variant][variants] of {{< param "PRODUCT_NAME" >}} to run.
* Refer to the documentation for the variant to use:
  * [Static mode][]
  * [Static mode Kubernetes operator][]
  * [Flow mode][]

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

A new minor release is planned every six weeks for the entire {{< param "PRODUCT_NAME" >}}
project, including Static mode, the Static mode Kubernetes operator, and Flow
mode.

The release cadence is best-effort: releases may be moved forwards or backwards
if needed. The planned release dates for future minor releases do not change if
one minor release is moved.

Patch and security releases may be created at any time.

{{% docs/reference %}}
[variants]: "/docs/alloy/ -> /docs/alloy/<ALLOY_VERSION>/about"
[variants]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/about"

[Static mode]: "/docs/alloy/ -> /docs/alloy/<ALLOY_VERSION>/static"
[Static mode]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/static"

[Static mode Kubernetes operator]: "/docs/alloy/ -> /docs/alloy/<ALLOY_VERSION>/operator"
[Static mode Kubernetes operator]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/operator"

[Flow mode]: "/docs/alloy/ -> /docs/alloy/<ALLOY_VERSION>/flow"
[Flow mode]: "/docs/grafana-cloud/ -> /docs/alloy/<ALLOY_VERSION>/flow"

[UI]: "/docs/alloy/ -> /docs/alloy/<ALLOY_VERSION>/tasks/debug.md#grafana-agent-flow-ui"
[UI]: "/docs/grafana-cloud/ -> /docs/alloy/<ALLOY_VERSION>/tasks/debug.md#grafana-agent-flow-ui"
{{% /docs/reference %}}

# {{% param "PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} is a _component-based_ revision of {{< param "PRODUCT_ROOT_NAME" >}} with a focus on ease-of-use,
debuggability, and ability to adapt to the needs of power users.

Components allow for reusability, composability, and focus on a single task.

* **Reusability** allows for the output of components to be reused as the input for multiple other components.
* **Composability** allows for components to be chained together to form a pipeline.
* **Single task** means the scope of a component is limited to one narrow task and thus has fewer side effects.

## Features

* Write declarative configurations with a Terraform-inspired configuration
  language.
* Declare components to configure parts of a pipeline.
* Use expressions to bind components together to build a programmable pipeline.
* Includes a UI for debugging the state of a pipeline.

## Example

```river
// Discover Kubernetes pods to collect metrics from
discovery.kubernetes "pods" {
  role = "pod"
}

// Scrape metrics from Kubernetes pods and send to a prometheus.remote_write
// component.
prometheus.scrape "default" {
  targets    = discovery.kubernetes.pods.targets
  forward_to = [prometheus.remote_write.default.receiver]
}

// Get an API key from disk.
local.file "apikey" {
  filename  = "/var/data/my-api-key.txt"
  is_secret = true
}

// Collect and send metrics to a Prometheus remote_write endpoint.
prometheus.remote_write "default" {
  endpoint {
    url = "http://localhost:9009/api/prom/push"

    basic_auth {
      username = "MY_USERNAME"
      password = local.file.apikey.content
    }
  }
}
```


## {{% param "PRODUCT_NAME" %}} configuration generator

The {{< param "PRODUCT_NAME" >}} [configuration generator](https://grafana.github.io/agent-configurator/) helps you get a head start on creating flow code.

{{< admonition type="note" >}}
This feature is experimental, and it doesn't support all River components.
{{< /admonition >}}

## Next steps

* [Install][] {{< param "PRODUCT_NAME" >}}.
* Learn about the core [Concepts][] of {{< param "PRODUCT_NAME" >}}.
* Follow the [Tutorials][] for hands-on learning of {{< param "PRODUCT_NAME" >}}.
* Consult the [Tasks][] instructions to accomplish common objectives with {{< param "PRODUCT_NAME" >}}.
* Check out the [Reference][] documentation to find specific information you might be looking for.

{{% docs/reference %}}
[Install]: "/docs/alloy/ -> /docs/alloy/<ALLOY_VERSION>/get-started/install/"
[Install]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/get-started/install/"
[Concepts]: "/docs/alloy/ -> /docs/alloy/<ALLOY_VERSION>/concepts/"
[Concepts]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/concepts/"
[Tasks]: "/docs/alloy/ -> /docs/alloy/<ALLOY_VERSION>/tasks/"
[Tasks]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/tasks/"
[Tutorials]: "/docs/alloy/ -> /docs/alloy/<ALLOY_VERSION>/tutorials/"
[Tutorials]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/tutorials/
[Reference]: "/docs/alloy/ -> /docs/alloy/<ALLOY_VERSION>/reference/"
[Reference]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/reference/
{{% /docs/reference %}}

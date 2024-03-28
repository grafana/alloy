---
canonical: https://grafana.com/docs/alloy/latest/about/
description: Alloy is a flexible, performant, vendor-neutral, telemetry collector
menuTitle: Introduction
title: Introduction to Alloy
weight: 10
---

# Introduction to {{% param "PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} is a flexible, high performance, vendor-neutral telemetry collector. It's fully compatible with the most popular open source observability standards such as OpenTelemetry (OTel) and Prometheus.

{{< param "PRODUCT_NAME" >}} is a _component-based_ revision of {{< param "PRODUCT_NAME" >}} with a focus on ease-of-use,
debuggability, and ability to adapt to the needs of power users.

Components allow for reusability, composability, and focus on a single task.

* **Reusability** allows for the output of components to be reused as the input for multiple other components.
* **Composability** allows for components to be chained together to form a pipeline.
* **Single task** means the scope of a component is limited to one narrow task and thus has fewer side effects.

## Features

* Write declarative configurations with a Terraform-inspired configuration language.
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

The {{< param "PRODUCT_NAME" >}} [configuration generator][] helps you get a head start on creating {{< param "PRODUCT_NAME" >}} configurations.

{{< admonition type="note" >}}
This feature is experimental, and it doesn't support all {{< param "PRODUCT_NAME" >}} components.
{{< /admonition >}}

## Next steps

* [Install][] {{< param "PRODUCT_NAME" >}}.
* Learn about the core [Concepts][] of {{< param "PRODUCT_NAME" >}}.
* Follow the [Tutorials][] for hands-on learning of {{< param "PRODUCT_NAME" >}}.
* Consult the [Tasks][] instructions to accomplish common objectives with {{< param "PRODUCT_NAME" >}}.
* Check out the [Reference][] documentation to find specific information you might be looking for.

[configuration generator]: https://grafana.github.io/agent-configurator/
[Install]: ../get-started/install/
[Concepts]: ../concepts/
[Tasks]: ../tasks/
[Tutorials]: ../tutorials/
[Reference]: ../reference/

<!--
## Choose which variant of {{% param "PRODUCT_NAME" %}} to run

> **NOTE**: You don't have to pick just one variant; it's possible to
> mix-and-match installations of Grafana Agent.

### Compare variants

Each variant of {{< param "PRODUCT_NAME" >}} provides a different level of functionality. The following tables compare {{< param "PRODUCT_NAME" >}} Flow mode with Static mode, Operator, OpenTelemetry, and Prometheus.

#### Core telemetry

|              | Grafana Agent Flow mode  | Grafana Agent Static mode | Grafana Agent Operator | OpenTelemetry Collector | Prometheus Agent mode |
|--------------|--------------------------|---------------------------|------------------------|-------------------------|-----------------------|
| **Metrics**  | [Prometheus][], [OTel][] | Prometheus                | Prometheus             | OTel                    | Prometheus            |
| **Logs**     | [Loki][], [OTel][]       | Loki                      | Loki                   | OTel                    | No                    |
| **Traces**   | [OTel][]                 | OTel                      | OTel                   | OTel                    | No                    |
| **Profiles** | [Pyroscope][]            | No                        | No                     | Planned                 | No                    |

#### **OSS features**

|                          | Grafana Agent Flow mode | Grafana Agent Static mode | Grafana Agent Operator | OpenTelemetry Collector | Prometheus Agent mode |
|--------------------------|-------------------------|---------------------------|------------------------|-------------------------|-----------------------|
| **Kubernetes native**    | [Yes][helm chart]       | No                        | Yes                    | Yes                     | No                    |
| **Clustering**           | [Yes][clustering]       | No                        | No                     | No                      | No                    |
| **Prometheus rules**     | [Yes][rules]            | No                        | No                     | No                      | No                    |
| **Native Vault support** | [Yes][vault]            | No                        | No                     | No                      | No                    |

#### Grafana Cloud solutions

|                               | Grafana Agent Flow mode | Grafana Agent Static mode | Grafana Agent Operator | OpenTelemetry Collector | Prometheus Agent mode |
|-------------------------------|-------------------------|---------------------------|------------------------|-------------------------|-----------------------|
| **Official vendor support**   | [Yes][sla]              | Yes                       | Yes                    | No                      | No                    |
| **Cloud integrations**        | Some                    | Yes                       | Some                   | No                      | No                    |
| **Kubernetes monitoring**     | [Yes][helm chart]       | Yes, custom               | Yes                    | No                      | Yes, custom           |
| **Application observability** | [Yes][observability]    | No                        | No                     | Yes                     | No                    |

### Static mode

[Static mode][] is the original variant of Grafana Agent, introduced on March 3, 2020.
Static mode is the most mature variant of Grafana Agent.

You should run Static mode when:

* **Maturity**: You need to use the most mature version of Grafana Agent.

* **Grafana Cloud integrations**: You need to use Grafana Agent with Grafana Cloud integrations.

### Static mode Kubernetes operator

{{< admonition type="note" >}}
Grafana Agent version 0.37 and newer provides Prometheus Operator compatibility in Flow mode.
You should use Grafana Agent Flow mode for all new Grafana Agent deployments.
{{< /admonition >}}

The [Static mode Kubernetes operator][] is a variant of Grafana Agent introduced on June 17, 2021. It's currently in beta.

The Static mode Kubernetes operator provides compatibility with Prometheus Operator,
allowing static mode to support resources from Prometheus Operator, such as ServiceMonitors, PodMonitors, and Probes.

You should run the Static mode Kubernetes operator when:

* **Prometheus Operator compatibility**: You need to be able to consume
  ServiceMonitors, PodMonitors, and Probes from the Prometheus Operator project
  for collecting Prometheus metrics.

### Flow mode

[Flow mode][] is a stable variant of Grafana Agent, introduced on September 29, 2022.

Grafana Agent Flow mode focuses on vendor neutrality, ease-of-use,
improved debugging, and ability to adapt to the needs of power users by adopting a configuration-as-code model.

You should run Flow mode when:

* You need functionality unique to Flow mode:

  * **Improved debugging**: You need to more easily debug configuration issues using a UI.

  * **Full OpenTelemetry support**: Support for collecting OpenTelemetry metrics, logs, and traces.

  * **PrometheusRule support**: Support for the PrometheusRule resource from the Prometheus Operator project for configuring Grafana Mimir.

  * **Ecosystem transformation**: You need to be able to convert Prometheus and Loki pipelines to and from OpenTelmetry Collector pipelines.

  * **Grafana Pyroscope support**: Support for collecting profiles for Grafana Pyroscope.

[Pyroscope]: https://grafana.com/docs/pyroscope/latest/configure-client/grafana-agent/go_pull
[helm chart]: https://grafana.com/docs/grafana-cloud/monitor-infrastructure/kubernetes-monitoring/configuration/config-k8s-helmchart
[sla]: https://grafana.com/legal/grafana-cloud-sla
[observability]: https://grafana.com/docs/grafana-cloud/monitor-applications/application-observability/setup#send-telemetry

[integrations]: https://grafana.com/docs/agent/static/configuration/integrations/
[components]: ./reference/components
[Static mode]: https://grafana.com/docs/agent/static/
[Static mode Kubernetes operator]: https://grafana.com/docs/agent/operator/
[Flow mode]: https://grafana.com/docs/agent/flow/
[Prometheus]: ./tasks/collect-prometheus-metrics/
[OTel]: ./tasks/collect-opentelemetry-data/
[Loki]: ./tasks/migrate/from-promtail/
[clustering]: ./concepts/clustering/
[rules]: ./reference/components/mimir.rules.kubernetes/
[vault]: ./reference/components/remote.vault/

-->

<!--
### BoringCrypto

[BoringCrypto][] is an **EXPERIMENTAL** feature for building {{< param "PRODUCT_NAME" >}}
binaries and images with BoringCrypto enabled. Builds and Docker images for Linux arm64/amd64 are made available.

[BoringCrypto]: https://pkg.go.dev/crypto/internal/boring
-->

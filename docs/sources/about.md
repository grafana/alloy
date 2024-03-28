---
canonical: https://grafana.com/docs/alloy/latest/about/
description: Alloy is a flexible, performant, vendor-neutral, telemetry collector
menuTitle: Introduction
title: Introduction to Alloy
weight: 10
---

# Introduction to {{% param "FULL_PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} is a flexible, high performance, vendor-neutral telemetry collector. It's fully compatible with the most popular open source observability standards such as OpenTelemetry (OTel) and Prometheus.

{{< param "PRODUCT_NAME" >}} focuses on ease-of-use, debuggability, and ability to adapt to the needs of power users.

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

```alloy
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

[configuration generator]: https://grafana.github.io/alloy-configurator/
[Install]: ../get-started/install/
[Concepts]: ../concepts/
[Tasks]: ../tasks/
[Tutorials]: ../tutorials/
[Reference]: ../reference/

### Compare distributions

The following tables compare {{< param "PRODUCT_NAME" >}} with OpenTelemetry and Prometheus.

#### Core telemetry

|              | Grafana Alloy            | OpenTelemetry Collector | Prometheus Agent |
|--------------|--------------------------|-------------------------|------------------|
| **Metrics**  | [Prometheus][], [OTel][] | OTel                    | Prometheus       |
| **Logs**     | [Loki][], [OTel][]       | OTel                    | No               |
| **Traces**   | [OTel][]                 | OTel                    | No               |
| **Profiles** | [Pyroscope][]            | Planned                 | No               |

#### **OSS features**

|                          | Grafana Alloy     | OpenTelemetry Collector | Prometheus Agent |
|--------------------------|-------------------|-------------------------|------------------|
| **Kubernetes native**    | [Yes][helm chart] | Yes                     | No               |
| **Clustering**           | [Yes][clustering] | No                      | No               |
| **Prometheus rules**     | [Yes][rules]      | No                      | No               |
| **Native Vault support** | [Yes][vault]      | No                      | No               |

#### Grafana Cloud solutions

|                               | Grafana Alloy        | OpenTelemetry Collector | Prometheus Agent |
|-------------------------------|----------------------|-------------------------|------------------|
| **Official vendor support**   | [Yes][sla]           | No                      | No               |
| **Cloud integrations**        | Some                 | No                      | No               |
| **Kubernetes monitoring**     | [Yes][helm chart]    | No                      | Yes, custom      |
| **Application observability** | [Yes][observability] | Yes                     | No               |

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

<!--
### BoringCrypto

[BoringCrypto][] is an **EXPERIMENTAL** feature for building {{< param "PRODUCT_NAME" >}}
binaries and images with BoringCrypto enabled. Builds and Docker images for Linux arm64/amd64 are made available.

[BoringCrypto]: https://pkg.go.dev/crypto/internal/boring
-->
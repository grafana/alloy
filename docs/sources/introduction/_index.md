---
canonical: https://grafana.com/docs/alloy/latest/introduction/
description: Grafana Alloy is a flexible, high performance, vendor-neutral distribution of the OTel Collector
menuTitle: Introduction
title: Introduction to Grafana Alloy
weight: 10
---

# Introduction to {{% param "FULL_PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} is a flexible, high performance, vendor-neutral distribution of the [OpenTelemetry][] (OTel) Collector.
It's fully compatible with the most popular open source observability standards such as OpenTelemetry (OTel), Prometheus, and .

{{< param "PRODUCT_NAME" >}} focuses on ease-of-use and the ability to adapt to the needs of power users.

## Key features

Some of the key features of {{< param "PRODUCT_NAME" >}} include:

* **Custom components:** You can use {{< param "PRODUCT_NAME" >}} to create and share custom components.
  Custom components combine a pipeline of existing components into a single, easy-to-understand component that is just a few lines long.
  You can use pre-built custom components from the community, ones packaged by Grafana, or create your own.
* **Reusable components:** You can use the output of a component as the input for multiple other components.
* **Chained components:** You can chain components together to form a pipeline.
* **Single task per component:** The scope of each component is limited to one specific task.
* **GitOps compatibility:** {{< param "PRODUCT_NAME" >}} uses frameworks to pull configurations from Git, S3, HTTP endpoints, and just about any other source.
* **Clustering support:** {{< param "PRODUCT_NAME" >}} has native clustering support.
  Clustering helps distribute the workload and ensures you have high availability.
  You can quickly create horizontally scalable deployments with minimal resource and operational overhead.
* **Security:** {{< param "PRODUCT_NAME" >}} helps you manage authentication credentials and connect to HashiCorp Vaults or Kubernetes clusters to retrieve secrets.
* **Debugging utilities:** {{< param "PRODUCT_NAME" >}} provides troubleshooting support and an embedded [user interface][UI] to help you identify and resolve configuration problems.

### Compare {{% param "PRODUCT_NAME" %}} with OpenTelemetry and Prometheus

The following tables compare some of the features of {{< param "PRODUCT_NAME" >}} with OpenTelemetry and Prometheus.

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

<!--
### BoringCrypto

[BoringCrypto][] is an **EXPERIMENTAL** feature for building {{< param "PRODUCT_NAME" >}}
binaries and images with BoringCrypto enabled. Builds and Docker images for Linux arm64/amd64 are made available.

[BoringCrypto]: https://pkg.go.dev/crypto/internal/boring
-->

## Next steps

* [Install][] {{< param "PRODUCT_NAME" >}}.
* Learn about the core [Concepts][] of {{< param "PRODUCT_NAME" >}}.
* Follow the [Tutorials][] for hands-on learning of {{< param "PRODUCT_NAME" >}}.
* Consult the [Tasks][] instructions to accomplish common objectives with {{< param "PRODUCT_NAME" >}}.
* Check out the [Reference][] documentation to find specific information you might be looking for.

[Install]: ../get-started/install/
[Concepts]: ../concepts/
[Tasks]: ../tasks/
[Tutorials]: ../tutorials/
[Reference]: ../reference/
[Pyroscope]: https://grafana.com/docs/pyroscope/latest/configure-client/grafana-agent/go_pull
[helm chart]: https://grafana.com/docs/grafana-cloud/monitor-infrastructure/kubernetes-monitoring/configuration/config-k8s-helmchart
[sla]: https://grafana.com/legal/grafana-cloud-sla
[observability]: https://grafana.com/docs/grafana-cloud/monitor-applications/application-observability/setup#send-telemetry
[components]: ./reference/components
[Prometheus]: ./tasks/collect-prometheus-metrics/
[OTel]: ./tasks/collect-opentelemetry-data/
[Loki]: ./tasks/migrate/from-promtail/
[clustering]: ./concepts/clustering/
[rules]: ./reference/components/mimir.rules.kubernetes/
[vault]: ./reference/components/remote.vault/

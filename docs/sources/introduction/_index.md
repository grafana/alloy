---
canonical: https://grafana.com/docs/alloy/latest/introduction/
description: Grafana Alloy is a flexible, high performance, vendor-neutral distribution of the OTel Collector
menuTitle: Introduction
title: Introduction to Grafana Alloy
weight: 10
---

# Introduction to {{% param "FULL_PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} is a flexible, high performance, vendor-neutral distribution of the [OpenTelemetry][] (OTel) Collector.
It's fully compatible with the most popular open source observability standards such as OpenTelemetry (OTel) and Prometheus.

{{< param "PRODUCT_NAME" >}} focuses on ease-of-use and the ability to adapt to the needs of power users.

## Key features

Some of the key features of {{< param "PRODUCT_NAME" >}} include:

* **Custom components:** You can use {{< param "PRODUCT_NAME" >}} to create and share custom components.
  Custom components combine a pipeline of existing components into a single, easy-to-understand component that's just a few lines long.
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

## How does {{% param "PRODUCT_NAME" %}} work as an OpenTelemetry collector?

{{< figure src="/media/docs/alloy/flow-diagram-small-alloy.png" alt="Alloy flow diagram" >}}

### Collect

{{< param "PRODUCT_NAME" >}} uses more than 120 components to collect telemetry data from applications, databases, and OpenTelemetry collectors.
{{< param "PRODUCT_NAME" >}} supports collection using multiple ecosystems, including OpenTelemetry and Prometheus.

Telemetry data can be either pushed to {{< param "PRODUCT_NAME" >}}, or {{< param "PRODUCT_NAME" >}} can pull it from your data sources.

### Transform

{{< param "PRODUCT_NAME" >}} processes data and transforms it for sending.

You can use transformations to inject extra metadata into telemetry or filter out unwanted data.

### Write

{{< param "PRODUCT_NAME" >}} sends data to OpenTelemetry-compatible databases or collectors, the Grafana LGTM stack, or Grafana Cloud.

{{< param "PRODUCT_NAME" >}} can also write alerting rules in compatible databases.

## Next steps

* [Install][] {{< param "PRODUCT_NAME" >}}.
* Learn about the core [Concepts][] of {{< param "PRODUCT_NAME" >}}.
* Follow the [tutorials][] for hands-on learning about {{< param "PRODUCT_NAME" >}}.
* Learn how to [collect and forward data][Collect] with {{< param "PRODUCT_NAME" >}}.
* Check out the [reference][] documentation to find information about the {{< param "PRODUCT_NAME" >}} components, configuration blocks, and command line tools.

[OpenTelemetry]: https://opentelemetry.io/ecosystem/distributions/
[Install]: ../set-up/install/
[Concepts]: ../get-started/
[Collect]: ../collect/
[tutorials]: ../tutorials/
[reference]: ../reference/
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

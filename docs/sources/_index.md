---
canonical: https://grafana.com/docs/alloy/latest/
title: Grafana Alloy
description: Grafana Alloy is a a vendor-neutral distribution of the OTel Collector
weight: 350
cascade:
  ALLOY_RELEASE: v1.0.0
  OTEL_VERSION: v0.87.0
  FULL_PRODUCT_NAME: Grafana Alloy
  PRODUCT_NAME: Alloy
---

# {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} is a vendor-neutral distribution of the [OpenTelemetry][] (OTel) Collector.
{{< param "PRODUCT_NAME" >}} uniquely combines the very best OSS observability signals in the community.
It offers native pipelines for OTel, [Prometheus][], [Pyroscope][], [Loki][], and many other metrics, logs, traces, and profile tools.
In additon, you can use {{< param "PRODUCT_NAME" >}} pipelines to do other tasks such as configure alert rules in Loki and Mimir.
{{< param "PRODUCT_NAME" >}} is fully compatible with the OTel Collector, Prometheus Agent, and Promtail.
You can use {{< param "PRODUCT_NAME" >}} as an alternative to either of these solutions or combined into a hybrid system of multiple collectors and agents.
You can deploy {{< param "PRODUCT_NAME" >}} anywhere within your IT infrastructure and you can pair it with your Grafana LGTM stack, a telemetry backend from Grafana Cloud, or any other compatible backend from any other vendor.
{{< param "PRODUCT_NAME" >}} is flexible, and you can easily configure it to fit your needs in on-prem, cloud-only, or a mix of both.

## What can {{% param "PRODUCT_NAME" %}} do?

{{< param "PRODUCT_NAME" >}} is more than just observability signals like metrics, logs, and traces. It provides many features that help you quickly find and process your data in complex environments.
Some of these features include:

* **Custom components:** You can use {{< param "PRODUCT_NAME" >}} to create and share custom components.
  Custom components combine a pipeline of existing components into a single, easy-to-understand component that's just a few lines long.
  You can use pre-built custom components from the community, ones packaged by Grafana, or create your own.
* **GitOps compatibility:** {{< param "PRODUCT_NAME" >}} uses frameworks to pull configurations from Git, S3, HTTP endpoints, and just about any other source.
* **Clustering support:** {{< param "PRODUCT_NAME" >}} has native clustering support.
  Clustering helps distribute the workload and ensures you have high availability.
  You can quickly create horizontally scalable deployments with minimal resource and operational overhead.
* **Security:** {{< param "PRODUCT_NAME" >}} helps you manage authentication credentials and connect to HashiCorp Vaults or Kubernetes clusters to retrieve secrets.
* **Debugging utilities:** {{< param "PRODUCT_NAME" >}} provides troubleshooting support and an embedded [user interface][UI] to help you identify and resolve configuration problems.

[OpenTelemetry]: https://opentelemetry.io/ecosystem/distributions/
[Prometheus]: https://prometheus.io/
[Loki]: https://grafana.com/docs/loki/
[Grafana]: https://grafana.com/docs/grafana/
[Tempo]: https://grafana.com/docs/tempo/
[Mimir]: https://grafana.com/docs/mimir/
[Pyroscope]: https://grafana.com/docs/pyroscope/
[UI]: ./tasks/debug/#alloy-ui

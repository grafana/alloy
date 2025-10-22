---
canonical: https://grafana.com/docs/alloy/latest/collect/choose-component/
description: Find out which components are useful for which tasks
title: Choose a Grafana Alloy component
menuTitle: Choose a component
weight: 100
---

# Choose a  {{% param "FULL_PRODUCT_NAME" %}} component

[Components][components] are the building blocks of {{< param "FULL_PRODUCT_NAME" >}}, and there is a [large number of them][components-ref].
The components you select and configure depend on the telemetry signals you want to collect.

[components]: ../../get-started/components/
[components-ref]: ../../reference/components/

## Metrics for infrastructure

Use `prometheus.*` components to collect infrastructure metrics.
This gives you the best experience with [Grafana Infrastructure Observability][].

For example, you can get metrics for a Linux host using `prometheus.exporter.unix`, and metrics for a MongoDB instance using `prometheus.exporter.mongodb`.

You can also scrape any Prometheus endpoint using `prometheus.scrape`.
Use `discovery.*` components to find targets for `prometheus.scrape`.

[Grafana Infrastructure Observability]:https://grafana.com/docs/grafana-cloud/monitor-infrastructure/

## Metrics for applications

Use `otelcol.receiver.*` components to collect application metrics.
This gives you the best experience with [Grafana Application Observability][], which is OpenTelemetry-native.

For example, use `otelcol.receiver.otlp` to collect metrics from OpenTelemetry-instrumented applications.

If your application is already instrumented with Prometheus metrics, there is no need to use `otelcol.*` components.
Use `prometheus.*` components for the entire pipeline and send the metrics using `prometheus.remote_write`.

[Grafana Application Observability]:https://grafana.com/docs/grafana-cloud/monitor-applications/application-observability/introduction/

## Logs from infrastructure

Use `loki.*` components to collect infrastructure logs.
The `loki.*` components label your logs in a way that resembles Prometheus metrics.
This makes it easy to correlate infrastructure metrics collected by `prometheus.*` components
with logs collected by `loki.*` components.

For example, the label that both `prometheus.*` and `loki.*` components would use for a Kubernetes namespace is called `namespace`.
On the other hand, gathering logs using an `otelcol.*` component might use the [OpenTelemetry semantics][OTel-semantics] label called `k8s.namespace.name`,
which wouldn't correspond to the `namespace` label that's common in the Prometheus ecosystem.

## Logs from applications

Use `otelcol.receiver.*` components to collect application logs.
This gathers the application logs in an OpenTelemetry-native way, making it easier to
correlate the logs with OpenTelemetry metrics and traces coming from the application.
All application telemetry must follow the [OpenTelemetry semantic conventions][OTel-semantics], simplifying this correlation.

For example, if your application runs on Kubernetes, every trace, log, and metric can have a `k8s.namespace.name` resource attribute.

[OTel-semantics]:https://opentelemetry.io/docs/concepts/semantic-conventions/

## Traces

Use `otelcol.receiver.*` components to collect traces.

If your application isn't yet instrumented for tracing, use `beyla.ebpf` to generate traces for it automatically.

## Profiles

Use `pyroscope.*` components to collect profiles.

## Frontend telemetry

In order to use [Grafana Cloud Frontend Observability][frontend-observability], you have to collect and forward frontend telemetry using `otelcol.receiver.faro` and `otelcol.exporter.faro`.

You can also gather frontend telemetry using `faro.receiver` and send it to Grafana Cloud, but [Grafana Cloud Frontend Observability][frontend-observability] will not work and you will need to create your own dashboards.

`faro.receiver` is recommended only for a self-hosted Grafana setup.

[frontend-observability]: https://grafana.com/docs/grafana-cloud/monitor-applications/frontend-observability/
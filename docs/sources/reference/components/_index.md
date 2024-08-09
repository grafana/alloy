---
canonical: https://grafana.com/docs/alloy/latest/reference/components/
description: Learn about the components in Grafana Alloy
title: Components
weight: 300
---

# Components

This section contains reference documentation for all recognized [components][].

{{< section >}}

[components]: ../../get-started/components/

## How to collect different telemetry signals

### Metrics for infrastructure

Use `prometheus.*` components for collecting infrastructure metrics.
This will give you the best experience with [Grafana Infrastructure Observability][].

For example, you can get metrics for a Linux host using `prometheus.exporter.unix`, 
and metrics for a MongoDB instance using `prometheus.exporter.mongodb`. 

You can also scrape any Prometheus endpoint using `prometheus.scrape`.
Use `discovery.*` components to find targets for `prometheus.scrape`.

[Grafana Infrastructure Observability]:https://grafana.com/docs/grafana-cloud/monitor-infrastructure/

### Metrics for applications

Use `otelcol.receiver.*` components for collecting application metrics.
This will give you the best experience with [Grafana Application Observability][], which is OpenTelemetry-native.

For example, use `otelcol.receiver.otlp` to collect metrics from OpenTelemetry-instrumented applications.

If your application is already instrumented with Prometheus metrics, there is no need to use `otelcol.*` components.
Use `prometheus.*` components for the entire pipeline and send the metrics using `prometheus.remote_write`.

[Grafana Application Observability]:https://grafana.com/docs/grafana-cloud/monitor-applications/application-observability/introduction/

### Logs from infrastructure

Use `loki.*` components to collect infrastructure logs.
The `loki.*` components label your logs in a way which resembles Prometheus metrics.
This makes it easy to correlate infrastructure metrics collected by `prometheus.*` components
with logs collected by `loki.*` components.

For example, the label which both `prometheus.*` and `loki.*` components would use for a Kubernetes namespace is called `namespace`.
On the other hand, gathering logs using an `otelcol.*` component might use the [OpenTelemetry semantics][OTel-semantics] label called `k8s.namespace.name`,
which wouldn't correspond to the `namespace` label that is common in the Prometheus ecosystem.

### Logs from applications

Use `otelcol.receiver.*` components to collect application logs.
This will gather the application logs in an OpenTelemetry-native way, which will make it easier to 
correlate the logs with OpenTelemetry metrics and traces coming from the application.
To make this correlation easier, all application telemetry must follow the [OpenTelemetry semantic conventions][OTel-semantics].

For example, if your application is running on Kubernetes, every trace, log, and metric may have a `k8s.namespace.name` resource attribute.


[OTel-semantics]:https://opentelemetry.io/docs/concepts/semantic-conventions/

### Traces

Use `otelcol.receiver.*` components for collecting traces.

If your application is not yet instrumented for tracing, use `beyla.ebpf` to generate traces for it automatically.

### Profiles

Use `pyroscope.*` components to collect profiles.

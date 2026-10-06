---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/helm/
description: Learn how to run the Alloy OpenTelemetry Engine with the OpenTelemetry Collector Helm chart
menuTitle: Helm chart
review_date: 2026-09-23
title: Run the Alloy OpenTelemetry Engine with the OpenTelemetry Collector Helm chart
weight: 300
---

# Run the {{% param "FULL_OTEL_ENGINE" %}} with the OpenTelemetry Collector Helm chart

Use the upstream [OpenTelemetry Collector Helm chart][Chart] to run the {{< param "OTEL_ENGINE" >}} on Kubernetes.
The chart deploys the {{< param "PRODUCT_NAME" >}} image with the standard upstream chart values, so you configure it the same way you configure any OpenTelemetry Collector deployment.

{{< docs/shared lookup="stability/experimental_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Before you begin

Make sure you have the following:

- [Helm][] installed on your computer
- A Kubernetes cluster that you can use for {{< param "PRODUCT_NAME" >}}
- A local Kubernetes context that points at the cluster

## Run the {{% param "OTEL_ENGINE" %}}

1. Add the OpenTelemetry Collector Helm chart repository:

   ```shell
   helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
   helm repo update
   ```

1. Create a `values.yaml` file.
   The following example runs the configuration from [Send data to Grafana Cloud][CLICloud] in a Kubernetes Deployment:

   ```yaml
   image:
     repository: grafana/alloy
     tag: <ALLOY_VERSION>

   command:
     name: "bin/otelcol"

   mode: deployment

   ports:
     metrics:
       enabled: true

   config:
     extensions:
       basicauth/my_auth:
         client_auth:
           username: "<USERNAME>"
           password: "<PASSWORD>"

     receivers:
       otlp:
         protocols:
           grpc: {}
           http: {}
       jaeger: null
       zipkin: null
       prometheus: null

     processors:
       batch:
         timeout: 1s
         send_batch_size: 512

     exporters:
       debug: null
       otlp_http/my_backend:
         endpoint: <URL>
         auth:
           authenticator: basicauth/my_auth

     service:
       telemetry:
         metrics:
           readers:
             - pull:
                 exporter:
                   prometheus:
                     host: 0.0.0.0
                     port: 8888
       extensions: [basicauth/my_auth, health_check]
       pipelines:
         logs: null
         metrics: null
         traces:
           receivers: [otlp]
           processors: [batch]
           exporters: [otlp_http/my_backend]
   ```

   Replace the following:

   - _`<ALLOY_VERSION>`_: The {{< param "PRODUCT_NAME" >}} release you want to run, for example {{< param "ALLOY_RELEASE" >}}.
   - _`<USERNAME>`_: Your Grafana Cloud instance ID.
   - _`<PASSWORD>`_: Your Grafana Cloud API token.
   - _`<URL>`_: Your Grafana Cloud OTLP endpoint URL.

   For more information about where to find the Grafana Cloud values, refer to [Send data using OpenTelemetry Protocol][SendOTLP].

   The chart runs `/<COMMAND_NAME>` as the container command, so `bin/otelcol` resolves to `/bin/otelcol`.
   This path is a compatibility entrypoint in the {{< param "PRODUCT_NAME" >}} image that runs `alloy otel`.
   Without this setting, the chart runs the image's default entrypoint, which starts the {{< param "DEFAULT_ENGINE" >}}.

   The example sets the chart defaults it doesn't need to `null` and uses the chart's own `health_check` extension without redefining it.
   Refer to the [Helm chart documentation][ChartConfig] for how `config` combines with the chart's defaults.
   The example also sets the metrics endpoint host to `0.0.0.0` so it listens on all interfaces inside the Pod.
   The chart's default binds the Pod IP, which other Pods in the cluster can already reach, so set `0.0.0.0` when you also want to reach the endpoint on `127.0.0.1` inside the Pod, for example from a sidecar container.
   Set `ports.metrics.enabled` to `true` to expose port `8888` on the Pod and the Service.

1. Install the chart:

   ```shell
   helm install <RELEASE_NAME> open-telemetry/opentelemetry-collector --values values.yaml
   ```

   Replace _`<RELEASE_NAME>`_ with a name for your Helm release.

1. Verify that the Pod runs:

   ```shell
   kubectl get pods
   ```

   The Pod reaches `Running` status once the {{< param "OTEL_ENGINE" >}} starts.

## Configuration options

Configure the {{< param "OTEL_ENGINE" >}} through the chart's `config` field, as the preceding example does.
The [Helm chart documentation][ChartConfig] describes this field and how your values combine with the chart's defaults.

The chart also provides `alternateConfig`, which replaces the chart's default configuration instead of combining with it.
Upstream provides that field for use as a subchart and doesn't recommend it when you install the chart directly.

Refer to the [upstream documentation][ChartDocs] for more information about configuring the Helm chart for your use case.

[Chart]: https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-collector
[ChartConfig]: https://opentelemetry.io/docs/platforms/kubernetes/helm/collector/#configuration
[ChartDocs]: https://opentelemetry.io/docs/platforms/kubernetes/helm/collector/
[CLICloud]: ../cli/#send-data-to-grafana-cloud
[Helm]: https://helm.sh
[SendOTLP]: https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/

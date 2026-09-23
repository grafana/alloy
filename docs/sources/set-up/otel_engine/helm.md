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

- A Kubernetes cluster
- Helm 3

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

   alternateConfig:
     extensions:
       health_check:
         endpoint: 0.0.0.0:13133 # This is necessary for the Kubernetes liveness check
       basicauth/my_auth:
         client_auth:
           username: <USERNAME>
           password: <PASSWORD>

     receivers:
       otlp:
         protocols:
           grpc: {}
           http: {}

     processors:
       batch:
         timeout: 1s
         send_batch_size: 512

     exporters:
       otlphttp/my_backend:
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
         traces:
           receivers: [otlp]
           processors: [batch]
           exporters: [otlphttp/my_backend]
   ```

   Replace the following:

   - _`<ALLOY_VERSION>`_: The {{< param "PRODUCT_NAME" >}} release you want to run, for example `v1.19.0`.
   - _`<USERNAME>`_: Your Grafana Cloud instance ID.
   - _`<PASSWORD>`_: Your Grafana Cloud API token.
   - _`<URL>`_: Your Grafana Cloud OTLP endpoint URL.

   For more information about where to find the Grafana Cloud values, refer to [Send data using OpenTelemetry Protocol][SendOTLP].

   The chart runs `/<COMMAND_NAME>` as the container command, so `bin/otelcol` resolves to `/bin/otelcol`.
   This path is a compatibility entrypoint in the {{< param "PRODUCT_NAME" >}} image that runs `alloy otel`.
   Without this setting, the chart runs the image's default entrypoint, which starts the {{< param "DEFAULT_ENGINE" >}}.

   The chart's default configuration doesn't apply when you set `alternateConfig`, so this example declares every component it needs, including the `health_check` extension.
   The example also sets the metrics endpoint host to `0.0.0.0` so it listens on all interfaces inside the Pod.
   This lets other Pods in the cluster reach it without `kubectl port-forward`.
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

The Helm chart includes a default OpenTelemetry Collector configuration in the `config` field.
The [Helm chart documentation][ChartConfig] describes this field.
You have three ways to configure the {{< param "OTEL_ENGINE" >}}:

- **Replace the defaults**: Set `alternateConfig`, as the preceding example does. The chart ignores `config` entirely and uses only what you provide. You must supply the `health_check` extension yourself, because the chart's `readinessProbe` and `livenessProbe` checks depend on it.
- **Merge with the defaults**: Set `config`. The chart merges your values into its default configuration. Maps merge key by key, and lists replace the default list.
- **Remove a default**: Set a default key to `null` within `config`. This works when you install the chart directly, but not when you use it as a subchart.

Refer to the [upstream documentation][ChartDocs] for more information about configuring the Helm chart for your use case.

[Chart]: https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-collector
[ChartConfig]: https://opentelemetry.io/docs/platforms/kubernetes/helm/collector/#configuration
[ChartDocs]: https://opentelemetry.io/docs/platforms/kubernetes/helm/collector/
[CLICloud]: ../cli/#send-data-to-grafana-cloud
[SendOTLP]: https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/

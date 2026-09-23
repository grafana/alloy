---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/helm/
description: Learn how to run the OpenTelemetry Engine with the OpenTelemetry Collector Helm chart
menuTitle: Helm chart
title: Run the OpenTelemetry Engine with the OpenTelemetry Collector Helm chart
weight: 300
---

# Run the {{% param "OTEL_ENGINE" %}} with the OpenTelemetry Collector Helm chart

Use the upstream [OpenTelemetry Collector Helm chart][Chart] to run the {{< param "OTEL_ENGINE" >}}.
This approach delivers an identical upstream collector experience.
It also ensures you get improvements, bug fixes, and security updates as they're released.

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
     tag: latest

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

   - _`<USERNAME>`_: Your Grafana Cloud instance ID.
   - _`<PASSWORD>`_: Your Grafana Cloud API token.
   - _`<URL>`_: Your Grafana Cloud OTLP endpoint URL.

   The `command.name` key points at `/bin/otelcol`, a compatibility entrypoint in the {{< param "PRODUCT_NAME" >}} image that runs `alloy otel`.
   The Helm chart doesn't expose custom commands, so this setting is necessary.

   Binding port `8888` to `0.0.0.0` makes the metrics endpoint listen on all interfaces inside the Pod.
   This lets other Pods in the cluster reach it without using `kubectl port-forward`.

1. Install the chart:

   ```shell
   helm install <RELEASE_NAME> open-telemetry/opentelemetry-collector --values values.yaml
   ```

   Replace _`<RELEASE_NAME>`_ with a name for your Helm release.

1. Verify that the Pod runs:

   ```shell
   kubectl get pods
   ```

## Configuration options

The Helm chart ships with a default OpenTelemetry Collector configuration in the `config` field.
The upstream Helm chart [documentation][ChartConfig] describes this field.
If you want to completely override that default configuration, use the `alternateConfig` field.
In the example, the `alternateConfig` field ensures the configuration matches the other {{< param "OTEL_ENGINE" >}} examples and doesn't inherit any of the chart's defaults.
Alternatively, you can omit both `config` and `alternateConfig` to use the default configuration as-is.
You can also provide your own `config` block that merges with the chart's default configuration.

Refer to the [upstream documentation][ChartDocs] for more information about how to configure the Helm chart to work for your use case.

[Chart]: https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-collector
[ChartConfig]: https://opentelemetry.io/docs/platforms/kubernetes/helm/collector/#configuration
[ChartDocs]: https://opentelemetry.io/docs/platforms/kubernetes/helm/collector/
[CLICloud]: ../cli/#send-data-to-grafana-cloud

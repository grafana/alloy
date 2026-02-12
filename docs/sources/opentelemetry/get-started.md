---
canonical: https://grafana.com/docs/alloy/latest/opentelemetry/get-started
description: Get started with the Alloy OpenTelemetry Engine
menuTitle: Get Started
title: Get Started with the Alloy OpenTelemetry Engine
_build:
  list: false
noindex: true
weight: 10
---

# Get Started with the {{% param "FULL_OTEL_ENGINE" %}}

You can run the {{< param "OTEL_ENGINE" >}} using the CLI, Helm chart, or service installation.

## Prerequisites

There are no additional prerequisites.
The tools needed to run the {{< param "OTEL_ENGINE" >}} are shipped within {{< param "PRODUCT_NAME" >}}.

Before you start, validate your OpenTelemetry YAML configuration with the `validate` command:

```bash
./build/alloy otel validate --config=<CONFIG_FILE>
```

Whilst this is an experimental feature, it is not hidden behind an `experimental` feature flag like regular components are.

## Run with the CLI

The {{< param "OTEL_ENGINE" >}} is available under the {{< param "PRODUCT_NAME" >}} `otel` command.
The CLI is the easiest way to experiment locally or on a single host.
Refer to the [OTel CLI](../../reference/cli/otel/) documentation for more information.

The following example configuration file accepts telemetry over [OTLP](https://opentelemetry.io/docs/specs/otel/protocol/) and sends it to the configured backend:

```yaml
extensions:
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
  extensions: [basicauth/my_auth]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/my_backend]
```

Replace the following:

- _`<USERNAME>`_: Your username, if you are using Grafana Cloud this will be your Grafana Cloud instance ID.
- _`<PASSWORD>`_: Your password, if you are using Grafana Cloud this will be your Grafana Cloud API token.
- _`<URL>`_: The URL to export data to, if you are using Grafana Cloud this will be your Grafana Cloud OTLP endpoint URL.

For more information about where to find these values for Grafana Cloud, refer to [Send data using OpenTelemetry Protocol](https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/).

To start the {{< param "OTEL_ENGINE" >}}, run the following command:

```shell
alloy otel --config=<CONFIG_FILE> [<FLAGS> ...]
```

{{< param "PRODUCT_NAME" >}} then accepts incoming OTLP data on `0.0.0.0:4317` for gRPC and `0.0.0.0:4318` for HTTP requests.
Metrics are also available on the default collector port and endpoint at `0.0.0.0:8888/metrics`.
Since the {{< param "DEFAULT_ENGINE" >}} isn't running, the UI and metrics aren't available at `0.0.0.0:12345/metrics`.

### Run the {{% param "PRODUCT_NAME" %}} Engine extension

You can also run the {{< param "OTEL_ENGINE" >}} with the {{< param "DEFAULT_ENGINE" >}}.
Modify your YAML configuration to include the `alloyengine` extension, which accepts a path to the {{< param "DEFAULT_ENGINE" >}} configuration and starts a {{< param "DEFAULT_ENGINE" >}} pipeline alongside the {{< param "OTEL_ENGINE" >}} pipeline.

The following example shows the configuration:

```yaml
extensions:
  basicauth/my_auth:
    client_auth:
      username: <USERNAME>
      password: <PASSWORD>
  alloyengine:
    config:
      file: <ALLOY_CONFIG_PATH>
    flags:
      server.http.listen-addr: 0.0.0.0:12345

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
  extensions: [basicauth/my_auth, alloyengine]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/my_backend]
```

Replace the following:

- _`<ALLOY_CONFIG_PATH>`_: The path to your {{< param "DEFAULT_ENGINE" >}} configuration file.
- _`<USERNAME>`_: Your username, if you are using Grafana Cloud this will be your Grafana Cloud instance ID.
- _`<PASSWORD>`_: Your password, if you are using Grafana Cloud this will be your Grafana Cloud API token.
- _`<URL>`_: The URL to export data to, if you are using Grafana Cloud this will be your Grafana Cloud OTLP endpoint URL.

This example adds the `alloyengine` block in the extension declarations and enables the extension in the `service` block.
You can then run {{< param "PRODUCT_NAME" >}} with the exact same command as before:

```shell
alloy otel --config=<CONFIG_FILE> [<FLAGS> ...]
```

This starts both the {{< param "DEFAULT_ENGINE" >}} and {{< param "OTEL_ENGINE" >}}.
The output of both engines is visible in the logs.
You can access the {{< param "DEFAULT_ENGINE" >}} UI and metrics on port `12345`.

## Run with The OpenTelemetry Collector Helm chart

Use the upstream OpenTelemetry Collector Helm chart run the {{< param "OTEL_ENGINE" >}} . 
This delivers an identical upstream collector experience and ensures you get improvements, bug fixes, and security updates as they are released.

The following example helm `values.yaml` incorporates the same configuration seen above into a Kubernetes deployment.

{{< admonition type="note" >}}
In this configuration, binding port `8888` to `0.0.0.0` makes the metrics endpoint listen on all interfaces inside the Pod. It can be reached from other Pods in the cluster and with `kubectl port-forward`.
{{< /admonition >}}

```yaml
image:
  repository: grafana/alloy
  tag: latest
  pullPolicy: IfNotPresent

command: 
  name: "bin/otelcol"

mode: deployment

ports:
  metrics:
    enabled: true

config:
  extensions:
    health_check:
      endpoint: 0.0.0.0:13133 # This is necessary for the k8s liveliness check
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

- _`<USERNAME>`_: Your username. If you are using Grafana Cloud this is your Grafana Cloud instance ID.
- _`<PASSWORD>`_: Your password. If you are using Grafana Cloud this is your Grafana Cloud API token.
- _`<URL>`_: The URL to export data to. If you are using Grafana Cloud this is your Grafana Cloud OTLP endpoint URL.

Please refer to the [upstream documentation](https://opentelemetry.io/docs/platforms/kubernetes/helm/collector/) for more information about how to configure the helm chart to work for your use case

**Note:**
In the above configuration, binding port `8888` to `0.0.0.0` makes the metrics endpoint listen on all interfaces inside the pod, so it can be reached both from other pods in the cluster and via kubectl port-forward.


## Run with service installation

Service installation support for systemd, launchd, and similar systems isn't included in the initial experimental release.
Service installers will work seamlessly with the {{< param "OTEL_ENGINE" >}} as the feature matures.
In the meantime, use the CLI or Helm options for testing.

## Considerations

1. **Storage configuration**: The {{< param "DEFAULT_ENGINE" >}} accepts the `--storage.path` flag to set a base directory for components to store data on disk.
   The {{< param "OTEL_ENGINE" >}} uses the `filestorage` extension instead of a CLI flag.
   Refer to the [upstream documentation](https://opentelemetry.io/docs/collector/resiliency/#persistent-storage-write-ahead-log---wal) for more information.
1. **Server ports**: The {{< param "DEFAULT_ENGINE" >}} exposes its HTTP server on port `12345`.
   The {{< param "OTEL_ENGINE" >}} exposes its HTTP server on port `8888`.
   The {{< param "OTEL_ENGINE" >}} HTTP server doesn't expose a UI, support bundles, or reload endpoint functionality like the {{< param "DEFAULT_ENGINE" >}}.
1. **Fleet management**: [Grafana Fleet Management](https://grafana.com/blog/opentelemetry-and-grafana-labs-whats-new-and-whats-next-in-2026/#fleet-management) isn't supported yet for the {{< param "OTEL_ENGINE" >}}.
   You must define and manage the input configuration manually.
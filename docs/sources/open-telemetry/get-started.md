---
canonical: https://grafana.com/docs/alloy/latest/opentelemetry/get-started
description: Getting Started with the Alloy OpenTelemetry Engine 
menuTitle: Get Started
title: Get Started with the Alloy OpenTelemetry Engine
_build:
  list: false
noindex: true
weight: 10
---

# Get Started with the {{% param "FULL_OTEL_ENGINE" %}}

You can run the {{< param "OTEL_ENGINE" >}} using the CLI, Helm chart, or service installation.

## Run with the CLI

The {{< param "OTEL_ENGINE" >}} is available under the {{< param "PRODUCT_NAME" >}} `otel` command.
The CLI is the easiest way to experiment locally or on a single host.
Refer to the [OTel CLI](../reference/cli/otel.md) documentation for more information.

The following example configuration file accepts OTLP and sends it to the Grafana Cloud OTLP gateway.
You can adapt the endpoint and credentials for your region and tenant.
For more information about where to find your `INSTANCE_ID`, `API_TOKEN`, and `GRAFANA_URL`, refer to [Send data using OpenTelemetry Protocol](https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/).

```yaml
extensions:
  basicauth/grafana_cloud:
    client_auth:
      username: INSTANCE_ID
      password: API_TOKEN

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
  otlphttp/grafana_cloud:
    endpoint: GRAFANA_URL
    auth:
      authenticator: basicauth/grafana_cloud

service:
  extensions: [basicauth/grafana_cloud]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/grafana_cloud]
```

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
  basicauth/grafana_cloud:
    client_auth:
      username: INSTANCE_ID
      password: API_TOKEN
  alloyengine:
    config:
      file: path/to/alloy-config.alloy
    flags:
      server.http.listen-addr: 0.0.0.0:12345
      stability.level: experimental

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
  otlphttp/grafana_cloud:
    endpoint: GRAFANA_URL
    auth:
      authenticator: basicauth/grafana_cloud

service:
  extensions: [basicauth/grafana_cloud, alloyengine]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/grafana_cloud]
```

This example adds the `alloyengine` block in the extension declarations and enables the extension in the `service` block.
You can then run {{< param "PRODUCT_NAME" >}} with the exact same command as before:

```shell
alloy otel --config=<CONFIG_FILE> [<FLAGS> ...] 
```

This starts both the {{< param "DEFAULT_ENGINE" >}} and {{< param "OTEL_ENGINE" >}}.
The output of both engines is visible in the logs.
You can access the {{< param "DEFAULT_ENGINE" >}} UI and metrics on port `12345`.

## Run with {{% param "PRODUCT_NAME" %}} Helm chart

TODO

## Run with service installation

Service installation support for systemd, launchd, and similar systems isn't included in the initial experimental release.
Service installers will work seamlessly with the {{< param "OTEL_ENGINE" >}} as the feature progresses.
In the meantime, use the CLI or Helm options for testing.

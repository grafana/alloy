---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/cli/
description: Learn how to run the OpenTelemetry Engine with the Alloy CLI
menuTitle: CLI
title: Run the OpenTelemetry Engine with the CLI
weight: 100
---

# Run the {{% param "OTEL_ENGINE" %}} with the CLI

The {{< param "OTEL_ENGINE" >}} is available under the {{< param "PRODUCT_NAME" >}} `otel` command.
The CLI is the easiest way to experiment locally or on a single host.
Refer to the [`otel`][OTelCommand] reference documentation for more information.

## Run the {{% param "OTEL_ENGINE" %}}

1. Create a configuration file named `config.yaml`.
   The following example accepts telemetry over [OTLP][] and prints it to the console:

   ```yaml
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
     debug:
       verbosity: detailed

   service:
     pipelines:
       traces:
         receivers: [otlp]
         processors: [batch]
         exporters: [debug]
   ```

1. Check the configuration:

   ```shell
   alloy otel validate --config=config.yaml
   ```

1. Start the {{< param "OTEL_ENGINE" >}}:

   ```shell
   alloy otel --config=config.yaml
   ```

   To pass extra command-line flags, refer to the [`otel` command reference][OTelCommand].

1. Verify that the {{< param "OTEL_ENGINE" >}} runs:

   ```shell
   curl http://localhost:8888/metrics
   ```

{{< param "PRODUCT_NAME" >}} then accepts incoming OTLP data on `0.0.0.0:4317` for gRPC and `0.0.0.0:4318` for HTTP requests.
Metrics are also available on the default collector port and endpoint at `0.0.0.0:8888/metrics`.
Since the {{< param "DEFAULT_ENGINE" >}} isn't running, the UI and metrics aren't available at `0.0.0.0:12345/metrics`.

## Send data to Grafana Cloud

The following example configuration accepts telemetry over OTLP and sends it to Grafana Cloud:

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

- _`<USERNAME>`_: Your Grafana Cloud instance ID.
- _`<PASSWORD>`_: Your Grafana Cloud API token.
- _`<URL>`_: Your Grafana Cloud OTLP endpoint URL.

For more information about where to find these values, refer to [Send data using OpenTelemetry Protocol][SendOTLP].

## Manage with Grafana Fleet Management

Use the embedded `otel-supervisor` command to manage the {{< param "OTEL_ENGINE" >}} through Grafana Fleet Management.
The supervisor starts {{< param "PRODUCT_NAME" >}} and receives OpenTelemetry Collector configuration from Grafana Fleet Management through OpAMP.

Refer to [`otel-supervisor`][OTelSupervisor] for setup instructions and required environment variables.

[OTLP]: https://opentelemetry.io/docs/specs/otel/protocol/
[OTelCommand]: ../../../reference/cli/otel/
[OTelSupervisor]: ../../../reference/cli/otel-supervisor/
[SendOTLP]: https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/

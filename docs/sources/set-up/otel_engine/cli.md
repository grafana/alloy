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
Refer to the [OTel CLI](../../../reference/cli/otel/) documentation for more information.

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

- _`<USERNAME>`_: Your username. If you're using Grafana Cloud, this is your Grafana Cloud instance ID.
- _`<PASSWORD>`_: Your password. If you're using Grafana Cloud, this is your Grafana Cloud API token.
- _`<URL>`_: The URL to export data to. If you're using Grafana Cloud, this is your Grafana Cloud OTLP endpoint URL.

For more information about where to find these values for Grafana Cloud, refer to [Send data using OpenTelemetry Protocol](https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/).

To start the {{< param "OTEL_ENGINE" >}}, run the following command:

```shell
alloy otel --config=<CONFIG_FILE> [<FLAGS> ...]
```

{{< param "PRODUCT_NAME" >}} then accepts incoming OTLP data on `0.0.0.0:4317` for gRPC and `0.0.0.0:4318` for HTTP requests.
Metrics are also available on the default collector port and endpoint at `0.0.0.0:8888/metrics`.
Since the {{< param "DEFAULT_ENGINE" >}} isn't running, the UI and metrics aren't available at `0.0.0.0:12345/metrics`.

## Manage with Grafana Fleet Management

Use the embedded `otel-supervisor` command to manage the {{< param "OTEL_ENGINE" >}} through Grafana Fleet Management.
The supervisor starts {{< param "PRODUCT_NAME" >}} and receives OpenTelemetry Collector configuration from Grafana Fleet Management through OpAMP.

Refer to [`otel-supervisor`](../../../reference/cli/otel-supervisor/) for setup instructions and required environment variables.

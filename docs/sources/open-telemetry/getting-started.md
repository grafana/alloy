---
canonical: https://grafana.com/docs/alloy/latest/opentelemetry/getting-started
description: Getting Started with the Open Telemetry Engine at Alloy
menuTitle: Getting Started
title: Getting Started with the Open Telemetry Engine at Alloy
_build:
  list: false
noindex: true
weight: 10
---

## Running with the CLI
The OTel Engine is exposed under the otel subcommand of the Alloy CLI. More details about the CLI surface and subcommands can be found [here](../reference/cli/otel.md). The CLI is the easiest way to experiment locally or on a single host.

This is a minimal example config file that accepts OTLP and sends it to the Grafana Cloud OTLP gateway. Adapt the endpoint and credentials for your region/tenant. You can find more information about where to find your INSTANCE_ID, API_TOKEN AND GRAFANA_URL [here](https://grafana.com/docs/grafana-cloud/send-data/otlp/send-data-otlp/)

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

Then to start the Alloy OTel Engine, run the following command

```shell
alloy otel --config=<CONFIG_FILE> [<FLAGS> ...] 
```

Alloy will then accept incoming OTLP data on `0.0.0.0:4317` for gRPC and `0.0.0.0:4318` for HTTP requests. Metrics are also available on the default collector port and endpoint at `0.0.0.0:8888/metrics`. Please note that since the Default Engine is not running, UI and metrics are _not_ available at `0.0.0.0:12345/metrics`.

### Running The Alloy Engine Extension

In addition to running the OTel Engine by itself, you can modify your YAML config to include the `Alloy Engine Extension`, which will accept a path to Default Engine configuration that will boot up a Default Engine pipeline alongside the OTel Engine pipeline.

You can see an example configuration below:

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

We've only added a couple lines here - the `alloyengine` block in the extension declarations, and enabled the extension in the `service` block. You can then run Alloy with the exact same command as before:

```shell
alloy otel --config=<CONFIG_FILE> [<FLAGS> ...] 
```

This will boot up both the Default _and_ OTel Engine, the output of both will be visible in the logs. You can now access the Default Engine UI and metrics on port `12345`.

## Running with Alloy Helm Chart
TODO

## Running with Service Installation

Service installation support (systemd, launchd, etc.) for running Alloy with the OTel Engine is planned but not included in the initial experimental release. We intend to ensure service installers work seamlessly with the OTel engine as the feature progresses. In the meantime, please use the CLI or Helm options above for testing.

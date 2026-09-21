---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/engine-extension/
description: Learn how to run a Default Engine pipeline inside the OpenTelemetry Engine with the Alloy Engine extension
menuTitle: Alloy Engine extension
title: Run the Alloy Engine extension
weight: 200
---

# Run the {{% param "PRODUCT_NAME" %}} Engine extension

You can run the {{< param "OTEL_ENGINE" >}} and the {{< param "DEFAULT_ENGINE" >}} in the same process.
Modify your YAML configuration to include the `alloyengine` extension.
This extension accepts a path to the {{< param "DEFAULT_ENGINE" >}} configuration or an inline {{< param "DEFAULT_ENGINE" >}} configuration.
It starts a {{< param "DEFAULT_ENGINE" >}} pipeline alongside the {{< param "OTEL_ENGINE" >}} pipeline.

You can also embed the `alloyengine` extension into any other OpenTelemetry Collector distribution using the OpenTelemetry Collector Builder (OCB).
Refer to the `alloyengine` extension [README](https://github.com/grafana/alloy/blob/main/extension/alloyengine/README.md#include-alloyengine-extension-in-an-ocb-distribution) for the instructions how to do this.

The following example uses `config.path` to load the {{< param "DEFAULT_ENGINE" >}} configuration from a file or directory:

```yaml
extensions:
  basicauth/my_auth:
    client_auth:
      username: <USERNAME>
      password: <PASSWORD>
  alloyengine:
    config:
      path: <ALLOY_CONFIG_PATH>
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

- _`<ALLOY_CONFIG_PATH>`_: The path to your {{< param "DEFAULT_ENGINE" >}} configuration file or directory.
  If you provide a directory, {{< param "PRODUCT_NAME" >}} finds `*.alloy` files in it and loads them as a single configuration source.
  Refer to the [run command reference](../../../reference/cli/run/) for more information.
- _`<USERNAME>`_: Your username. If you're using Grafana Cloud, this is your Grafana Cloud instance ID.
- _`<PASSWORD>`_: Your password. If you're using Grafana Cloud, this is your Grafana Cloud API token.
- _`<URL>`_: The URL to export data to. If you're using Grafana Cloud, this is your Grafana Cloud OTLP endpoint URL.

To provide the {{< param "DEFAULT_ENGINE" >}} configuration inline, use `config.inline.content` instead of `config.path`:

```yaml
extensions:
  alloyengine:
    config:
      inline:
        content: |
          logging {
            level = "info"
          }
```

This example adds the `alloyengine` block in the extension declarations and enables the extension in the `service` block.
You can then run {{< param "PRODUCT_NAME" >}} with the same command you use without the extension:

```shell
alloy otel --config=<CONFIG_FILE> [<FLAGS> ...]
```

This command starts both the {{< param "DEFAULT_ENGINE" >}} and {{< param "OTEL_ENGINE" >}}.
The output of both engines is visible in the logs.
You can access the {{< param "DEFAULT_ENGINE" >}} UI and metrics on port `12345`.

{{< admonition type="warning" >}}
Only one `alloyengine` extension can be active per process.
{{< /admonition >}}

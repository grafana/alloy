---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/engine-extension/
description: Learn how to run a Default Engine pipeline inside the OpenTelemetry Engine with the Alloy Engine extension
menuTitle: Alloy Engine extension
title: Run the Alloy Engine extension
weight: 200
---

# Run the {{% param "PRODUCT_NAME" %}} Engine extension

You can run the {{< param "OTEL_ENGINE" >}} and the {{< param "DEFAULT_ENGINE" >}} in the same process.
Add the `alloyengine` extension to your OpenTelemetry Collector configuration to start a {{< param "DEFAULT_ENGINE" >}} pipeline alongside the {{< param "OTEL_ENGINE" >}} pipeline.
The extension accepts a path to the {{< param "DEFAULT_ENGINE" >}} configuration or an inline {{< param "DEFAULT_ENGINE" >}} configuration.

You can also embed the `alloyengine` extension into any other OpenTelemetry Collector distribution using the OpenTelemetry Collector Builder (OCB).
Refer to [Include `alloyengine` extension in an OCB distribution][OCBDistribution] for instructions on how to do this.

## Before you begin

Make sure you have the following:

- An OpenTelemetry Collector configuration. To create one, refer to [Run the {{< param "OTEL_ENGINE" >}} with the CLI][CLI].
- A {{< param "DEFAULT_ENGINE" >}} configuration file or directory.

## Run both engines

1. Add the `alloyengine` extension to your Collector configuration, and enable it in the `service` block:

   ```yaml
   extensions:
     alloyengine:
       config:
         path: <ALLOY_CONFIG_PATH>
       flags:
         server.http.listen-addr: 0.0.0.0:12345

   service:
     extensions: [alloyengine]
   ```

   Replace _`<ALLOY_CONFIG_PATH>`_ with the path to your {{< param "DEFAULT_ENGINE" >}} configuration file or directory.
   If you provide a directory, {{< param "PRODUCT_NAME" >}} finds the `*.alloy` files in that directory, excluding subdirectories, and loads them as a single configuration source.

   Keep your other `extensions` and `service.extensions` entries, and add `alloyengine` to them.

1. Start both engines:

   ```shell
   alloy otel --config=config.yaml
   ```

   The output of both engines is visible in the logs.

1. Verify that the {{< param "DEFAULT_ENGINE" >}} runs by opening its UI at `localhost:12345`.

## Provide the configuration inline

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

If `config.inline.module_path` isn't defined, the `module_path` {{< param "PRODUCT_NAME" >}} configuration keyword resolves to the process current working directory.

## Pass flags to the {{% param "DEFAULT_ENGINE" %}}

Use the optional `flags` map to pass command-line flags to the {{< param "DEFAULT_ENGINE" >}}.
Write each flag without the leading `--`.
The example sets `server.http.listen-addr` so the {{< param "DEFAULT_ENGINE" >}} UI accepts connections from outside the container.
Refer to the [run command reference][RunCommand] for the available flags.

## Limitations

The {{< param "DEFAULT_ENGINE" >}} runs with some features disabled in extension mode.
The `remotecfg` block isn't supported, and configuration reload isn't available.
Use OpenTelemetry OpAMP for configuration management instead.

If the {{< param "DEFAULT_ENGINE" >}} configuration fails to load, the extension retries at most every 15 seconds.
The Collector still reports the extension as ready, so a broken configuration doesn't stop the {{< param "OTEL_ENGINE" >}}.
Check the `/-/ready` and `/-/healthy` endpoints of the {{< param "DEFAULT_ENGINE" >}} HTTP server for its actual state.

{{< admonition type="warning" >}}
Only one `alloyengine` extension can be active per process.
If you configure more than one, only the first to start succeeds, and the Collector fails to start.
{{< /admonition >}}

[CLI]: ../cli/
[OCBDistribution]: https://github.com/grafana/alloy/blob/main/extension/alloyengine/README.md#include-alloyengine-extension-in-an-ocb-distribution
[RunCommand]: ../../../reference/cli/run/

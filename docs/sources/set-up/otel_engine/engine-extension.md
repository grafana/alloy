---
canonical: https://grafana.com/docs/alloy/latest/set-up/otel_engine/engine-extension/
description: Learn how to run a Default Engine pipeline inside the Alloy OpenTelemetry Engine with the Grafana Alloy Engine extension
menuTitle: Alloy Engine extension
review_date: 2026-09-23
title: Run the Grafana Alloy Engine extension
weight: 200
---

# Run the {{% param "FULL_PRODUCT_NAME" %}} Engine extension

You can run the {{< param "OTEL_ENGINE" >}} and the {{< param "DEFAULT_ENGINE" >}} in the same process.
Add the `alloyengine` extension to your OpenTelemetry Collector configuration to start a {{< param "DEFAULT_ENGINE" >}} pipeline alongside the {{< param "OTEL_ENGINE" >}} pipeline.
The two pipelines run in parallel and can't exchange data with each other.

Set exactly one of `config.path` or `config.inline.content`.
The extension fails to start if you set both or neither.

{{< docs/shared lookup="stability/experimental_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

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
   If you provide a directory, {{< param "PRODUCT_NAME" >}} loads the `*.alloy` files in that directory as a single configuration source and ignores subdirectories.

   The `flags` map passes command-line flags to the {{< param "DEFAULT_ENGINE" >}}.
   Refer to [Pass flags to the {{< param "DEFAULT_ENGINE" >}}][PassFlags] for details.

   Keep your other `extensions` and `service.extensions` entries, and add `alloyengine` to them.

1. Start both engines:

   ```shell
   alloy otel --config=config.yaml
   ```

   The output of both engines is visible in the logs.

1. Verify that the {{< param "DEFAULT_ENGINE" >}} runs by opening its UI at `localhost:12345`.

## Pass flags to the {{% param "DEFAULT_ENGINE" %}}

Use the optional `flags` map to pass command-line flags to the {{< param "DEFAULT_ENGINE" >}}.
Write each flag without the leading `--`.
The example sets `server.http.listen-addr` to `0.0.0.0:12345` so the {{< param "DEFAULT_ENGINE" >}} UI accepts connections from other hosts.
The default address, `127.0.0.1:12345`, accepts connections only from the local host.
Refer to the [run command reference][RunCommand] for the available flags.

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

The `module_path` {{< param "PRODUCT_NAME" >}} configuration keyword resolves to the value of `config.inline.module_path`.
If you don't set `config.inline.module_path`, `module_path` resolves to the current working directory of the Collector process.

## Limitations

The {{< param "DEFAULT_ENGINE" >}} runs with some features disabled in extension mode.
The `remotecfg` block isn't supported, and configuration reload isn't available.
Use OpenTelemetry OpAMP for configuration management instead.

If the {{< param "DEFAULT_ENGINE" >}} configuration fails to load, the extension retries with exponential backoff, starting at 2 seconds and capping at 15 seconds.
The Collector still reports the extension as ready, so a broken configuration doesn't stop the {{< param "OTEL_ENGINE" >}}.
Check the `/-/ready` and `/-/healthy` endpoints of the {{< param "DEFAULT_ENGINE" >}} HTTP server for its actual state.

{{< admonition type="warning" >}}
Only one `alloyengine` extension can be active per process.
If you configure more than one, the first extension to start succeeds, the rest fail, and the Collector doesn't start.
{{< /admonition >}}

## Embed the extension in another distribution

You can embed the `alloyengine` extension into any other OpenTelemetry Collector distribution with the OpenTelemetry Collector Builder (OCB).
Refer to [Include `alloyengine` extension in an OCB distribution][OCBDistribution] for instructions.

[CLI]: ../cli/
[OCBDistribution]: https://github.com/grafana/alloy/blob/main/extension/alloyengine/README.md#include-alloyengine-extension-in-an-ocb-distribution
[PassFlags]: #pass-flags-to-the-default-engine
[RunCommand]: ../../../reference/cli/run/

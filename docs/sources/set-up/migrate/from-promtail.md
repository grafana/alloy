---
canonical: https://grafana.com/docs/alloy/latest/set-up/migrate/from-promtail/
aliases:
  - ../../tasks/migrate/from-promtail/ # /docs/alloy/latest/tasks/migrate/from-promtail/
description: Learn how to migrate from Promtail to Grafana Alloy
menuTitle: Migrate from Promtail
title: Migrate from Promtail to Grafana Alloy
weight: 300
---

# Migrate from Promtail to {{% param "FULL_PRODUCT_NAME" %}}

The built-in {{< param "PRODUCT_NAME" >}} convert command can migrate your [Promtail][] configuration to an {{< param "PRODUCT_NAME" >}} configuration.

This topic describes how to:

* Convert a Promtail configuration to an {{< param "PRODUCT_NAME" >}} configuration.
* Run a Promtail configuration natively using {{< param "PRODUCT_NAME" >}}.

## Components used in this topic

* [local.file_match][]
* [loki.source.file][]
* [loki.write][]

## Before you begin

* You must have an existing Promtail configuration.
* You must be familiar with the concept of [Components][] in {{< param "PRODUCT_NAME" >}}.

## Convert a Promtail configuration

To fully migrate from [Promtail] to {{< param "PRODUCT_NAME" >}}, you must convert your Promtail configuration into an {{< param "PRODUCT_NAME" >}} configuration.
This conversion will enable you to take full advantage of the many additional features available in {{< param "PRODUCT_NAME" >}}.

> In this task, you will use the [convert][] CLI command to output an {{< param "PRODUCT_NAME" >}}
> configuration from a Promtail configuration.

1. Open a terminal window and run the following command.

   ```shell
   alloy convert --source-format=promtail --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
   ```

   Replace the following:
    * _`<INPUT_CONFIG_PATH>`_: The full path to the Promtail configuration.
    * _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.

1. [Run][run alloy] {{< param "PRODUCT_NAME" >}} using the new configuration from _`<OUTPUT_CONFIG_PATH>`_:

### Debugging

1. If the convert command can't convert a Promtail configuration, diagnostic information is sent to `stderr`.
   You can bypass any non-critical issues and output the {{< param "PRODUCT_NAME" >}} configuration using a best-effort conversion by including the `--bypass-errors` flag.

   {{< admonition type="caution" >}}
   If you bypass the errors, the behavior of the converted configuration may not match the original Promtail configuration.
   Make sure you fully test the converted configuration before using it in a production environment.
   {{< /admonition >}}

   ```shell
   alloy convert --source-format=promtail --bypass-errors --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
   ```

   Replace the following:
   * _`<INPUT_CONFIG_PATH>`_: The full path to the Promtail configuration.
   * _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.

1. You can also output a diagnostic report by including the `--report` flag.

   ```shell
   alloy convert --source-format=promtail --report=<OUTPUT_REPORT_PATH> --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
   ```

   Replace the following:

   * _`<INPUT_CONFIG_PATH>`_: The full path to the Promtail configuration.
   * _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.
   * _`<OUTPUT_REPORT_PATH>`_: The output path for the report.

   If you use the [example](#example) Promtail configuration below, the diagnostic report provides the following information:

    ```plaintext
    (Warning) If you have a tracing set up for Promtail, it cannot be migrated to {{< param "PRODUCT_NAME" >}} automatically. Refer to the documentation on how to configure tracing in {{< param "PRODUCT_NAME" >}}.
    (Warning) The metrics from {{< param "PRODUCT_NAME" >}} are different from the metrics emitted by Promtail. If you rely on Promtail's metrics, you must change your configuration, for example, your alerts and dashboards.
    ```

## Run a Promtail configuration

If you’re not ready to completely switch to an {{< param "PRODUCT_NAME" >}} configuration, you can run {{< param "PRODUCT_NAME" >}} using your existing Promtail configuration.
The `--config.format=promtail` flag tells {{< param "PRODUCT_NAME" >}} to convert your Promtail configuration to {{< param "PRODUCT_NAME" >}} and load it directly without saving the new configuration.
This allows you to try {{< param "PRODUCT_NAME" >}} without modifying your existing Promtail configuration infrastructure.

> In this task, you will use the [run][] CLI command to run {{< param "PRODUCT_NAME" >}} using a Promtail configuration.

[Run][run alloy] {{< param "PRODUCT_NAME" >}} and include the command line flag `--config.format=promtail`.
Your configuration file must be a valid Promtail configuration file rather than an {{< param "PRODUCT_NAME" >}} configuration file.

### Debugging

1. You can follow the convert CLI command [debugging][] instructions to generate a diagnostic report.

1. Refer to the {{< param "PRODUCT_NAME" >}}  [Debugging][DebuggingUI] for more information about running {{< param "PRODUCT_NAME" >}}.

1. If your Promtail configuration can't be converted and loaded directly into {{< param "PRODUCT_NAME" >}}, diagnostic information is sent to `stderr`.
   You can bypass any non-critical issues and start {{< param "PRODUCT_NAME" >}} by including the `--config.bypass-conversion-errors` flag in addition to `--config.format=promtail`.

   {{< admonition type="caution" >}}
   If you bypass the errors, the behavior of the converted configuration may not match the original Promtail configuration.
   Do not use this flag in a production environment.
   {{< /admonition >}}

## Example

This example demonstrates converting a Promtail configuration file to an {{< param "PRODUCT_NAME" >}} configuration file.

The following Promtail configuration file provides the input for the conversion.

```yaml
clients:
  - url: http://localhost/loki/api/v1/push
scrape_configs:
  - job_name: example
    static_configs:
      - targets:
          - localhost
        labels:
          __path__: /var/log/*.log
```

The convert command takes the YAML file as input and outputs an [{{< param "PRODUCT_NAME" >}} configuration][configuration] file.

```shell
alloy convert --source-format=promtail --output=<OUTPUT_CONFIG_PATH> <INPUT_CONFIG_PATH>
```

Replace the following:

* _`<INPUT_CONFIG_PATH>`_: The full path to the Promtail configuration.
* _`<OUTPUT_CONFIG_PATH>`_: The full path to output the {{< param "PRODUCT_NAME" >}} configuration.

The new {{< param "PRODUCT_NAME" >}} configuration file looks like this:

```alloy
local.file_match "example" {
	path_targets = [{
		__address__ = "localhost",
		__path__    = "/var/log/*.log",
	}]
}

loki.source.file "example" {
	targets    = local.file_match.example.targets
	forward_to = [loki.write.default.receiver]
}

loki.write "default" {
	endpoint {
		url = "http://localhost/loki/api/v1/push"
	}
	external_labels = {}
}
```

## Limitations

Configuration conversion is done on a best-effort basis. {{< param "PRODUCT_NAME" >}} will issue warnings or errors where the conversion can't be performed.

After the configuration is converted, review the {{< param "PRODUCT_NAME" >}} configuration file created and verify that it's correct before starting to use it in a production environment.

The following list is specific to the convert command and not {{< param "PRODUCT_NAME" >}}:

* Check if you are using any extra command line arguments with Promtail that aren't present in your configuration file. For example, `-max-line-size`.
* Check if you are setting any environment variables, whether [expanded in the configuration file][] itself or consumed directly by Promtail, such as `JAEGER_AGENT_HOST`.
* In {{< param "PRODUCT_NAME" >}}, the positions file is saved at a different location.
  Refer to the [loki.source.file][] documentation for more details.
  Check if you have any existing setup, for example, a Kubernetes Persistent Volume, that you must update to use the new positions file path.
* Metamonitoring metrics exposed by {{< param "PRODUCT_NAME" >}} usually match Promtail metamonitoring metrics but will use a different name.
  Make sure that you use the new metric names, for example, in your alerts and dashboards queries.
* The logs produced by {{< param "PRODUCT_NAME" >}} will differ from those produced by Promtail.
* {{< param "PRODUCT_NAME" >}} exposes the {{< param "PRODUCT_NAME" >}} [UI][], which differs from Promtail's Web UI.

[Promtail]: https://www.grafana.com/docs/loki/<LOKI_VERSION>/clients/promtail/
[debugging]: #debugging
[expanded in the configuration file]: https://www.grafana.com/docs/loki/<LOKI_VERSION>/clients/promtail/configuration/#use-environment-variables-in-the-configuration
[local.file_match]: ../../../reference/components/local/local.file_match/
[loki.source.file]: ../../../reference/components/loki/loki.source.file/
[loki.write]: ../../../reference/components/loki/loki.write/
[Components]: ../../../get-started/components/
[convert]: ../../../reference/cli/convert/
[run]: ../../../reference/cli/run/
[run alloy]: ../../../set-up/run/
[DebuggingUI]: ../../../troubleshoot/debug/
[configuration]: ../../../get-started/configuration-syntax/
[UI]: ../../../troubleshoot/debug/#alloy-ui

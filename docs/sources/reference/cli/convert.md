---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/convert/
description: Learn about the convert command
menuTitle: convert
title: The convert command
weight: 100
---

<span class="badge docs-labels__stage docs-labels__item">Public preview</span>

# The convert command

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `convert` command converts a supported configuration format to the {{< param "PRODUCT_NAME" >}} configuration format.

## Usage

Usage:

```shell
alloy convert [<FLAG> ...] <FILE_NAME>
```

   Replace the following:

   * _`<FLAG>`_: One or more flags that define the input and output of the command.
   * _`<FILE_NAME>`_: The {{< param "PRODUCT_NAME" >}} configuration file.

If the _`<FILE_NAME>`_ argument isn't provided or if the _`<FILE_NAME>`_ argument is equal to `-`, `convert` converts the contents of standard input.
Otherwise, `convert` reads and converts the file from disk specified by the argument.

There are several different flags available for the `convert` command. You can use the `--output` flag to write the contents of the converted configuration to a specified path.
You can use the `--report` flag to generate a diagnostic report.
The `--bypass-errors` flag allows you to bypass any [errors][] generated during the file conversion.

The command fails if the source configuration has syntactically incorrect configuration or can't be converted to an {{< param "PRODUCT_NAME" >}} configuration.

The following flags are supported:

* `--output`, `-o`: The filepath and filename where the output is written.
* `--report`, `-r`: The filepath and filename where the report is written.
* `--source-format`, `-f`: Required. The format of the source file. Supported formats: [otelcol], [prometheus], [promtail], [static].
* `--bypass-errors`, `-b`: Enable bypassing errors when converting.
* `--extra-args`, `e`: Extra arguments from the original format used by the converter.

### Defaults

{{< param "PRODUCT_NAME" >}} defaults are managed as follows:
* If a provided source configuration value matches an {{< param "PRODUCT_NAME" >}} default value, the property is left off the output.
* If a non-provided source configuration value default matches an {{< param "PRODUCT_NAME" >}} default value, the property is left off the output.
* If a non-provided source configuration value default doesn't match an {{< param "PRODUCT_NAME" >}} default value, the default value is included in the output.

### Errors

Errors are defined as non-critical issues identified during the conversion where an output can still be generated.
These can be bypassed using the `--bypass-errors` flag.

### OpenTelemetry Collector

You can use the `--source-format=otelcol` to convert the source configuration from an [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/configuration/) to a {{< param "PRODUCT_NAME" >}} configuration.

Many OpenTelemetry Collector components are supported.
Review the `otelcol.*` component information in the [Component Reference][] for more information about `otelcol` components that you can convert.
If a source configuration has unsupported features, you will receive [errors] when you convert it to an {{< param "PRODUCT_NAME" >}} configuration.
The converter raises warnings for configuration options that may require your attention.

Refer to [Migrate from OpenTelemetry Collector to {{< param "PRODUCT_NAME" >}}][migrate otelcol] for a detailed migration guide.

### Prometheus

Using the `--source-format=prometheus` will convert the source configuration from [Prometheus v2.45][] to an {{< param "PRODUCT_NAME" >}} configuration.

This includes Prometheus features such as [scrape_config][], [relabel_config][], [metric_relabel_configs][], [remote_write][], and many supported *_sd_configs.
Unsupported features in a source configuration result in [errors][].

Refer to [Migrate from Prometheus to {{< param "PRODUCT_NAME" >}}][migrate prometheus] for a detailed migration guide.

### Promtail

Using the `--source-format=promtail` will convert the source configuration from [Promtail v2.8.x][] to an {{< param "PRODUCT_NAME" >}} configuration.

Nearly all [Promtail features][] are supported and can be converted to {{< param "PRODUCT_NAME" >}} configuration.

If you have unsupported features in a source configuration, you will receive [errors][] when you convert to an {{< param "PRODUCT_NAME" >}} configuration.
The converter will also raise warnings for configuration options that may require your attention.

Refer to [Migrate from Promtail to {{< param "PRODUCT_NAME" >}}][migrate promtail] for a detailed migration guide.

### Static

Using the `--source-format=static` will convert the source configuration from a [Grafana Agent Static][] configuration to an {{< param "PRODUCT_NAME" >}} configuration.

Include `--extra-args` for passing additional command line flags from the original format.
For example, `--extra-args="-enable-features=integrations-next"` converts a Grafana Agent Static [integrations-next][] configuration to an {{< param "PRODUCT_NAME" >}} configuration.
You can also expand environment variables with `--extra-args="-config.expand-env"`.
You can combine multiple command line flags with a space between each flag, for example `--extra-args="-enable-features=integrations-next -config.expand-env"`.

If you have unsupported features in a Grafana Agent Static mode source configuration, you will receive [errors][] when you convert to an {{< param "PRODUCT_NAME" >}} configuration.
The converter also raises warnings for configuration options that may require your attention.

Refer to [Migrate from Grafana Agent Static to {{< param "PRODUCT_NAME" >}}][migrate static] for a detailed migration guide.

[otelcol]: #opentelemetry-collector
[prometheus]: #prometheus
[promtail]: #promtail
[static]: #static
[errors]: #errors
[scrape_config]: https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#scrape_config
[relabel_config]: https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#relabel_config
[metric_relabel_configs]: https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#metric_relabel_configs
[remote_write]: https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#remote_write
[migrate otelcol]: ../../../set-up/migrate/from-otelcol/
[migrate prometheus]: ../../../set-up/migrate/from-prometheus/
[Promtail v2.8.x]: https://grafana.com/docs/loki/v2.8.x/clients/promtail/
[Prometheus v2.45]: https://prometheus.io/docs/prometheus/2.45/configuration/configuration/
[Promtail features]: https://grafana.com/docs/loki/v2.8.x/clients/promtail/configuration/
[migrate promtail]: ../../../set-up/migrate/from-promtail/
[Grafana Agent Static]: https://grafana.com/docs/agent/latest/static/
[integrations-next]: https://grafana.com/docs/agent/latest/static/configuration/integrations/integrations-next/
[migrate static]: ../../../set-up/migrate/from-static/

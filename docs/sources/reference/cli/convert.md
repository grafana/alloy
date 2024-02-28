---
aliases:
- /docs/grafana-cloud/agent/flow/reference/cli/convert/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/cli/convert/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/cli/convert/
- /docs/grafana-cloud/send-data/agent/flow/reference/cli/convert/
canonical: https://grafana.com/docs/agent/latest/flow/reference/cli/convert/
description: Learn about the convert command
labels:
  stage: beta
menuTitle: convert
title: The convert command
weight: 100
---

# The convert command

The `convert` command converts a supported configuration format to {{< param "PRODUCT_NAME" >}} River format.

## Usage

Usage:

* `AGENT_MODE=flow grafana-agent convert [<FLAG> ...] <FILE_NAME>`
* `grafana-agent-flow convert [<FLAG> ...] <FILE_NAME>`

   Replace the following:

   * _`<FLAG>`_: One or more flags that define the input and output of the command.
   * _`<FILE_NAME>`_: The {{< param "PRODUCT_ROOT_NAME" >}} configuration file.

If the `FILE_NAME` argument isn't provided or if the `FILE_NAME` argument is
equal to `-`, `convert` converts the contents of standard input. Otherwise,
`convert` reads and converts the file from disk specified by the argument.

There are several different flags available for the `convert` command. You can use the `--output` flag to write the contents of the converted configuration to a specified path. You can use the `--report` flag to generate a diagnostic report. The `--bypass-errors` flag allows you to bypass any [errors] generated during the file conversion.

The command fails if the source configuration has syntactically incorrect
configuration or can't be converted to {{< param "PRODUCT_NAME" >}} River format.

The following flags are supported:

* `--output`, `-o`: The filepath and filename where the output is written.

* `--report`, `-r`: The filepath and filename where the report is written.

* `--source-format`, `-f`: Required. The format of the source file. Supported formats: [prometheus], [promtail], [static].

* `--bypass-errors`, `-b`: Enable bypassing errors when converting.

* `--extra-args`, `e`: Extra arguments from the original format used by the converter.

[prometheus]: #prometheus
[promtail]: #promtail
[static]: #static
[errors]: #errors

### Defaults

{{< param "PRODUCT_NAME" >}} defaults are managed as follows:
* If a provided source configuration value matches a {{< param "PRODUCT_NAME" >}} default value, the property is left off the output.
* If a non-provided source configuration value default matches a {{< param "PRODUCT_NAME" >}} default value, the property is left off the output.
* If a non-provided source configuration value default doesn't match a {{< param "PRODUCT_NAME" >}} default value, the default value is included in the output.

### Errors

Errors are defined as non-critical issues identified during the conversion
where an output can still be generated. These can be bypassed using the
`--bypass-errors` flag.

### Prometheus

Using the `--source-format=prometheus` will convert the source configuration from
[Prometheus v2.45](https://prometheus.io/docs/prometheus/2.45/configuration/configuration/)
to {{< param "PRODUCT_NAME" >}} configuration.

This includes Prometheus features such as
[scrape_config](https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#scrape_config),
[relabel_config](https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#relabel_config),
[metric_relabel_configs](https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#metric_relabel_configs),
[remote_write](https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#remote_write),
and many supported *_sd_configs. Unsupported features in a source configuration result
in [errors].

Refer to [Migrate from Prometheus to {{< param "PRODUCT_NAME" >}}]({{< relref "../../tasks/migrate/from-prometheus/" >}}) for a detailed migration guide.

### Promtail

Using the `--source-format=promtail` will convert the source configuration from
[Promtail v2.8.x](/docs/loki/v2.8.x/clients/promtail/)
to {{< param "PRODUCT_NAME" >}} configuration.

Nearly all [Promtail features](/docs/loki/v2.8.x/clients/promtail/configuration/)
are supported and can be converted to {{< param "PRODUCT_NAME" >}} configuration.

If you have unsupported features in a source configuration, you will receive [errors] when you convert to a flow configuration. The converter will
also raise warnings for configuration options that may require your attention.

Refer to [Migrate from Promtail to {{< param "PRODUCT_NAME" >}}]({{< relref "../../tasks/migrate/from-promtail/" >}}) for a detailed migration guide.

### Static

Using the `--source-format=static` will convert the source configuration from a
[Grafana Agent Static]({{< relref "../../../static" >}}) configuration to a {{< param "PRODUCT_NAME" >}} configuration.

Include `--extra-args` for passing additional command line flags from the original format.
For example, `--extra-args="-enable-features=integrations-next"` will convert a Grafana Agent Static
[integrations-next]({{< relref "../../../static/configuration/integrations/integrations-next/" >}})
configuration to a {{< param "PRODUCT_NAME" >}} configuration. You can also
expand environment variables with `--extra-args="-config.expand-env"`. You can combine multiple command line
flags with a space between each flag, for example `--extra-args="-enable-features=integrations-next -config.expand-env"`.

If you have unsupported features in a Static mode source configuration, you will receive [errors][] when you convert to a Flow mode configuration. The converter will
also raise warnings for configuration options that may require your attention.

Refer to [Migrate from Grafana Agent Static to {{< param "PRODUCT_NAME" >}}]({{< relref "../../tasks/migrate/from-static/" >}}) for a detailed migration guide.
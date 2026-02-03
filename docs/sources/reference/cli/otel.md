---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/otel/
description: Learn about the otel command
labels:
  stage: experimental
  products:
    - oss
title: otel
_build:
  list: false
noindex: true
weight: 350
---

# `otel`

> **EXPERIMENTAL**: This is an [experimental][] feature.
> Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.

[experimental]: https://grafana.com/docs/release-life-cycle/

The `otel` command runs {{< param "PRODUCT_NAME" >}} with the {{< param "OTEL_ENGINE" >}}. This command accepts OpenTelemetry Collector YAML configuration files.

The {{< param "OTEL_ENGINE" >}} includes receivers, processors, exporters, extensions, and connectors from the OpenTelemetry Collector core and contrib repositories.
This includes components for OTLP, Prometheus, Kafka, Zipkin, and other popular integrations.

As with the `run` command, this runs in the foreground until an interrupt is received.

## Usage

```shell
alloy otel --config=<CONFIG_FILE> [<FLAGS> ...] 
```

Replace the following:

- _`<CONFIG_FILE>`_: Path to an OpenTelemetry Collector configuration file.
- _`<FLAGS>`_: One or more flags that [configure the OpenTelemetry Collector](https://opentelemetry.io/docs/collector/configuration/).
  These flags are the same as upstream.
  Run `alloy otel --help` to show the complete list of supported flags.

## Configuration

The `otel` command accepts standard OpenTelemetry Collector YAML configuration files.
The configuration file defines receivers, processors, exporters, and other components that make up your telemetry pipeline.

For information about configuration options, refer to the [OpenTelemetry Collector documentation](https://opentelemetry.io/docs/collector/configuration/).

### Run the {{% param "DEFAULT_ENGINE" %}} in parallel

The {{< param "OTEL_ENGINE" >}} includes the option to run pipelines with the {{< param "DEFAULT_ENGINE" >}} alongside the {{< param "OTEL_ENGINE" >}} using the built-in {{< param "PRODUCT_NAME" >}} Engine extension.

This runs a {{< param "DEFAULT_ENGINE" >}} pipeline _in parallel_ to the {{< param "OTEL_ENGINE" >}} pipeline.
The two pipelines can't natively interact.

### Examples

Refer to [Get started](../../../opentelemetry/get-started/) for examples that show you how to run the {{< param "OTEL_ENGINE" >}} and {{< param "PRODUCT_NAME" >}} Engine extension.

## Related documentation

- [OpenTelemetry Collector documentation](https://opentelemetry.io/docs/collector/): Official OpenTelemetry Collector documentation.

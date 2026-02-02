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

The `otel` command runs Grafana Alloy using the OpenTelemetry (OTel) Collector engine. This command accepts OpenTelemetry Collector YAML configuration files.

The Alloy OTel distribution includes receivers, processors, exporters, extensions, and connectors from the OpenTelemetry Collector core and contrib repositories. This includes components for OTLP, Prometheus, Kafka, Zipkin, and other popular integrations.

As with the `run` command, this runs in the foreground until an interrupt is received.

## Usage

```shell
alloy otel --config=<CONFIG_FILE> [<FLAGS> ...] 
```

Replace the following:

* _`<CONFIG_FILE>`_: Path to an OpenTelemetry Collector configuration file.
* _`<FLAGS>`_: One or more flags that configure the OpenTelemetry Collector. These flags are the same as upstream. Run `alloy otel --help` to show the complete list of supported flags.

## Configuration

The `otel` command accepts standard OpenTelemetry Collector YAML configuration files. The configuration file defines receivers, processors, exporters, and other components that make up your telemetry pipeline.

For information about configuration options, refer to the [OpenTelemetry Collector documentation](https://opentelemetry.io/docs/collector/configuration/).

### Optionally Running the Default Engine

The Alloy Collector Distro includes the option to run pipelines using the Default Engine alongside the OTel Engine using the built-in Alloy Engine extension.

This will run a Default Engine pipeline _in parallel_ to the OTel Engine pipeline - the two pipelines cannot natively interact.

### Examples

Refer to [Get Started](../../../open-telemetry/get-started/) for examples that show you how to run the OTel Engine and Alloy Engine Extension.

## Related documentation
* [OpenTelemetry Collector documentation](https://opentelemetry.io/docs/collector/): Official OpenTelemetry Collector documentation.



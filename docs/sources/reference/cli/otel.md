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
* _`<FLAGS>`_: One or more flags that configure the OpenTelemetry Collector.

## Configuration

The `otel` command accepts standard OpenTelemetry Collector YAML configuration files. The configuration file defines receivers, processors, exporters, and other components that make up your telemetry pipeline.

For information about configuration options, refer to the [OpenTelemetry Collector documentation](https://opentelemetry.io/docs/collector/configuration/).

### Optionally Running the Default Engine

The Alloy Collector Distro includes the option to run pipelines using the Default Engine alongside the OTel Engine using the built in Alloy Engine extension. More information on how to run the extension can be found [here](https://github.com/grafana/alloy/blob/main/extension/alloyengine/README.md)

This will run a Default Engine pipeline _in parallel_ to the OTel Engine pipeline - the two pipelines cannot natively interact.

### Available Components

The included components are based off the upstream core distribution, in order to ensure that full end-to-end pipelines are accessible for most use cases. In addition to upstream components, we also integrate some of our own components that enable functionality to work well within the Alloy ecosystem.

To view the full list of components and their versioning, please refer to the [OCB manifest](https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml)

## Examples

### Running with OTel Engine only

This example runs the OTel Engine without the Alloy Engine extension:

```shell
alloy otel --config=config.yaml
```

Example `config.yaml`:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:

exporters:
  debug:

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
```

### Running with OTel Engine and Alloy Engine extension

This example runs both the OTel Engine and the Alloy Engine extension in parallel:

```shell
alloy otel --config=config.yaml
```

Example `config.yaml`:

```yaml
extensions:
  alloyengine:
    config:
      file: path/to/alloy-config.alloy
    flags:
      server.http.listen-addr: 0.0.0.0:12345
      stability.level: experimental

receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:

exporters:
  debug:

service:
  extensions: [alloyengine]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
```

## Related documentation
* [OpenTelemetry Collector documentation](https://opentelemetry.io/docs/collector/): Official OpenTelemetry Collector documentation.



---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/otel/
description: Learn about the otel command
labels:
  stage: experimental
  products:
    - oss
title: otel
weight: 350
---

# `otel`

The `otel` command runs the Alloy Open Telemetry (OTel) Engine, a collector distribution that embeds both upstream and internal components. The Alloy collector distribution is a pre-packaged, supported set of components designed for ease of use and integration with backend observability platforms.

As with the `run` command, this runs in the foreground until an interrupt is received.

{{< admonition type="warning" >}}
Please note that this is an *experimental* feature and can therefore be subject to breaking changes or removal in future releases.
{{< /admonition >}}

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

The Alloy Collector Distro includes the option to run pipelines using the Default Engine alongside the OTel Engine using the built in `alloyengine` extension. More information on how to run the extension can be found [here](https://github.com/grafana/alloy/blob/main/extension/alloyengine/README.md)

This will run a Default Engine pipeline _in parallel_ to the OTel Engine pipeline - the two pipelines cannot natively interact.

### Available Components

The included components are based off the upstream core distribution, in order to ensure that full end-to-end pipelines are accessible for most use cases. In addition to upstream components, we also integrate some of our own components that enable functionality to work well within the Alloy ecosystem.

To view the full list of components and their versioning, please refer to the [OCB manifest](https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml)

## Examples

### Running with YAML configuration and Alloy Engine extension

```shell
alloy otel --config=config.yaml
```

Example `config.yaml`:

```yaml
extensions:
  alloyengine:
    config:
      file: ./alloy-config.alloy
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

Removing the `alloyengine` portion of the config will run the OTel Engine alone, without any Alloy Engine pipeline running alongside.

## Related documentation
* [OpenTelemetry Collector documentation](https://opentelemetry.io/docs/collector/): Official OpenTelemetry Collector documentation.



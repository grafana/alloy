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

The following table lists all components available in the Alloy OTel Engine:

| Component Name | Stability Level |
|----------------|-----------------|
| **Extensions** | |
| [`zpages`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/extension/zpagesextension) | Generally Available |
| [`healthcheck`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/healthcheckextension) | Generally Available |
| [`pprof`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/pprofextension) | Generally Available |
| [`alloyengine`](https://github.com/grafana/alloy/tree/main/extension/alloyengine) | Experimental |
| **Receivers** | |
| [`otlp`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/receiver/otlpreceiver) | Generally Available |
| **Processors** | |
| [`batch`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor) | Generally Available |
| [`memory_limiter`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiterprocessor) | Generally Available |
| [`attributes`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/attributesprocessor) | Generally Available |
| [`resource`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourceprocessor) | Generally Available |
| [`span`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/spanprocessor) | Generally Available |
| [`probabilistic_sampler`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/probabilisticsamplerprocessor) | Generally Available |
| [`filter`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor) | Generally Available |
| **Exporters** | |
| [`debug`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/debugexporter) | Generally Available |
| [`nop`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/nopexporter) | Generally Available |
| [`otlp`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlpexporter) | Generally Available |
| [`otlphttp`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter) | Generally Available |
| [`file`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/fileexporter) | Generally Available |
| [`kafka`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/kafkaexporter) | Generally Available |
| [`prometheus`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/prometheusexporter) | Generally Available |
| [`prometheusremotewrite`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/prometheusremotewriteexporter) | Generally Available |
| [`zipkin`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/zipkinexporter) | Generally Available |
| **Connectors** | |
| [`forward`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/connector/forwardconnector) | Generally Available |
| **Providers** | |
| [`env`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/confmap/provider/envprovider) | Generally Available |
| [`file`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/confmap/provider/fileprovider) | Generally Available |
| [`http`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/confmap/provider/httpprovider) | Generally Available |
| [`https`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/confmap/provider/httpsprovider) | Generally Available |
| [`yaml`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/confmap/provider/yamlprovider) | Generally Available |

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



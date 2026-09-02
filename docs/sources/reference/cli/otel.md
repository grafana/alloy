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

> **EXPERIMENTAL**: This is an [experimental][] feature.
> Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.

[experimental]: https://grafana.com/docs/release-life-cycle/

The `otel` command runs {{< param "PRODUCT_NAME" >}} with the {{< param "OTEL_ENGINE" >}}.
This command accepts OpenTelemetry Collector YAML configuration files.

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

In addition to the upstream flags, the `otel` command accepts:

- `--disable-reporting`: Disable [anonymous usage statistics reporting](../../../data-collection/) to Grafana.

## Configuration

The `otel` command accepts standard OpenTelemetry Collector YAML configuration files.
The configuration file defines receivers, processors, exporters, and other components that make up your telemetry pipeline.

### Run the {{% param "DEFAULT_ENGINE" %}} in parallel

The {{< param "OTEL_ENGINE" >}} includes the option to run pipelines with the {{< param "DEFAULT_ENGINE" >}} alongside the {{< param "OTEL_ENGINE" >}}.
Use the built-in {{< param "PRODUCT_NAME" >}} Engine extension to enable this.

This runs a {{< param "DEFAULT_ENGINE" >}} pipeline _in parallel_ to the {{< param "OTEL_ENGINE" >}} pipeline.
The two pipelines can't natively interact.

### Examples

Refer to [The OpenTelemetry Engine](../../../set-up/otel_engine/) for examples that show you how to run the {{< param "OTEL_ENGINE" >}} and {{< param "PRODUCT_NAME" >}} Engine extension.

## Included components

The {{< param "OTEL_ENGINE" >}} includes:

- Standard components from the OpenTelemetry Collector Core repository
- Selected components from the OpenTelemetry Collector Contrib repositories
- The `alloyengine` extension

{{< param "PRODUCT_NAME" >}} {{< param "ALLOY_RELEASE" >}} bundles OpenTelemetry Collector components from version {{< param "OTEL_VERSION" >}}.
You can find more information about the bundled version in both the [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}) and [OpenTelemetry Collector Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}) repositories.

The {{< param "OTEL_ENGINE" >}} bundles a curated subset of these components rather than mirroring all of the OpenTelemetry Collector Contrib repository.
The bundled component set is based on demand, maturity, maintenance cost, dependency footprint, security risk, and license compatibility.
Because the {{< param "OTEL_ENGINE" >}} is the Grafana distribution of the OpenTelemetry Collector, it always includes the components you need for a smooth integration with Grafana products.
Refer to the [OpenTelemetry component contributor guide](https://github.com/grafana/alloy/blob/main/docs/developer/add-otel-component.md#inclusion-criteria) for information about the bundle criteria.

The following sections list all included components:

{{< collapse title="Extensions" >}}

- [alloyengine](https://github.com/grafana/alloy/tree/main/extension/alloyengine)
- [basicauth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/basicauthextension/README.md)
- [bearertokenauth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/bearertokenauthextension/README.md)
- [headerssetter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/headerssetterextension/README.md)
- [healthcheck](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/healthcheckextension/README.md)
- [jaegerremotesampling](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/jaegerremotesampling/README.md)
- [k8sleaderelector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/k8sleaderelector/README.md)
- [oauth2clientauth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/oauth2clientauthextension/README.md)
- [opamp](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/opampextension/README.md)
- [pprof](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/pprofextension/README.md)
- [sigv4auth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/sigv4authextension/README.md)
- [filestorage](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/storage/filestorage/README.md)
- [zpages](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/extension/zpagesextension/README.md)

{{< /collapse >}}

{{< collapse title="Configuration Providers" >}}

- [env](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/confmap/provider/envprovider/README.md)
- [file](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/confmap/provider/fileprovider/README.md)
- [http](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/confmap/provider/httpprovider/README.md)
- [https](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/confmap/provider/httpsprovider/README.md)
- [yaml](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/confmap/provider/yamlprovider/README.md)

{{< /collapse >}}

{{< collapse title="Receivers" >}}

- [awscloudwatch](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/awscloudwatchreceiver/README.md)
- [awsecscontainermetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/awsecscontainermetricsreceiver/README.md)
- [awss3](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/awss3receiver/README.md)
- [cloudflare](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/cloudflarereceiver/README.md)
- [datadog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/datadogreceiver/README.md)
- [faro](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/faroreceiver/README.md)
- [filelog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/filelogreceiver/README.md)
- [filestats](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/filestatsreceiver/README.md)
- [fluentforward](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/fluentforwardreceiver/README.md)
- [hostmetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/hostmetricsreceiver/README.md)
- [influxdb](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/influxdbreceiver/README.md)
- [jaeger](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/jaegerreceiver/README.md)
- [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/k8sclusterreceiver/README.md)
- [k8sobjectsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/k8sobjectsreceiver/README.md)
- [kafka](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/kafkareceiver/README.md)
- [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/kubeletstatsreceiver/README.md)
- [nginx](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/nginxreceiver/README.md)
- [prometheus](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/prometheusreceiver/README.md)
- [prometheusremotewrite](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/prometheusremotewritereceiver/README.md)
- [solace](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/solacereceiver/README.md)
- [splunkhec](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/splunkhecreceiver/README.md)
- [syslog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/syslogreceiver/README.md)
- [tcplog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/tcplogreceiver/README.md)
- [vcenter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/vcenterreceiver/README.md)
- [zipkin](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/zipkinreceiver/README.md)
- [nop](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/receiver/nopreceiver/README.md)
- [otlp](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/receiver/otlpreceiver/README.md)
{{< /collapse >}}

{{< collapse title="Connectors" >}}
- [count](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/countconnector/README.md)
- [grafanacloud](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/grafanacloudconnector/README.md)
- [servicegraph](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/servicegraphconnector/README.md)
- [signaltometrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/signaltometricsconnector/README.md)
- [spanmetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/spanmetricsconnector/README.md)
- [forward](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/connector/forwardconnector/README.md)

{{< /collapse >}}

{{< collapse title="Processors" >}}

- [attributes](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/attributesprocessor/README.md)
- [cumulativetodelta](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/cumulativetodeltaprocessor/README.md)
- [deltatocumulative](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/deltatocumulativeprocessor/README.md)
- [filter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/filterprocessor/README.md)
- [groupbyattrs](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/groupbyattrsprocessor/README.md)
- [interval](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/intervalprocessor/README.md)
- [k8sattributes](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/k8sattributesprocessor/README.md)
- [metricstarttime](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/metricstarttimeprocessor/README.md)
- [probabilisticsampler](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/probabilisticsamplerprocessor/README.md)
- [redaction](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/redactionprocessor/README.md)
- [resource](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/resourceprocessor/README.md)
- [resourcedetection](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/resourcedetectionprocessor/README.md)
- [span](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/spanprocessor/README.md)
- [tailsampling](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/tailsamplingprocessor/README.md)
- [transform](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor/README.md)
- [batch](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/processor/batchprocessor/README.md)
- [memorylimiter](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/processor/memorylimiterprocessor/README.md)

{{< /collapse >}}

{{< collapse title="Exporters" >}}

- [awss3](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/awss3exporter/README.md)
- [faro](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/faroexporter/README.md)
- [file](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/fileexporter/README.md)
- [kafka](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/kafkaexporter/README.md)
- [loadbalancing](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/loadbalancingexporter/README.md)
- [prometheus](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/prometheusexporter/README.md)
- [prometheusremotewrite](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/prometheusremotewriteexporter/README.md)
- [syslog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/syslogexporter/README.md)
- [zipkin](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/zipkinexporter/README.md)
- [debug](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/exporter/debugexporter/README.md)
- [nop](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/exporter/nopexporter/README.md)
- [otlp](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/exporter/otlpexporter/README.md)
- [otlphttp](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/exporter/otlphttpexporter/README.md)

{{< /collapse >}}

To view the full list of components and their versions, refer to the [OpenTelemetry Collector Builder manifest](https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml).

## Component lifecycle

Bundled components follow the upstream OpenTelemetry Collector [component lifecycle](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md).
When a component becomes deprecated or unmaintained upstream, {{< param "PRODUCT_NAME" >}} deprecates it and eventually removes it from the {{< param "OTEL_ENGINE" >}}.

{{< admonition type="note" >}}
{{< param "PRODUCT_NAME" >}} provides notice before it removes a component. You can keep using a removed component through a [custom OpenTelemetry Collector Builder (OCB) build](../../../set-up/otel_engine/#custom-builds-with-the-opentelemetry-collector-builder-ocb).
{{< /admonition >}}

## Related documentation

- [OpenTelemetry in {{< param "PRODUCT_NAME" >}}](../../../introduction/otel_alloy/): Learn when to use the {{< param "OTEL_ENGINE" >}} or {{< param "DEFAULT_ENGINE" >}}.
- [OpenTelemetry Collector documentation](https://opentelemetry.io/docs/collector/): Official OpenTelemetry Collector documentation.

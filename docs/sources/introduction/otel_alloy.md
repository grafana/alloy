---
canonical: https://grafana.com/docs/alloy/latest/introduction/otel_alloy/
aliases:
  - ../opentelemetry/ # /docs/alloy/latest/opentelemetry/
description: Learn about the OpenTelemetry Engine, a bundled OpenTelemetry Collector distribution embedded within Grafana Alloy
menuTitle: OpenTelemetry in Alloy
title: OpenTelemetry in Alloy
weight: 230
---

# OpenTelemetry in {{% param "PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} combines the Prometheus-native, production-grade collection features of {{< param "PRODUCT_NAME" >}} with the broad ecosystem and standards of OpenTelemetry.
The {{< param "FULL_OTEL_ENGINE" >}} is a bundled OpenTelemetry Collector distribution embedded within {{< param "PRODUCT_NAME" >}}.
It lets you run {{< param "PRODUCT_NAME" >}} as a fully compatible OTel Collector while retaining access to all {{< param "PRODUCT_NAME" >}} features and integrations.

{{< docs/shared lookup="stability/experimental_otel.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Terminology

{{< param "PRODUCT_NAME" >}} supports two runtime engines and an extension:

- **{{< param "DEFAULT_ENGINE" >}}**: The default {{< param "PRODUCT_NAME" >}} runtime and [configuration syntax](../get-started/syntax/).
  This remains the default, stable experience with [backward compatibility](../introduction/backward-compatibility/) guarantees for {{< param "PRODUCT_NAME" >}} users.

- **{{< param "OTEL_ENGINE" >}}**: The standard OpenTelemetry Collector runtime embedded within {{< param "PRODUCT_NAME" >}}.
  It uses [upstream collector YAML configuration](https://opentelemetry.io/docs/collector/configuration/) for pipelines and components.

- **{{< param "PRODUCT_NAME" >}} Engine extension**: An OpenTelemetry Collector extension that lets you run both the {{< param "DEFAULT_ENGINE" >}} and the {{< param "OTEL_ENGINE" >}} in parallel.

## Included components

The {{< param "OTEL_ENGINE" >}} bundle includes:

- Standard components from the OpenTelemetry Collector core
- A curated selection of components from contributor repositories
- The `alloyengine` extension

{{< param "PRODUCT_NAME" >}} {{< param ALLOY_RELEASE >}} bundles versions {{< param "OTEL_VERSION" >}} of OpenTelemetry Collector components.
You can find more information about the bundled version in both the [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}) and [OpenTelemetry Collector Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}) repositories.

The following sections list all included components:

{{< collapse title="Extensions" >}}

- [alloyengine](https://github.com/grafana/alloy/tree/main/extension/alloyengine)
- [basicauth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/basicauthextension/README.md)
- [bearertokenauth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/bearertokenauthextension/README.md)
- [headerssetter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/headerssetterextension/README.md)
- [healthcheck](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/healthcheckextension/README.md)
- [jaegerremotesampling](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/jaegerremotesampling/README.md)
- [oauth2clientauth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/oauth2clientauthextension/README.md)
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
- [googlecloudpubsub](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/googlecloudpubsubreceiver/README.md)
- [hostmetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/hostmetricsreceiver/README.md)
- [influxdb](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/influxdbreceiver/README.md)
- [jaeger](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/jaegerreceiver/README.md)
- [k8sobjectsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/k8sobjectsreceiver/README.md)
- [kafka](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/kafkareceiver/README.md)
- [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/kubeletstatsreceiver/README.md)
- [prometheus](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/prometheusreceiver/README.md)
- [prometheusremotewrite](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/prometheusremotewritereceiver/README.md)
- [solace](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/solacereceiver/README.md)
- [splunkhec](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/splunkhecreceiver/README.md)
- [syslog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/syslogreceiver/README.md)
- [tcplog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/tcplogreceiver/README.md)
- [vcenter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/vcenterreceiver/README.md)
- [zipkin](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/zipkinreceiver/README.md)
- [otlp](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/receiver/otlpreceiver/README.md)
{{< /collapse >}}

{{< collapse title="Connectors" >}}
- [count](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/countconnector/README.md)
- [grafanacloud](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/grafanacloudconnector/README.md)
- [servicegraph](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/servicegraphconnector/README.md)
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
- [googlecloud](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/googlecloudexporter/README.md)
- [googlecloudpubsub](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/googlecloudpubsubexporter/README.md)
- [kafka](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/kafkaexporter/README.md)
- [loadbalancing](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/loadbalancingexporter/README.md)
- [prometheus](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/prometheusexporter/README.md)
- [prometheusremotewrite](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/prometheusremotewriteexporter/README.md)
- [splunkhec](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/splunkhecexporter/README.md)
- [syslog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/syslogexporter/README.md)
- [zipkin](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/zipkinexporter/README.md)
- [debug](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/exporter/debugexporter/README.md)
- [nop](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/exporter/nopexporter/README.md)
- [otlp](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/exporter/otlpexporter/README.md)
- [otlphttp](https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/exporter/otlphttpexporter/README.md)

{{< /collapse >}}

To view the full list of components and their versions, refer to the [OpenTelemetry Collector Builder manifest](https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml).

## Next steps

- Refer to [The {{< param "OTEL_ENGINE" >}}](../../set-up/otel_engine/) for information about how to run the {{< param "OTEL_ENGINE" >}}.
- Refer to the [OTel CLI reference](../../reference/cli/otel/) for more information about the OTel CLI.

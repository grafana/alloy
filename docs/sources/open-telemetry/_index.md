---
canonical: https://grafana.com/docs/alloy/latest/opentelemetry/
description: Grafana Alloy is a flexible, high performance, vendor-neutral distribution of the OTel Collector
menuTitle: Open Telemetry
title: Open Telemetry With Grafana Alloy
_build:
  list: false
noindex: true
weight: 10
---

## Overview
Grafana Alloy combines Alloy’s Prometheus-native, production-grade collection features with the broad ecosystem and standards of OpenTelemetry. The Alloy Open Telemtry (OTel) Engine is a new, bundled OpenTelemetry Collector distribution embedded within Alloy that lets you run Alloy as a fully-compatible OTel Collector while retaining access to existing features and integrations.

Please note that this is an **experimental** feature. You may see changes as we refine it, and we welcome feedback to help shape it toward general availability with wider support of our cloud features.

## A Note on Feature Flags
Although the OTel Engine is an experimental feature, it is not currently hidden behind stability flags, this is due to the engine not being a component but a sub command that is included in the Alloy build. Please ensure that you understand the risks involved in utilizing an experimental component if you choose to use this in your production environments.

## Terminology
To avoid confusion, we use precise terminology for the two runtime models:

* **Default Engine:** The existing Alloy runtime and configuration syntax. This remains the non-breaking, primary experience for existing Alloy users.

* **OTel Engine:** The new runtime that runs our OpenTelemetry distribution inside Alloy, using standard [upstream collector YAML configuration](https://opentelemetry.io/docs/collector/configuration/) for pipelines and components.

## Included Components
The OTel Engine bundle includes components from two places:

Core upstream OpenTelemetry Collector distribution — the standard components from the upstream core collector distribution that are required to build OTel-native pipelines.

Selected Default Engine components — the Default Engine wraps native components and extensions that remain available as part of the OTel Engine

A complete list can be found below:

{{< collapse title="Extensions" >}}

* [alloyengine](https://github.com/grafana/alloy/tree/main/extension/alloyengine)
* [basicauth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/basicauthextension)
* [bearertokenauth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/bearertokenauthextension)
* [headerssetter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/headerssetterextension)
* [healthcheck](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/healthcheckextension)
* [jaegerremotesampling](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/jaegerremotesampling)
* [oauth2clientauth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/oauth2clientauthextension)
* [pprof](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/pprofextension)
* [sigv4auth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/sigv4authextension)
* [filestorage](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/storage/filestorage)
* [zpages](https://github.com/open-telemetry/opentelemetry-collector/tree/main/extension/zpagesextension)

{{< /collapse >}}

{{< collapse title="Config Providers" >}}

* [env](https://github.com/open-telemetry/opentelemetry-collector/tree/main/confmap/provider/envprovider)
* [file](https://github.com/open-telemetry/opentelemetry-collector/tree/main/confmap/provider/fileprovider)
* [http](https://github.com/open-telemetry/opentelemetry-collector/tree/main/confmap/provider/httpprovider)
* [https](https://github.com/open-telemetry/opentelemetry-collector/tree/main/confmap/provider/httpsprovider)
* [yaml](https://github.com/open-telemetry/opentelemetry-collector/tree/main/confmap/provider/yamlprovider)

{{< /collapse >}}

{{< collapse title="Receivers" >}}

* [awscloudwatch](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/awscloudwatchreceiver)
* [awsecscontainermetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/awsecscontainermetricsreceiver)
* [awss3](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/awss3receiver)
* [cloudflare](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/cloudflarereceiver)
* [datadog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/datadogreceiver)
* [faro](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/faroreceiver)
* [filelog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver)
* [filestats](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filestatsreceiver)
* [fluentforward](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/fluentforwardreceiver)
* [googlecloudpubsub](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/googlecloudpubsubreceiver)
* [hostmetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/hostmetricsreceiver)
* [influxdb](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/influxdbreceiver)
* [jaeger](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/jaegerreceiver)
* [kafka](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kafkareceiver)
* [solace](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/solacereceiver)
* [splunkhec](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/splunkhecreceiver)
* [syslog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/syslogreceiver)
* [tcplog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/tcplogreceiver)
* [vcenter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/vcenterreceiver)
* [zipkin](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/zipkinreceiver)
* [otlp](https://github.com/open-telemetry/opentelemetry-collector/tree/main/receiver/otlpreceiver)
{{< /collapse >}}

{{< collapse title="Connectors" >}}
* [count](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/countconnector)
* [grafanacloud](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/grafanacloudconnector)
* [servicegraph](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/servicegraphconnector)
* [spanmetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/spanmetricsconnector)
* [forward](https://github.com/open-telemetry/opentelemetry-collector/tree/main/connector/forwardconnector)

{{< /collapse >}}

{{< collapse title="Processors" >}}

* [attributes](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/attributesprocessor)
* [cumulativetodelta](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor)
* [deltatocumulative](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/deltatocumulativeprocessor)
* [filter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor)
* [groupbyattrs](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/groupbyattrsprocessor)
* [interval](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/intervalprocessor)
* [k8sattributes](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/k8sattributesprocessor)
* [metricstarttime](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstarttimeprocessor)
* [probabilisticsampler](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/probabilisticsamplerprocessor)
* [resource](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourceprocessor)
* [resourcedetection](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor)
* [span](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/spanprocessor)
* [tailsampling](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor)
* [transform](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor)
* [batch](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)
* [memorylimiter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiterprocessor)

{{< /collapse >}}

{{< collapse title="Exporters" >}}

* [awss3](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/awss3exporter)
* [faro](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/faroexporter)
* [file](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/fileexporter)
* [googlecloud](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/googlecloudexporter)
* [googlecloudpubsub](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/googlecloudpubsubexporter)
* [kafka](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/kafkaexporter)
* [loadbalancing](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/loadbalancingexporter)
* [prometheus](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/prometheusexporter)
* [prometheusremotewrite](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/prometheusremotewriteexporter)
* [splunkhec](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/splunkhecexporter)
* [syslog](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/syslogexporter)
* [zipkin](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/zipkinexporter)
* [debug](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/debugexporter)
* [nop](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/nopexporter)
* [otlp](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlpexporter)
* [otlphttp](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter)

{{< /collapse >}}

To view the full list of components and their versioning, please refer to the [OCB manifest](https://github.com/grafana/alloy/blob/main/collector/builder-config.yaml)

## Get Started
You can find information for getting off the ground with our OTel Engine in the [Getting Started](getting-started.md) docs
---
canonical: https://grafana.com/docs/alloy/latest/collect/datadog-traces-metrics/
aliases:
  - ../tasks/configure-alloy-to-use-datadog-receiver/ # /docs/alloy/latest/tasks/configure-alloy-to-use-datadog-receiver/
description: Learn how to configure Grafana Alloy to use the Datadog receiver
menuTitle: Collect Datadog traces and metrics
title: Receive traces and metrics from Datadog-instrumented applications
weight: 200
---

# Receive traces and metrics from Datadog-instrumented applications

You can configure {{< param "PRODUCT_NAME" >}} to collect [Datadog][] traces and metrics and forward them to any OpenTelemetry-compatible database.

This topic describes how to:

* Configure {{< param "PRODUCT_NAME" >}} to send traces and metrics.
* Configure the {{< param "PRODUCT_NAME" >}} Datadog Receiver.
* Configure the Datadog Agent to forward traces and metrics to the {{< param "PRODUCT_NAME" >}} Datadog Receiver.

## Before you begin

* Ensure that at least one instance of the [Datadog Agent][] is collecting metrics and traces.
* Identify where to write the collected telemetry.
  Metrics can be written to [Prometheus][] or any other OpenTelemetry-compatible database such as Grafana Mimir, Grafana Cloud, or Grafana Enterprise Metrics.
  Traces can be written to Grafana Tempo, Grafana Cloud, or Grafana Enterprise Traces.
* Be familiar with the concept of [Components][] in {{< param "PRODUCT_NAME" >}}.

## Configure {{% param "PRODUCT_NAME" %}} to send traces and metrics

Before components can collect Datadog telemetry signals, you must have a component responsible for writing this telemetry somewhere.

The [otelcol.exporter.otlp][] component is responsible for delivering OTLP data to OpenTelemetry-compatible endpoints.

1. Add the following `otelcol.exporter.otlp` component to your configuration file.

   ```alloy
   otelcol.exporter.otlp "default" {
     client {
       endpoint = "<OTLP_ENDPOINT_URL>"
       auth     = otelcol.auth.basic.auth.handler
     }
   }
   ```

   Replace the following:

   * _`<OTLP_ENDPOINT_URL>`_: The full URL of the OpenTelemetry-compatible endpoint where metrics and traces are sent, such as `https://otlp-gateway-prod-eu-west-2.grafana.net/otlp`.

1. If your endpoint requires basic authentication, paste the following inside the `endpoint` block.

   ```alloy
   basic_auth {
     username = "<USERNAME>"
     password = "<PASSWORD>"
   }
   ```

   Replace the following:

   * _`<USERNAME>`_: The basic authentication username.
   * _`<PASSWORD>`_: The basic authentication password or API key.

## Configure the {{% param "PRODUCT_NAME" %}} Datadog Receiver

1. Add the following `otelcol.processor.batch` component to your configuration file.

   ```alloy
   otelcol.processor.batch "default" {
     output {
       metrics = [otelcol.exporter.otlp.default.input]
       traces  = [otelcol.exporter.otlp.default.input]
     }
   }
   ```

1. Add the following `otelcol.processor.deltatocumulative` component to your configuration file.

   ```alloy
   otelcol.processor.deltatocumulative "default" {
     max_stale = "<MAX_STALE>"
     max_streams = <MAX_STREAMS>
     output {
       metrics = [otelcol.processor.batch.default.input]
     }
   }
   ```

   Replace the following:

   * _`<MAX_STALE>`_: How long until a series not receiving new samples is removed, such as "5m".
   * _`<MAX_STREAMS>`_: The upper limit of streams to track. New streams exceeding this limit are dropped.

1. Add the following `otelcol.receiver.datadog` component to your configuration file.

   ```alloy
   otelcol.receiver.datadog "default" {
     endpoint = "<HOST>:<PORT>"
     output {
       metrics = [otelcol.processor.deltatocumulative.default.input]
       traces  = [otelcol.processor.batch.default.input]
     }
   }
   ```

    Replace the following:

    * _`<HOST>`_: The host address where the receiver listens.
    * _`<PORT>`_: The port where the receiver listens.

1. If your endpoint requires basic authentication, paste the following inside the `endpoint` block.

   ```alloy
   basic_auth {
     username = "<USERNAME>"
     password = "<PASSWORD>"
   }
   ```

    Replace the following:

    * _`<USERNAME>`_: The basic authentication username.
    * _`<PASSWORD>`_: The basic authentication password or API key.

## Configure Datadog Agent to forward telemetry to the {{% param "PRODUCT_NAME" %}} Datadog Receiver

You can set up your Datadog Agent to forward Datadog metrics and traces simultaneously to {{< param "PRODUCT_NAME" >}} and Datadog.

We recommend this approach for current Datadog users who want to try using {{< param "PRODUCT_NAME" >}}.

1. Add the following environment variables to your Datadog-agent installation.

   ```bash
   DD_ADDITIONAL_ENDPOINTS='{"http://<DATADOG_RECEIVER_HOST>:<DATADOG_RECEIVER_PORT>": ["datadog-receiver"]}'
   DD_APM_ADDITIONAL_ENDPOINTS='{"http://<DATADOG_RECEIVER_HOST>:<DATADOG_RECEIVER_PORT>": ["datadog-receiver"]}'
   ```

   `DD_ADDITIONAL_ENDPOINTS` is used for forwarding metrics, whereas `DD_APM_ADDITIONAL_ENDPOINTS` is for traces.

   Replace the following:

   * _`<DATADOG_RECEIVER_HOST>`_: The hostname where the {{< param "PRODUCT_NAME" >}} receiver is found.
   * _`<DATADOG_RECEIVER_PORT>`_: The port where the {{< param "PRODUCT_NAME" >}} receiver is exposed.

Alternatively, you might want your Datadog Agent to send metrics only to {{< param "PRODUCT_NAME" >}}.
You can do this by setting up your Datadog Agent in the following way:

1. Replace the DD_URL in the configuration YAML:

   ```yaml
    dd_url: http://<DATADOG_RECEIVER_HOST>:<DATADOG_RECEIVER_PORT>
   ```

   Or by setting an environment variable:

   ```bash
   DD_URL='{"http://<DATADOG_RECEIVER_HOST>:<DATADOG_RECEIVER_PORT>": ["datadog-receiver"]}'
   ```

## Run {{% param "PRODUCT_NAME" %}}

The `otelcol.receiver.datadog` component is experimental.
To use this component, you need to start {{< param "PRODUCT_NAME" >}} with additional command line flags:

   ```bash
   alloy run config.alloy --stability.level=experimental
   ```

[Datadog]: https://www.datadoghq.com/
[Datadog Agent]: https://docs.datadoghq.com/agent/
[Prometheus]: https://prometheus.io
[otelcol.exporter.otlp]: ../../reference/components/otelcol/otelcol.exporter.otlp/
[Components]: ../../get-started/components/

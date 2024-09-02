---
canonical: https://grafana.com/docs/alloy/latest/collect/metamonitoring/
aliases:
  - ../tasks/metamonitoring/ # /docs/alloy/latest/tasks/metamonitoring/
description: Learn how to set up meta-monitoring for Grafana Alloy
menuTitle: Collect Alloy telemetry
title: Set up meta-monitoring to collect Alloy telemetry
weight: 175
---

# Set up meta-monitoring to collect {{% param "PRODUCT_NAME" %}} telemetry

You can configure {{< param "PRODUCT_NAME" >}} to collect its own telemetry and forward it to the backend of your choosing.

This topic describes how to collect and forward metrics, logs, and traces data from {{< param "PRODUCT_NAME" >}}.

## Components and configuration blocks used in this topic

* [prometheus.exporter.self][]
* [prometheus.scrape][]
* [logging][]
* [tracing][]

## Before you begin

* Identify where to send {{< param "PRODUCT_NAME" >}}'s telemetry data.
* Be familiar with the concept of [Components][] in {{< param "PRODUCT_NAME" >}}.

## Meta-monitoring metrics

{{< param "PRODUCT_NAME" >}} exposes its internal metrics using the Prometheus exposition format.

In this task, you will use the [prometheus.exporter.self][] and [prometheus.scrape][] components to scrape {{< param "PRODUCT_NAME" >}}'s  internal metrics and forward it to compatible {{< param "PRODUCT_NAME" >}} components.

1. Add the following `prometheus.exporter.self` component to your configuration. The component accepts no arguments.

   ```alloy
   prometheus.exporter.self "<SELF_LABEL>" {
   }
   ```

1. Add the following `prometheus.scrape` component to your configuration file.
   ```alloy
   prometheus.scrape "<SCRAPE_LABEL>" {
     targets    = prometheus.exporter.self.<SELF_LABEL>.targets
     forward_to = [<METRICS_RECEIVER_LIST>]
   }
   ```

   Replace the following:
   - _`<SELF_LABEL>`_: The label for the component such as `default` or `metamonitoring`. The label must be unique across all `prometheus.exporter.self` components in the same configuration file.
   - _`<SCRAPE_LABEL>`_: The label for the scrape component such as `default`. The label must be unique across all `prometheus.scrape` components in the same configuration file.
   - _`<METRICS_RECEIVER_LIST>`_: A comma-delimited list of component receivers to forward metrics to.
     For example, to send to an existing remote write component, use `prometheus.remote_write.WRITE_LABEL.receiver`.
     Similarly, to send data to an existing relabeling component, use `prometheus.relabel.PROCESS_LABEL.receiver`.
     To use data in the OTLP format, you can send data to an existing converter component, like `otelcol.receiver.prometheus.OTEL.receiver`.

The following example demonstrates configuring a possible sequence of components.

```alloy
prometheus.exporter.self "default" {
}

prometheus.scrape "metamonitoring" {
  targets    = prometheus.exporter.self.default.targets
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://mimir:9009/api/v1/push"
  }
}
```

## Meta-monitoring logs

The [logging][] block defines the logging behavior of {{< param "PRODUCT_NAME" >}}.

In this task, you will use the [logging][] block to forward {{< param "PRODUCT_NAME" >}}'s logs to a compatible component.
The block is specified without a label and can only be provided once per configuration file.

1. Add the following `logging` configuration block to the top level of your configuration file.

   ```alloy
   logging {
     level    = "<LOG_LEVEL>"
     format   = "<LOG_FORMAT>"
     write_to = [<LOGS_RECEIVER_LIST>]
   }
   ```

   Replace the following:
   - _`<LOG_LEVEL>`_: The log level to use for {{< param "PRODUCT_NAME" >}}'s logs. If the attribute isn't set, it defaults to `info`.
   - _`<LOG_FORMAT>`_: The log format to use for {{< param "PRODUCT_NAME" >}}'s logs. If the attribute isn't set, it defaults to `logfmt`.
   - _`<LOGS_RECEIVER_LIST>`_: A comma-delimited list of component receivers to forward logs to.
     For example, to send to an existing processing component, use `loki.process.PROCESS_LABEL.receiver`.
     Similarly, to send data to an existing relabeling component, use `loki.relabel.PROCESS_LABEL.receiver`.
     To use data in the OTLP format, you can send data to an existing converter component, like `otelcol.receiver.loki.OTEL.receiver`.

The following example demonstrates configuring the logging block and sending to a compatible component.

```alloy
logging {
  level    = "warn"
  format   = "json"
  write_to = [loki.write.default.receiver]
}

loki.write "default" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}

```

## Meta-monitoring traces

The [tracing][] block defines the tracing behavior of {{< param "PRODUCT_NAME" >}}.

In this task you will use the [tracing][] block to forward {{< param "PRODUCT_NAME" >}} internal traces to a compatible component. The block is specified without a label and can only be provided once per configuration file.

1. Add the following `tracing` configuration block to the top level of your configuration file.

   ```alloy
   tracing {
     sampling_fraction = <SAMPLING_FRACTION>
     write_to          = [<TRACES_RECEIVER_LIST>]
   }
   ```

   Replace the following:
   - _`<SAMPLING_FRACTION>`_: The fraction of traces to keep. If the attribute isn't set, it defaults to `0.1`.
   - _`<TRACES_RECEIVER_LIST>`_: A comma-delimited list of component receivers to forward traces to.
     For example, to send to an existing OpenTelemetry exporter component use `otelcol.exporter.otlp.EXPORT_LABEL.input`.

The following example demonstrates configuring the tracing block and sending to a compatible component.

```alloy
tracing {
  sampling_fraction = 0.1
  write_to          = [otelcol.exporter.otlp.default.input]
}

otelcol.exporter.otlp "default" {
    client {
        endpoint = "tempo:4317"
    }
}
```

[prometheus.exporter.self]: ../../reference/components/prometheus/prometheus.exporter.self/
[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/
[logging]: ../../reference/config-blocks/logging/
[tracing]: ../../reference/config-blocks/tracing/
[Components]: ../../get-started/components/

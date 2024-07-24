---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.loki/
aliases:
  - ../otelcol.exporter.loki/ # /docs/alloy/latest/reference/components/otelcol.exporter.loki/
description: Learn about otelcol.exporter.loki
title: otelcol.exporter.loki
---

# otelcol.exporter.loki

`otelcol.exporter.loki` accepts OTLP-formatted logs from other `otelcol`
components, converts them to Loki-formatted log entries, and forwards them
to `loki` components.

{{< admonition type="note" >}}
`otelcol.exporter.loki` is a custom component unrelated to the `lokiexporter` from the OpenTelemetry Collector.
{{< /admonition >}}

The attributes of the OTLP log are not converted to Loki attributes by default.
To convert them, the OTLP log should contain special "hint" attributes:
* To convert OTLP resource attributes to Loki labels,
  use the `loki.resource.labels` hint attribute.
* To convert OTLP log attributes to Loki labels,
  use the `loki.attribute.labels` hint attribute.

Labels will be translated to a [Prometheus format][], which is more constrained than the OTLP format.
For examples on label translation, see the [Converting OTLP attributes to Loki labels][] section.

Multiple `otelcol.exporter.loki` components can be specified by giving them different labels.

[Converting OTLP attributes to Loki labels]: #converting-otlp-attributes-to-loki-labels

## Usage

```alloy
otelcol.exporter.loki "LABEL" {
  forward_to = [...]
}
```

## Arguments

`otelcol.exporter.loki` supports the following arguments:

Name         | Type             | Description                           | Default | Required
-------------|------------------|---------------------------------------|---------|---------
`forward_to` | `list(receiver)` | Where to forward converted Loki logs. |         | yes

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for logs. Other telemetry signals are ignored.

Logs sent to `input` are converted to Loki-compatible log entries and are
forwarded to the `forward_to` argument in sequence.

## Component health

`otelcol.exporter.loki` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.exporter.loki` does not expose any component-specific debug
information.

## Examples

### Basic usage

This example accepts OTLP logs over gRPC, transforms them and forwards
the converted log entries to `loki.write`:

```alloy
otelcol.receiver.otlp "default" {
  grpc {}

  output {
    logs = [otelcol.exporter.loki.default.input]
  }
}

otelcol.exporter.loki "default" {
  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "loki:3100"
  }
}
```

### Converting OTLP attributes to Loki labels

The example below will convert the following attributes to Loki labels:
* The `service.name` and `service.namespace` OTLP resource attributes.
* The `event.domain` and `event.name` OTLP log attributes.

Labels will be translated to a [Prometheus format][]. For example:

| OpenTelemetry Attribute | Prometheus Label                                      |
|-------------------------|-------------------------------------------------------|
| `name`                  | `name`                                                |
| `host.name`             | `host_name`                                           |
| `host_name`             | `host_name`                                           |
| `name (of the host)`    | `name__of_the_host_`                                  |
| `2 cents`               | `key_2_cents`                                         |
| `__name`                | `__name`                                              |
| `_name`                 | `key_name`                                            |
| `_name`                 | `_name` (if `PermissiveLabelSanitization` is enabled) |

```alloy
otelcol.receiver.otlp "default" {
  grpc {}

  output {
    logs = [otelcol.processor.attributes.default.input]
  }
}

otelcol.processor.attributes "default" {
  action {
    key = "loki.attribute.labels"
    action = "insert"
    value = "event.domain, event.name"
  }

  action {
    key = "loki.resource.labels"
    action = "insert"
    value = "service.name, service.namespace"
  }

  output {
    logs = [otelcol.exporter.loki.default.input]
  }
}

otelcol.exporter.loki "default" {
  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
      url = "loki:3100"
  }
}
```

[Prometheus format](https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels)

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.loki` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`otelcol.exporter.loki` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

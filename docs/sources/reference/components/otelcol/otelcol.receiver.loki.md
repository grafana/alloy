---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.loki/
aliases:
  - ../otelcol.receiver.loki/ # /docs/alloy/latest/reference/otelcol.receiver.loki/
description: Learn about otelcol.receiver.loki
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.receiver.loki
---

# `otelcol.receiver.loki`

`otelcol.receiver.loki` receives Loki log entries, converts them to the OpenTelemetry logs format, and forwards them to other `otelcol.*` components.

You can specify multiple `otelcol.receiver.loki` components by giving them different labels.

{{< admonition type="note" >}}
`otelcol.receiver.loki` is a custom component unrelated to any receivers from the upstream OpenTelemetry Collector.
{{< /admonition >}}

## Usage

```alloy
otelcol.receiver.loki "<LABEL>" {
  output {
    logs = [...]
  }
}
```

## Arguments

You can use the following argument with `otelcol.receiver.loki`:

| Name                            | Type   | Description                                                       | Default | Required |
|---------------------------------|--------|-------------------------------------------------------------------|---------|----------|
| `labels_as_resource_attributes` | `bool` | Convert labels to resource attributes instead of log attributes. | `false` | no       |

By default, `otelcol.receiver.loki` converts every label to a log attribute and adds a `loki.attribute.labels` attribute that lists them, so [`otelcol.exporter.loki`][otelcol.exporter.loki] can convert them back to labels.
If a log has a `filename` label, the component also sets the `log.file.path` and `log.file.name` log attributes from it.

When you set `labels_as_resource_attributes` to `true`, each set of labels becomes the resource attributes of the logs that have those labels:

* Labels that start with `__`, such as `__tenant_id__`, are dropped.
* The component doesn't add the `loki.attribute.labels` attribute.

Apart from dropping labels that start with `__`, the component maps labels to resource attributes as they are and doesn't rename or add any.
To choose which labels become resource attributes, use [`loki.relabel`][loki.relabel] or [`loki.process`][loki.process] before `otelcol.receiver.loki`.
To rename resource attributes to OpenTelemetry semantic convention names, such as `service_name` to `service.name`, use [`otelcol.processor.transform`][otelcol.processor.transform] after `otelcol.receiver.loki`.
To move a value that can differ between log records, such as structured metadata, to the resource, you also need [`otelcol.processor.groupbyattrs`][otelcol.processor.groupbyattrs] to regroup the logs.

[otelcol.exporter.loki]: ../otelcol.exporter.loki/
[otelcol.processor.transform]: ../otelcol.processor.transform/
[otelcol.processor.groupbyattrs]: ../otelcol.processor.groupbyattrs/
[loki.relabel]: ../../loki/loki.relabel/
[loki.process]: ../../loki/loki.process/

## Blocks

You can use the following blocks with `otelcol.receiver.loki`:

{{< docs/alloy-config >}}

| Block              | Description                                        | Required |
|--------------------|----------------------------------------------------|----------|
| [`output`][output] | Configures where to send converted telemetry data. | yes      |

[output]: #output

{{< /docs/alloy-config >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type           | Description                                                 |
|------------|----------------|-------------------------------------------------------------|
| `receiver` | `LogsReceiver` | A value that other components can use to send Loki logs to. |

## Component health

`otelcol.receiver.loki` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.loki` doesn't expose any component-specific debug information.

## Examples

### Bridge Loki and OpenTelemetry

This example uses the `otelcol.receiver.loki` component as a bridge between the Loki and OpenTelemetry ecosystems.
The component exposes a receiver which the `loki.source.file` component uses to send Loki log entries to.
The logs are converted to the OTLP format before they're forwarded to the `otelcol.exporter.otlphttp` component to be sent to an OTLP-capable endpoint:

```alloy
loki.source.file "default" {
  targets = [
    {__path__ = "/tmp/foo.txt", "loki.format" = "logfmt"},
    {__path__ = "/tmp/bar.txt", "loki.format" = "json"},
  ]
  forward_to = [otelcol.receiver.loki.default.receiver]
}

otelcol.receiver.loki "default" {
  output {
    logs = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

### Map labels to resource attributes

This example converts the labels of each log stream to resource attributes.
The `otelcol.processor.transform` component then renames the `service_name` resource attribute to the OpenTelemetry semantic convention name `service.name` before the logs are sent to an OTLP-capable endpoint:

```alloy
loki.source.file "default" {
  targets = [
    {__path__ = "/var/log/checkout.log", service_name = "checkout", env = "prod"},
  ]
  forward_to = [otelcol.receiver.loki.default.receiver]
}

otelcol.receiver.loki "default" {
  labels_as_resource_attributes = true

  output {
    logs = [otelcol.processor.transform.default.input]
  }
}

otelcol.processor.transform "default" {
  error_mode = "ignore"

  log_statements {
    context = "resource"
    statements = [
      `set(attributes["service.name"], attributes["service_name"])`,
      `delete_key(attributes, "service_name")`,
    ]
  }

  output {
    logs = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.loki` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.receiver.loki` has exports that can be consumed by the following components:

- Components that consume [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

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

| Block                                  | Description                                                                                      | Required |
|----------------------------------------|--------------------------------------------------------------------------------------------------|----------|
| [`output`][output]                     | Configures where to send converted telemetry data.                                               | yes      |
| [`retry_on_failure`][retry_on_failure] | Configures the retry behavior when the receiver encounters an error downstream in the pipeline. Only applies with the Loki pipeline v2. | no       |

[output]: #output
[retry_on_failure]: #retry_on_failure

{{< /docs/alloy-config >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `retry_on_failure`

{{< admonition type="note" >}}
The `retry_on_failure` block only takes effect when you enable the Loki pipeline v2.
With the Loki pipeline v1, which is the default, `otelcol.receiver.loki` logs a warning at startup and ignores the block.
{{< /admonition >}}

The `retry_on_failure` block configures the retry behavior when the receiver encounters an error downstream in the pipeline.
The receiver uses an exponential backoff algorithm with jitter to delay the retry after subsequent failures.

You can use the following arguments with `retry_on_failure`:

| Name               | Type       | Description                                                                                                         | Default | Required |
|--------------------|------------|---------------------------------------------------------------------------------------------------------------------|---------|----------|
| `enabled`          | `bool`     | If set to `true` and a non-permanent error occurs, the receiver blocks and sends the current batch of logs again.   | `false` | no       |
| `initial_interval` | `duration` | The time to wait after the first failure before retrying.                                                           | `"1s"`  | no       |
| `max_elapsed_time` | `duration` | The maximum time to keep retrying a batch before the receiver drops it.                                             | `"5m"`  | no       |
| `max_interval`     | `duration` | The maximum time to wait between retries after applying backoff.                                                    | `"30s"` | no       |

If you set `max_elapsed_time` to `0`, the receiver retries forever and never drops a batch.

When `enabled` is `false`, the receiver returns the error to the Loki component that sent the logs, and that component handles it as an unexpected error.

When `enabled` is `true`, the receiver blocks the call from the Loki component that sent the logs and sends the batch again with exponential backoff.
The receiver keeps retrying until one of the following happens:

- **Accepted:** Downstream accepts the batch.
- **Permanent error:** A permanent error occurs. The receiver never retries permanent errors.
- **Timeout:** The `max_elapsed_time` passes. The receiver drops the batch and logs an error.
- **Canceled:** The sending Loki component cancels its request.
- **Stopped:** The `otelcol.receiver.loki` component stops.

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

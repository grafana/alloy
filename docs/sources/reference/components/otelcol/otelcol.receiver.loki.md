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

The `otelcol.receiver.loki` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `otelcol.receiver.loki`:

| Block              | Description                                        | Required |
|--------------------|----------------------------------------------------|----------|
| [`output`][output] | Configures where to send converted telemetry data. | yes      |

[output]: #output

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

## Example

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

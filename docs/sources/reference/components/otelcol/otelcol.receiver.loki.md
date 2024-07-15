---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.loki/
aliases:
  - ../otelcol.receiver.loki/ # /docs/alloy/latest/reference/otelcol.receiver.loki/
description: Learn about otelcol.receiver.loki
title: otelcol.receiver.loki
---

# otelcol.receiver.loki

`otelcol.receiver.loki` receives Loki log entries, converts them to the
OpenTelemetry logs format, and forwards them to other `otelcol.*` components.

Multiple `otelcol.receiver.loki` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.receiver.loki "LABEL" {
  output {
    logs = [...]
  }
}
```

## Arguments

`otelcol.receiver.loki` doesn't support any arguments and is configured fully
through inner blocks.

## Blocks

The following blocks are supported inside the definition of
`otelcol.receiver.loki`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
output | [output][] | Configures where to send converted telemetry data. | yes

[output]: #output-block

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`receiver` | `LogsReceiver` | A value that other components can use to send Loki logs to.

## Component health

`otelcol.receiver.loki` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.receiver.loki` does not expose any component-specific debug
information.

## Example

This example uses the `otelcol.receiver.loki` component as a bridge
between the Loki and OpenTelemetry ecosystems. The component exposes a
receiver which the `loki.source.file` component uses to send Loki log entries
to. The logs are converted to the OTLP format before they are forwarded
to the `otelcol.exporter.otlp` component to be sent to an OTLP-capable
endpoint:

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
    logs = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = env("OTLP_ENDPOINT")
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

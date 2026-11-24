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

{{< docs/alloy-config >}}

| Block                                  | Description                                                                                      | Required |
|----------------------------------------|--------------------------------------------------------------------------------------------------|----------|
| [`output`][output]                     | Configures where to send converted telemetry data.                                               | yes      |
| [`retry_on_failure`][retry_on_failure] | Configures the retry behavior when the receiver encounters an error downstream in the pipeline. | no       |

[output]: #output
[retry_on_failure]: #retry_on_failure

{{< /docs/alloy-config >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `retry_on_failure`

The `retry_on_failure` block configures the retry behavior when the receiver encounters an error downstream in the pipeline.
A backoff algorithm is used to delay the retry upon subsequent failures.
`retry_on_failure` only takes effect when the loki pipeline v2 is enabled. Otherwise the component logs a warning and ignores it.
The following arguments are supported:

| Name               | Type       | Description                                                                                                 | Default | Required |
|--------------------|------------|-------------------------------------------------------------------------------------------------------------|---------|----------|
| `enabled`          | `bool`     | If set to `true` and a non-permanent error occurs, the receiver blocks and resends the current batch of logs. | `false` | no       |
| `initial_interval` | `duration` | The time to wait after first failure to retry.                                                              | `"1s"`  | no       |
| `max_elapsed_time` | `duration` | The maximum age of a message before the data is discarded.                                                  | `"5m"`  | no       |
| `max_interval`     | `duration` | The maximum time to wait after applying backoff logic.                                                      | `"30s"` | no       |

If `max_elapsed_time` is set to `0` data is never discarded.

When `enabled` is `false`, the error is returned to the Loki component that sent the logs, which handles it as an unexpected error.
When `enabled` is `true`, the Loki component that sent the logs waits until the batch is accepted, dropped, or the component stops. If the sender cancels its request, the wait ends too.

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

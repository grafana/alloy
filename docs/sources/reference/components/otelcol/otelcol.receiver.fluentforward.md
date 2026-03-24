---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.fluentforward/
description: Learn about otelcol.receiver.fluentforward
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.fluentforward
---

# `otelcol.receiver.fluentforward`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.fluentforward` accepts log messages over a TCP connection via the [Fluent Forward Protocol](https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1) and forwards them as logs to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.fluentforward` is a wrapper over the upstream OpenTelemetry Collector [`fluentforward`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`fluentforward`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/fluentforwardreceiver
{{< /admonition >}}

You can specify multiple `otelcol.receiver.fluentforward` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.fluentforward "<LABEL>" {
  endpoint = "<IP_ADDRESS:PORT>"

  output {
    logs    = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.fluentforward`:

| Name       | Type     | Description                                                                            | Default | Required |
|------------|----------|----------------------------------------------------------------------------------------|---------|----------|
| `endpoint` | `string` | The `<HOST:PORT>` or `unix://<path to socket>` address to listen to for logs messages. |         | yes      |

## Blocks

You can use the following blocks with `otelcol.receiver.fluentforward`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[debug_metrics]: #debug_metrics
[output]: #output

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block-logs.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.fluentforward` doesn't export any fields.

## Component health

`otelcol.receiver.fluentforward` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.fluentforward` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.receiver.fluentforward` doesn't expose any component-specific debug metrics.

## Example

This example receives log messages using Fluent Forward Protocol on TCP port 8006 and logs them.

```alloy
otelcol.receiver.fluentforward "default" {
    endpoint = "localhost:8006"
    output {
        logs = [otelcol.exporter.debug.default.input]
    }
}

otelcol.exporter.debug "default" {}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.fluentforward` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

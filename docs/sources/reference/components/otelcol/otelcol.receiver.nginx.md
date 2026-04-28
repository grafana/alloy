---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.nginx/
description: Learn about otelcol.receiver.nginx
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.nginx
---

# `otelcol.receiver.nginx`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.nginx` reads NGINX metrics and forwards them to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.nginx` is a wrapper over the upstream OpenTelemetry Collector [`nginx`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`nginx`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/nginxreceiver
{{< /admonition >}}

This receiver supports NGINX. Refer to the upstream [`nginx`][] receiver documentation for more details.

You can specify multiple `otelcol.receiver.nginx` components by giving them different labels.

[`nginx`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/nginxreceiver

## Usage

```alloy
otelcol.receiver.nginx "<LABEL>" {
  endpoint = "http://localhost:80/status"

  collection_interval = "10s"
  initial_delay = "1s"

  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.nginx`:

| Name                  | Type       | Description                                            | Default | Required |
| --------------------- | ---------- | ------------------------------------------------------ | ------- | -------- |
| `endpoint`            | `string`   | The URL of the NGINX status endpoint.                  |         | yes      |
| `collection_interval` | `duration` | Defines how often to collect metrics.                  | `"10s"` | no       |
| `initial_delay`       | `duration` | Defines how long this receiver waits before it starts. | `"1s"`  | no       |

## Blocks

You can use the following blocks with `otelcol.receiver.nginx`:

{{< docs/alloy-config >}}

| Block                            | Description                                                                | Required |
| -------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[debug_metrics]: #debug_metrics
[output]: #output

{{< /docs/alloy-config >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.nginx` doesn't export any fields.

## Component health

`otelcol.receiver.nginx` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.nginx` doesn't expose any component-specific debug information.

## Example

The following example collects all available metrics from an NGINX server and forwards them to an exporter.

```alloy
otelcol.receiver.nginx "default" {
  endpoint = "http://localhost:80/status"

  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = env("<OTLP_ENDPOINT>")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.nginx` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

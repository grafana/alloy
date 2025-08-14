---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.auth.basic/
aliases:
  - ../otelcol.auth.basic/ # /docs/alloy/latest/reference/components/otelcol.auth.basic/
description: Learn about otelcol.auth.basic
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.auth.basic
---

# `otelcol.auth.basic`

`otelcol.auth.basic` exposes a `handler` that other `otelcol` components can use to authenticate requests using basic authentication.

This component supports both server and client authentication.

{{< admonition type="note" >}}
`otelcol.auth.basic` is a wrapper over the upstream OpenTelemetry Collector [`basicauth`][] extension.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`basicauth`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/basicauthextension
{{< /admonition >}}

You can specify multiple `otelcol.auth.basic` components by giving them different labels.

## Usage

```alloy
otelcol.auth.basic "<LABEL>" {
  username = "<USERNAME>"
  password = "<PASSWORD>"
}
```

## Arguments

You can use the following arguments with `otelcol.auth.basic`:

| Name       | Type     | Description                                        | Default | Required |
| ---------- | -------- | -------------------------------------------------- | ------- | -------- |
| `password` | `secret` | Password to use for basic authentication requests. |         | yes      |
| `username` | `string` | Username to use for basic authentication requests. |         | yes      |

## Blocks

You can use the following block with `otelcol.auth.basic`:

| Block                            | Description                                                                | Required |
| -------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[debug_metrics]: #debug_metrics

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                       | Description                                                     |
| --------- | -------------------------- | --------------------------------------------------------------- |
| `handler` | `capsule(otelcol.Handler)` | A value that other components can use to authenticate requests. |

## Component health

`otelcol.auth.basic` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.auth.basic` doesn't expose any component-specific debug information.

## Example

This example configures [`otelcol.exporter.otlp`][otelcol.exporter.otlp] to use basic authentication:

```alloy
otelcol.exporter.otlp "example" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth     = otelcol.auth.basic.creds.handler
  }
}

otelcol.auth.basic "creds" {
  username = "demo"
  password = sys.env("API_KEY")
}
```

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/

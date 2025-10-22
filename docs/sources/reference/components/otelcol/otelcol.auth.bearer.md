---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.auth.bearer/
aliases:
  - ../otelcol.auth.bearer/ # /docs/alloy/latest/reference/components/otelcol.auth.bearer/
description: Learn about otelcol.auth.bearer
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.auth.bearer
---

# `otelcol.auth.bearer`

`otelcol.auth.bearer` exposes a `handler` that other `otelcol` components can use to authenticate requests using bearer token authentication.

This component supports both server and client authentication.

{{< admonition type="note" >}}
`otelcol.auth.bearer` is a wrapper over the upstream OpenTelemetry Collector [`bearertokenauth`][] extension.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`bearertokenauth`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/bearertokenauthextension
{{< /admonition >}}

You can specify multiple `otelcol.auth.bearer` components by giving them different labels.

## Usage

```alloy
otelcol.auth.bearer "<LABEL>" {
  token = "<TOKEN>"
}
```

## Arguments

You can use the following arguments with `otelcol.auth.bearer`:

| Name     | Type     | Description                                      | Default           | Required |
| -------- | -------- | ------------------------------------------------ | ----------------- | -------- |
| `token`  | `secret` | Bearer token to use for authenticating requests. |                   | yes      |
| `header` | `string` | Specifies the auth header name.                  | `"Authorization"` | no       |
| `scheme` | `string` | Authentication scheme name.                      | `"Bearer"`        | no       |

When sending the token, the value of `scheme` is prepended to the `token` value.
The string is then sent out as either a header for HTTP or as metadata for gRPC.

If you use a file to store the token, you can use [`local.file`][local.file] to retrieve the `token` value from the file.

[local.file]: ../../local/local.file/

## Blocks

You can use the following block with `otelcol.auth.bearer`:

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

`otelcol.auth.bearer` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.auth.bearer` doesn't expose any component-specific debug information.

## Examples

### Default scheme via gRPC transport

The following example configures [`otelcol.exporter.otlp`][otelcol.exporter.otlp] to use a bearer token authentication.

If you assume that the value of the `API_KEY` environment variable is `SECRET_API_KEY`, then the `Authorization` RPC metadata is set to `Bearer SECRET_API_KEY`.

```alloy
otelcol.exporter.otlp "example" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth     = otelcol.auth.bearer.creds.handler
  }
}

otelcol.auth.bearer "creds" {
  token = sys.env("<API_KEY>")
}
```

### Custom scheme via HTTP transport

The following example configures [`otelcol.exporter.otlphttp`][otelcol.exporter.otlphttp] to use a bearer token authentication.

If you assume that the value of the `API_KEY` environment variable is `SECRET_API_KEY`, then the `Authorization` HTTP header is set to `MyScheme SECRET_API_KEY`.

```alloy
otelcol.exporter.otlphttp "example" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth     = otelcol.auth.bearer.creds.handler
  }
}

otelcol.auth.bearer "creds" {
  token = sys.env("<API_KEY>")
  scheme = "MyScheme"
}
```

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/
[otelcol.exporter.otlphttp]: ../otelcol.exporter.otlphttp/

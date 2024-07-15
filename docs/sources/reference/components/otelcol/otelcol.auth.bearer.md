---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.auth.bearer/
aliases:
  - ../otelcol.auth.bearer/ # /docs/alloy/latest/reference/components/otelcol.auth.bearer/
description: Learn about otelcol.auth.bearer
title: otelcol.auth.bearer
---

# otelcol.auth.bearer

`otelcol.auth.bearer` exposes a `handler` that can be used by other `otelcol`
components to authenticate requests using bearer token authentication.

This extension supports both server and client authentication.

{{< admonition type="note" >}}
`otelcol.auth.bearer` is a wrapper over the upstream OpenTelemetry Collector `bearertokenauth` extension.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

Multiple `otelcol.auth.bearer` components can be specified by giving them different labels.

## Usage

```alloy
otelcol.auth.bearer "LABEL" {
  token = "TOKEN"
}
```

## Arguments

`otelcol.auth.bearer` supports the following arguments:

Name     | Type     | Description                                      | Default  | Required
---------|----------|--------------------------------------------------|----------|---------
`token`  | `secret` | Bearer token to use for authenticating requests. |          | yes
`scheme` | `string` | Authentication scheme name.                      | "Bearer" | no

When sending the token, the value of `scheme` is prepended to the `token` value.
The string is then sent out as either a header (in case of HTTP) or as metadata (in case of gRPC).

## Blocks

The following blocks are supported inside the definition of
`otelcol.auth.bearer`:

Hierarchy | Block      | Description                          | Required
----------|------------|--------------------------------------|---------
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

[debug_metrics]: #debug_metrics-block

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                       | Description
----------|----------------------------|----------------------------------------------------------------
`handler` | `capsule(otelcol.Handler)` | A value that other components can use to authenticate requests.

## Component health

`otelcol.auth.bearer` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.auth.bearer` does not expose any component-specific debug information.

## Examples

### Default scheme via gRPC transport

The example below configures [otelcol.exporter.otlp][] to use a bearer token authentication.

If we assume that the value of the `API_KEY` environment variable is `SECRET_API_KEY`, then the `Authorization` RPC metadata is set to `Bearer SECRET_API_KEY`.

```alloy
otelcol.exporter.otlp "example" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth     = otelcol.auth.bearer.creds.handler
  }
}

otelcol.auth.bearer "creds" {
  token = env("API_KEY")
}
```

### Custom scheme via HTTP transport

The example below configures [otelcol.exporter.otlphttp][] to use a bearer token authentication.

If we assume that the value of the `API_KEY` environment variable is `SECRET_API_KEY`, then 
the `Authorization` HTTP header is set to `MyScheme SECRET_API_KEY`.

```alloy
otelcol.exporter.otlphttp "example" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth     = otelcol.auth.bearer.creds.handler
  }
}

otelcol.auth.bearer "creds" {
  token = env("API_KEY")
  scheme = "MyScheme"
}
```

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/
[otelcol.exporter.otlphttp]: ../otelcol.exporter.otlphttp/

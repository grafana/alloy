---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/otelcol.auth.bearer/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/otelcol.auth.bearer/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/otelcol.auth.bearer/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/otelcol.auth.bearer/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/otelcol.auth.bearer/
description: Learn about otelcol.auth.bearer
title: otelcol.auth.bearer
---

# otelcol.auth.bearer

`otelcol.auth.bearer` exposes a `handler` that can be used by other `otelcol`
components to authenticate requests using bearer token authentication.

This extension supports both server and client authentication.

> **NOTE**: `otelcol.auth.bearer` is a wrapper over the upstream OpenTelemetry
> Collector `bearertokenauth` extension. Bug reports or feature requests will
> be redirected to the upstream repository, if necessary.

Multiple `otelcol.auth.bearer` components can be specified by giving them
different labels.

## Usage

```river
otelcol.auth.bearer "LABEL" {
  token = "TOKEN"
}
```

## Arguments

`otelcol.auth.bearer` supports the following arguments:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`token` | `secret` | Bearer token to use for authenticating requests. | | yes
`scheme` | `string` | Authentication scheme name. | "Bearer" | no

When sending the token, the value of `scheme` is prepended to the `token` value.
The string is then sent out as either a header (in case of HTTP) or as metadata (in case of gRPC).

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`handler` | `capsule(otelcol.Handler)` | A value that other components can use to authenticate requests.

## Component health

`otelcol.auth.bearer` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.auth.bearer` does not expose any component-specific debug information.

## Examples

### Default scheme via gRPC transport

The example below configures [otelcol.exporter.otlp][] to use a bearer token authentication.

If we assume that the value of the `API_KEY` environment variable is `SECRET_API_KEY`, then 
the `Authorization` RPC metadata is set to `Bearer SECRET_API_KEY`.

```river
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

```river
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

[otelcol.exporter.otlp]: {{< relref "./otelcol.exporter.otlp.md" >}}
[otelcol.exporter.otlphttp]: {{< relref "./otelcol.exporter.otlphttp.md" >}}

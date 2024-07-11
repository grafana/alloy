---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.auth.basic/
aliases:
  - ../otelcol.auth.basic/ # /docs/alloy/latest/reference/components/otelcol.auth.basic/
description: Learn about otelcol.auth.basic
title: otelcol.auth.basic
---

# otelcol.auth.basic

`otelcol.auth.basic` exposes a `handler` that can be used by other `otelcol`
components to authenticate requests using basic authentication.

This extension supports both server and client authentication.

> **NOTE**: `otelcol.auth.basic` is a wrapper over the upstream OpenTelemetry
> Collector `basicauth` extension. Bug reports or feature requests will be
> redirected to the upstream repository, if necessary.

Multiple `otelcol.auth.basic` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.auth.basic "LABEL" {
  username = "USERNAME"
  password = "PASSWORD"
}
```

## Arguments

`otelcol.auth.basic` supports the following arguments:

Name       | Type     | Description                                        | Default | Required
-----------|----------|----------------------------------------------------|---------|---------
`username` | `string` | Username to use for basic authentication requests. |         | yes
`password` | `secret` | Password to use for basic authentication requests. |         | yes

## Blocks

The following blocks are supported inside the definition of
`otelcol.auth.basic`:

Hierarchy | Block      | Description                          | Required
----------|------------|--------------------------------------|---------
debug_metrics  | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

[debug_metrics]: #debug_metrics-block

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name      | Type                       | Description
----------|----------------------------|----------------------------------------------------------------
`handler` | `capsule(otelcol.Handler)` | A value that other components can use to authenticate requests.

## Component health

`otelcol.auth.basic` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.auth.basic` does not expose any component-specific debug information.

## Example

This example configures [otelcol.exporter.otlp][] to use basic authentication:

```alloy
otelcol.exporter.otlp "example" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth     = otelcol.auth.basic.creds.handler
  }
}

otelcol.auth.basic "creds" {
  username = "demo"
  password = env("API_KEY")
}
```

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/

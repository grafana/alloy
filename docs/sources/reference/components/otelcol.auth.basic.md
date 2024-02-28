---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/otelcol.auth.basic/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/otelcol.auth.basic/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/otelcol.auth.basic/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/otelcol.auth.basic/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/otelcol.auth.basic/
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

```river
otelcol.auth.basic "LABEL" {
  username = "USERNAME"
  password = "PASSWORD"
}
```

## Arguments

`otelcol.auth.basic` supports the following arguments:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`username` | `string` | Username to use for basic authentication requests. | | yes
`password` | `secret` | Password to use for basic authentication requests. | | yes

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`handler` | `capsule(otelcol.Handler)` | A value that other components can use to authenticate requests.

## Component health

`otelcol.auth.basic` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.auth.basic` does not expose any component-specific debug information.

## Example

This example configures [otelcol.exporter.otlp][] to use basic authentication:

```river
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

[otelcol.exporter.otlp]: {{< relref "./otelcol.exporter.otlp.md" >}}

---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/otelcol.auth.oauth2/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/otelcol.auth.oauth2/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/otelcol.auth.oauth2/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/otelcol.auth.oauth2/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/otelcol.auth.oauth2/
description: Learn about otelcol.auth.oauth2
title: otelcol.auth.oauth2
---

# otelcol.auth.oauth2

`otelcol.auth.oauth2` exposes a `handler` that can be used by other `otelcol`
components to authenticate requests using OAuth 2.0.

The authorization tokens can be used by HTTP and gRPC based OpenTelemetry exporters.
This component can fetch and refresh expired tokens automatically. For further details about
OAuth 2.0 Client Credentials flow (2-legged workflow) see [this document](https://datatracker.ietf.org/doc/html/rfc6749#section-4.4).

> **NOTE**: `otelcol.auth.oauth2` is a wrapper over the upstream OpenTelemetry
> Collector `oauth2client` extension. Bug reports or feature requests will be
> redirected to the upstream repository, if necessary.

Multiple `otelcol.auth.oauth2` components can be specified by giving them
different labels.

## Usage

```river
otelcol.auth.oauth2 "LABEL" {
    client_id     = "CLIENT_ID"
    client_secret = "CLIENT_SECRET"
    token_url     = "TOKEN_URL"
}
```

## Arguments

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`client_id` | `string` | The client identifier issued to the client. | | yes
`client_secret` | `secret` | The secret string associated with the client identifier. | | yes
`token_url` | `string` | The server endpoint URL from which to get tokens. | | yes
`endpoint_params` | `map(list(string))` | Additional parameters that are sent to the token endpoint. | `{}` | no
`scopes` | `list(string)` | Requested permissions associated for the client. | `[]` | no
`timeout` | `duration` | The timeout on the client connecting to `token_url`. | `"0s"` | no

The `timeout` argument is used both for requesting initial tokens and for refreshing tokens. `"0s"` implies no timeout.

## Blocks

The following blocks are supported inside the definition of
`otelcol.auth.oauth2`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
tls | [tls][] | TLS settings for the token client. | no

[tls]: #tls-block

### tls block

The `tls` block configures TLS settings used for connecting to the token client. If the `tls` block isn't provided, 
TLS won't be used for communication.

{{< docs/shared lookup="flow/reference/components/otelcol-tls-config-block.md" source="agent" version="<AGENT_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`handler` | `capsule(otelcol.Handler)` | A value that other components can use to authenticate requests.

## Component health

`otelcol.auth.oauth2` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.auth.oauth2` does not expose any component-specific debug information.

## Example

This example configures [otelcol.exporter.otlp][] to use OAuth 2.0 for authentication:

```river
otelcol.exporter.otlp "example" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth     = otelcol.auth.oauth2.creds.handler
  }
}

otelcol.auth.oauth2 "creds" {
    client_id     = "someclientid"
    client_secret = "someclientsecret"
    token_url     = "https://example.com/oauth2/default/v1/token"
}
```

Here is another example with some optional attributes specified:
```river
otelcol.exporter.otlp "example" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth     = otelcol.auth.oauth2.creds.handler
  }
}

otelcol.auth.oauth2 "creds" {
    client_id       = "someclientid2"
    client_secret   = "someclientsecret2"
    token_url       = "https://example.com/oauth2/default/v1/token"
    endpoint_params = {"audience" = ["someaudience"]}
    scopes          = ["api.metrics"]
    timeout         = "3600s"
}
```

[otelcol.exporter.otlp]: {{< relref "./otelcol.exporter.otlp.md" >}}

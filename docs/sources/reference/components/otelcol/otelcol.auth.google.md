---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.auth.google/
aliases:
  - ../otelcol.auth.google/ # /docs/alloy/latest/reference/components/otelcol.auth.google/
description: Learn about otelcol.auth.google
labels:
  stage: public-preview
  products:
    - oss
title: otelcol.auth.google
---

# `otelcol.auth.google`

`otelcol.auth.google` exposes a `handler` that other `otelcol` components can use to authenticate requests using Google Application Default Credentials.

This component only supports client authentication.

The authorization tokens can be used by HTTP and gRPC based OpenTelemetry exporters.
This component can fetch and refresh expired tokens automatically.
Refer to the [Google Cloud Documentation](https://docs.cloud.google.com/docs/authentication/application-default-credentials) for more information about the Application Default Credentials.

{{< admonition type="note" >}}
`otelcol.auth.google` is a wrapper over the upstream OpenTelemetry Collector [`googleclientauth`][] extension.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`googleclientauth`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/googleclientauthextension
{{< /admonition >}}

You can specify multiple `otelcol.auth.google` components by giving them different labels.

## Usage

```alloy
otelcol.auth.google "<LABEL>" {
    project     = "<PROJECT_ID>"
}
```

## Arguments

You can use the following arguments with `otelcol.auth.google`:

| Name                 | Type                | Description                                                                        | Default | Required |
| -------------------- | ------------------- | ---------------------------------------------------------------------------------- | ------- | -------- |
| `project`            | `string`            | The project telemetry is sent to.                                                  | Project from Credentials | no      |
| `quota_project`      | `string`            | The a project for quota and billing purposes.                                      | `project` | no       |
| `token_type`         | `string`            | The type of token will be generated. One of `access_token`, or `id_token`          | `access_token` | no       |
| `audience`           | `string`            | The audience claim used for generating ID token.                                   |         | no       |
| `scopes`             | `list(string)`      | Requested permissions associated for the client.                                   | `[]`    | no       |


## Blocks

You can use the following blocks with `otelcol.auth.google`:

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

`otelcol.auth.google` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.auth.google` doesn't expose any component-specific debug information.

## Example

This example configures [`otelcol.exporter.otlp`][otelcol.exporter.otlp] to use `otelcol.auth.google` for authentication:

```alloy
otelcol.exporter.otlp "google" {
  client {
    endpoint = "telemetry.googleapis.com"
    auth     = otelcol.auth.google.creds.handler
  }
}

otelcol.auth.google "creds" {
    project = "myproject"
}
```

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/

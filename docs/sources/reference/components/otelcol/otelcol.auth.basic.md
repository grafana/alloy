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

{{< admonition type="caution" >}}
Don't use the top-level `username` and `password` arguments for new configurations as they are deprecated. Use the `client_auth` block for client authentication and the `htpasswd` block for server authentication instead.
{{< /admonition >}}

You can use the following arguments with `otelcol.auth.basic`:

| Name       | Type     | Description                                                     | Default | Required |
|------------|----------|-----------------------------------------------------------------|---------|----------|
| `password` | `secret` | (Deprecated) Password to use for basic authentication requests. |         | no       |
| `username` | `string` | (Deprecated) Username to use for basic authentication requests. |         | no       |


## Blocks

You can use the following block with `otelcol.auth.basic`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`client_auth`][client_auth]     | Configures client authentication credentials for exporters.                | no       |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |
| [`htpasswd`][htpasswd]           | Configures server authentication using htpasswd format for receivers.      | no       |


[client_auth]: #client_auth
[debug_metrics]: #debug_metrics
[htpasswd]: #htpasswd

### `client_auth`

The `client_auth` block configures credentials that client extensions (such as exporters) use to authenticate to servers.

| Name       | Type     | Description                                        | Default | Required |
| ---------- | -------- | -------------------------------------------------- | ------- | -------- |
| `password` | `string` | Password to use for basic authentication requests. |         | yes      |
| `username` | `string` | Username to use for basic authentication requests. |         | yes      |

{{< admonition type="note" >}}
When you specify both the `client_auth` block and the deprecated top-level `username` and `password` attributes, the `client_auth` block takes precedence and {{< param "PRODUCT_NAME" >}} ignores the top-level attributes for client authentication.
{{< /admonition >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `htpasswd`

The `htpasswd` block configures how server extensions (such as receivers) authenticate incoming requests using the `htpasswd` format.

| Name     | Type     | Description                                                           | Default | Required |
| -------- | -------- | --------------------------------------------------------------------- | ------- | -------- |
| `file`   | `string` | Path to the `htpasswd` file to use for basic authentication requests. | `""`    | no       |
| `inline` | `string` | The `htpasswd` file content in inline format.                         | `""`    | no       |

You can specify either `file`, `inline`, or both.
When you use `inline`, the format should be `username:password` with each user on a new line.

{{< admonition type="note" >}}
When you specify both the `htpasswd` block and the deprecated top-level `username` and `password` attributes, {{< param "PRODUCT_NAME" >}} automatically appends the deprecated credentials to the `inline` content.
This allows authentication using credentials from both the `htpasswd` configuration and the deprecated attributes.
If the same username appears in both the `file` and `inline` content, including appended deprecated credentials, the entry in the `inline` content takes precedence.
{{< /admonition >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                       | Description                                                     |
| --------- | -------------------------- | --------------------------------------------------------------- |
| `handler` | `capsule(otelcol.Handler)` | A value that other components can use to authenticate requests. |

## Component health

`otelcol.auth.basic` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.auth.basic` doesn't expose any component-specific debug information.

## Examples

This section includes examples to help you configure basic authentication for exporters and receivers.

### Forward signals to exporters

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

### Authenticating requests for receivers

These examples show how to perform basic authentication using the `client_auth` block for exporters or the `htpasswd` block for receivers.

#### Use client authentication

This example configures [`otelcol.exporter.otlp`][otelcol.exporter.otlp] to use basic authentication with a single username and password combination:

```alloy
otelcol.receiver.otlp "example" {
  grpc {
    endpoint = "127.0.0.1:4317"
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = "my-otlp-grpc-server:4317"
    auth = otelcol.auth.basic.creds.handler
  }
}

otelcol.auth.basic "creds" {
  client_auth {
    username = "demo"
    password = sys.env("API_KEY")
  }
}
```

{{< admonition type="note" >}}
To migrate from the deprecated `username` and `password` attributes, move them into the `client_auth` block for client authentication.
{{< /admonition >}}


#### Use htpasswd file

This example configures [`otelcol.receiver.otlp`][otelcol.receiver.otlp] to use basic authentication using an `htpasswd` file containing the users to use for basic authentication:

```alloy
otelcol.receiver.otlp "example" {
  grpc {
    endpoint = "127.0.0.1:4317"

    auth = otelcol.auth.basic.creds.handler
  }

  output {
    metrics = [otelcol.exporter.debug.default.input]
    logs    = [otelcol.exporter.debug.default.input]
    traces  = [otelcol.exporter.debug.default.input]
  }
}

otelcol.exporter.debug "default" {}

otelcol.auth.basic "creds" {
  htpasswd {
    file = "/etc/alloy/.htpasswd"
  }
}
```

#### Use htpasswd inline content

This example shows how to specify `htpasswd` content directly in the configuration:

```alloy
otelcol.receiver.otlp "example" {
  grpc {
    endpoint = "127.0.0.1:4317"

    auth = otelcol.auth.basic.creds.handler
  }

  output {
    metrics = [otelcol.exporter.debug.default.input]
    logs    = [otelcol.exporter.debug.default.input]
    traces  = [otelcol.exporter.debug.default.input]
  }
}

otelcol.exporter.debug "default" {}

otelcol.auth.basic "creds" {
  htpasswd {
    inline = "user1:password1\nuser2:password2"
  }
}
```

{{< admonition type="note" >}}
To make the migration from the deprecated `username` and `password` attributes easier, you can specify both the deprecated attributes and the `htpasswd` block in the same configuration.
{{< param "PRODUCT_NAME" >}} appends the deprecated attributes to the `htpasswd` content.

```alloy
otelcol.receiver.otlp "example" {
  grpc {
    endpoint = "127.0.0.1:4317"

    auth = otelcol.auth.basic.creds.handler
  }

  output {
    metrics = [otelcol.exporter.debug.default.input]
    logs    = [otelcol.exporter.debug.default.input]
    traces  = [otelcol.exporter.debug.default.input]
  }
}

otelcol.exporter.debug "default" {}

otelcol.auth.basic "creds" {
  username = "demo"
  password = sys.env("API_KEY")

  htpasswd {
    file = "/etc/alloy/.htpasswd"
  }
}
```
{{< /admonition >}}

[otelcol.receiver.otlp]: ../otelcol.receiver.otlp/
[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/

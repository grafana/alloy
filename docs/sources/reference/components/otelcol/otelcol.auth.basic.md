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
  
  htpasswd {
    file = "/etc/alloy/.htpasswd"
    inline = "<USERNAME>:<PASSWORD>"
  }
}
```

## Arguments

You can use the following arguments with `otelcol.auth.basic`:

| Name       | Type     | Description                                                                     | Default | Required |
|------------|----------|---------------------------------------------------------------------------------|---------|----------|
| `password` | `secret` | Password to use for basic authentication requests.                              |         | no       |
| `username` | `string` | Username to use for basic authentication requests.                              |         | no       |


## Blocks

You can use the following block with `otelcol.auth.basic`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |
| [`htpasswd`][htpasswd]           | Configures the service authentication for a receiver                       | no       |

[debug_metrics]: #debug_metrics
[htpasswd]: #htpasswd

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `htpasswd`

The `htpasswd` block configures how the server extensions will authenticate calls.

| Name     | Type     | Description                                                        | Default | Required |
|----------|----------|--------------------------------------------------------------------|---------|----------|
| `file`   | `string` | Path to the htpasswd file to use for basic authentication requests | `""`    | no       |
| `inline` | `string` | The htpasswd file inline content                                   | `""`    | no       |

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

#### Use Username/Password
This example configures [`otelcol.receiver.otlp`][otelcol.receiver.otlp] to use basic authentication using a single
username and password combination:

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
}
```

#### Use htpasswd file
This example configures [`otelcol.receiver.otlp`][otelcol.receiver.otlp] to use basic authentication using an htpasswd 
file containing the users to use for basic auth:

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

#### Combination of both
This example configures [`otelcol.receiver.otlp`][otelcol.receiver.otlp] to use basic authentication using a combination
of both an htpasswd file and username/password. Note that if the username provided also exists in the htpasswd file, it 
takes precedence over the one in the htpasswd file:

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


[otelcol.receiver.otlp]: ../otelcol.receiver.otlp/

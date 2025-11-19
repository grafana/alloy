---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.extension.jaeger_remote_sampling/
aliases:
  - ../otelcol.extension.jaeger_remote_sampling/ # /docs/alloy/latest/reference/otelcol.extension.jaeger_remote_sampling/
description: Learn about otelcol.extension.jaeger_remote_sampling
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.extension.jaeger_remote_sampling
---

# `otelcol.extension.jaeger_remote_sampling`

`otelcol.extension.jaeger_remote_sampling` serves a specified Jaeger remote sampling document.

{{< admonition type="note" >}}
`otelcol.extension.jaeger_remote_sampling` is a wrapper over the upstream OpenTelemetry Collector [`jaegerremotesampling`][] extension.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`jaegerremotesampling`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/jaegerremotesampling
{{< /admonition >}}

You can specify multiple `otelcol.extension.jaeger_remote_sampling` components by giving them different labels.

## Usage

```alloy
otelcol.extension.jaeger_remote_sampling "<LABEL>" {
  source {
  }
}
```

## Arguments

The `otelcol.extension.jaeger_remote_sampling` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `otelcol.extension.jaeger_remote_sampling`:

| Block                                                             | Description                                                                      | Required |
|-------------------------------------------------------------------|----------------------------------------------------------------------------------|----------|
| [`source`][source]                                                | Configures the Jaeger remote sampling document.                                  | yes      |
| `source` > [`remote`][remote]                                     | Configures the gRPC client used to retrieve the Jaeger remote sampling document. | no       |
| `source` > `remote` > [`keepalive` client][keepalive_client]      | Configures keepalive settings for the gRPC client.                               | no       |
| `source` > `remote` > [`tls` client][tls_client]                  | Configures TLS for the gRPC client.                                              | no       |
| [`http`][http]                                                    | Configures the HTTP server to serve Jaeger remote sampling.                      | no       |
| `http` > [`cors`][cors]                                           | Configures CORS for the HTTP server.                                             | no       |
| `http` > [`tls`][tls]                                             | Configures TLS for the HTTP server.                                              | no       |
| `http` > `tls` > [`tpm`][tpm]                                     | Configures TPM settings for the TLS key_file.                                    | no       |
| [`grpc`][grpc]                                                    | Configures the gRPC server to serve Jaeger remote sampling.                      | no       |
| `grpc` > [`keepalive`][keepalive]                                 | Configures keepalive settings for the configured server.                         | no       |
| `grpc` > `keepalive` > [`enforcement_policy`][enforcement_policy] | Enforcement policy for keepalive settings.                                       | no       |
| `grpc` > `keepalive` > [`server_parameters`][server_parameters]   | Server parameters used to configure keepalive settings.                          | no       |
| `grpc` > [`tls`][tls]                                             | Configures TLS for the gRPC server.                                              | no       |
| `grpc` > `tls` > [`tpm`][tpm]                                     | Configures TPM settings for the TLS key_file.                                    | no       |
| [`debug_metrics`][debug_metrics]                                  | Configures the metrics that this component generates to monitor its state.       | no       |

The > symbol indicates deeper levels of nesting.
For example, `grpc` > `tls` refers to a `tls` block defined inside a `grpc` block.

[http]: #http
[tls]: #tls
[tpm]: #tpm
[cors]: #cors
[grpc]: #grpc
[keepalive]: #keepalive
[server_parameters]: #server_parameters
[enforcement_policy]: #enforcement_policy
[source]: #source
[remote]: #remote
[tls_client]: #tls-client
[keepalive_client]: #keepalive-client
[debug_metrics]: #debug_metrics

### `source`

{{< badge text="Required" >}}

The `source` block configures the method of retrieving the Jaeger remote sampling document that's served by the servers specified in the `grpc` and `http` blocks.

The following arguments are supported:

| Name              | Type       | Description                                                                     | Default | Required |
|-------------------|------------|---------------------------------------------------------------------------------|---------|----------|
| `content`         | `string`   | A string containing the Jaeger remote sampling contents directly.               | `""`    | no       |
| `file`            | `string`   | A local file containing a Jaeger remote sampling document.                      | `""`    | no       |
| `reload_interval` | `duration` | The interval at which to reload the specified file. Leave at 0 to never reload. | `"0"`   | no       |

Exactly one of the `file` argument, `content` argument or `remote` block must be specified.

### `remote`

The `remote` block configures the gRPC client used by the component.

The following arguments are supported:

| Name                | Type                       | Description                                                                      | Default    | Required |
|---------------------|----------------------------|----------------------------------------------------------------------------------|------------|----------|
| `endpoint`          | `string`                   | `host:port` to send telemetry data to.                                           |            | yes      |
| `auth`              | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests.     |            | no       |
| `authority`         | `string`                   | Overrides the default `:authority` header in gRPC requests from the gRPC client. |            | no       |
| `compression`       | `string`                   | Compression mechanism to use for requests.                                       | `"gzip"`   | no       |
| `headers`           | `map(string)`              | Additional headers to send with the request.                                     | `{}`       | no       |
| `read_buffer_size`  | `string`                   | Size of the read buffer the gRPC client to use for reading server responses.     |            | no       |
| `wait_for_ready`    | `bool`                     | Waits for gRPC connection to be in the `READY` state before sending data.        | `false`    | no       |
| `write_buffer_size` | `string`                   | Size of the write buffer the gRPC client to use for writing requests.            | `"512KiB"` | no       |

{{< docs/shared lookup="reference/components/otelcol-compression-field.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< docs/shared lookup="reference/components/otelcol-grpc-balancer-name.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< docs/shared lookup="reference/components/otelcol-grpc-authority.md" source="alloy" version="<ALLOY_VERSION>" >}}

An HTTP proxy can be configured through the following environment variables:

* `HTTPS_PROXY`
* `NO_PROXY`

The `HTTPS_PROXY` environment variable specifies a URL to use for proxying requests.
Connections to the proxy are established via [the `HTTP CONNECT` method][HTTP CONNECT].

The `NO_PROXY` environment variable is an optional list of comma-separated hostnames for which the HTTPS proxy should _not_ be used.
Each hostname can be provided as an IP address (`1.2.3.4`), an IP address in CIDR notation (`1.2.3.4/8`), a domain name (`example.com`), or `*`.
A domain name matches that domain and all subdomains. A domain name with a leading "." (`.example.com`) matches subdomains only.
`NO_PROXY` is only read when `HTTPS_PROXY` is set.

Because `otelcol.extension.jaeger_remote_sampling` uses gRPC, the configured proxy server must be able to handle and proxy HTTP/2 traffic.

[HTTP CONNECT]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods/CONNECT

### `keepalive` client

This `keepalive` block configures keepalive settings for gRPC client connections.

The following arguments are supported:

| Name                    | Type       | Description                                                                               | Default | Required |
|-------------------------|------------|-------------------------------------------------------------------------------------------|---------|----------|
| `ping_wait`             | `duration` | How often to ping the server after no activity.                                           |         | no       |
| `ping_response_timeout` | `duration` | Time to wait before closing inactive connections if the server doesn't respond to a ping. |         | no       |
| `ping_without_stream`   | `boolean`  | Send pings even if there is no active stream request.                                     |         | no       |

### `tls` client

This `tls` block configures TLS settings used for the connection to the gRPC server.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `http`

The `http` block configures an HTTP server which serves the Jaeger remote sampling document.

The following arguments are supported:

| Name                     | Type                       | Description                                                                  | Default                                                    | Required |
|--------------------------|----------------------------|------------------------------------------------------------------------------|------------------------------------------------------------|----------|
| `auth`                   | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests. |                                                            | no       |
| `compression_algorithms` | `list(string)`             | A list of compression algorithms the server can accept.                      | `["", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"]` | no       |
| `endpoint`               | `string`                   | `host:port` to listen for traffic on.                                        | `"0.0.0.0:5778"`                                           | no       |
| `include_metadata`       | `boolean`                  | Propagate incoming connection metadata to downstream consumers.              |                                                            | no       |
| `keep_alives_enabled`    | `boolean`                  | Whether or not HTTP keep-alives are enabled                                  | `true`                                                     | no       |
| `max_request_body_size`  | `string`                   | Maximum request body size the server will allow.                             | `"20MiB"`                                                  | no       |

### `cors`

The `cors` block configures CORS settings for an HTTP server.

The following arguments are supported:

| Name              | Type           | Description                                              | Default                | Required |
|-------------------|----------------|----------------------------------------------------------|------------------------|----------|
| `allowed_headers` | `list(string)` | Accepted headers from CORS requests.                     | `["X-Requested-With"]` | no       |
| `allowed_origins` | `list(string)` | Allowed values for the `Origin` header.                  |                        | no       |
| `max_age`         | `number`       | Configures the `Access-Control-Max-Age` response header. |                        | no       |

The `allowed_headers` specifies which headers are acceptable from a CORS request.
The following headers are always implicitly allowed:

* `Accept`
* `Accept-Language`
* `Content-Type`
* `Content-Language`

If `allowed_headers` includes `"*"`, all headers will be permitted.

### `tls`

The `tls` block configures TLS settings used for a server. If the `tls` block
isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `grpc`

The `grpc` block configures a gRPC server which serves the Jaeger remote sampling document.

The following arguments are supported:

| Name                     | Type                       | Description                                                                  | Default           | Required |
|--------------------------|----------------------------|------------------------------------------------------------------------------|-------------------|----------|
| `auth`                   | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests. |                   | no       |
| `endpoint`               | `string`                   | `host:port` to listen for traffic on.                                        | `"0.0.0.0:14250"` | no       |
| `include_metadata`       | `boolean`                  | Propagate incoming connection metadata to downstream consumers.              |                   | no       |
| `max_concurrent_streams` | `number`                   | Limit the number of concurrent streaming RPC calls.                          |                   | no       |
| `max_recv_msg_size`      | `string`                   | Maximum size of messages the server will accept.                             | `"4MiB"`          | no       |
| `read_buffer_size`       | `string`                   | Size of the read buffer the gRPC server will use for reading from clients.   | `"512KiB"`        | no       |
| `transport`              | `string`                   | Transport to use for the gRPC server.                                        | `"tcp"`           | no       |
| `write_buffer_size`      | `string`                   | Size of the write buffer the gRPC server will use for writing to clients.    |                   | no       |

### `keepalive`

The `keepalive` block configures keepalive settings for connections to a gRPC server.

`keepalive` doesn't support any arguments and is configured fully through inner blocks.

### `enforcement_policy`

The `enforcement_policy` block configures the keepalive enforcement policy for gRPC servers.
The server will close connections from clients that violate the configured policy.

The following arguments are supported:

| Name                    | Type       | Description                                                             | Default | Required |
|-------------------------|------------|-------------------------------------------------------------------------|---------|----------|
| `min_time`              | `duration` | Minimum time clients should wait before sending a keepalive ping.       | `"5m"`  | no       |
| `permit_without_stream` | `boolean`  | Allow clients to send keepalive pings when there are no active streams. | `false` | no       |

### `server_parameters`

The `server_parameters` block controls keepalive and maximum age settings for gRPC servers.

The following arguments are supported:

| Name                       | Type       | Description                                                                         | Default      | Required |
|----------------------------|------------|-------------------------------------------------------------------------------------|--------------|----------|
| `max_connection_age_grace` | `duration` | Time to wait before forcibly closing connections.                                   | `"infinity"` | no       |
| `max_connection_age`       | `duration` | Maximum age for non-idle connections.                                               | `"infinity"` | no       |
| `max_connection_idle`      | `duration` | Maximum age for idle connections.                                                   | `"infinity"` | no       |
| `time`                     | `duration` | How often to ping inactive clients to check for liveness.                           | `"2h"`       | no       |
| `timeout`                  | `duration` | Time to wait before closing inactive clients that don't respond to liveness checks. | `"20s"`      | no       |

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`otelcol.extension.jaeger_remote_sampling` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.extension.jaeger_remote_sampling` doesn't expose any component-specific debug information.

## Examples

### Serve from a file

This example configures the Jaeger remote sampling extension to load a local JSON document and serve it over the default HTTP port of 5778.
Currently this configuration style exists for consistency with upstream OpenTelemetry Collector components and may be removed.

```alloy
otelcol.extension.jaeger_remote_sampling "example" {
  http {
  }
  source {
    file             = "/path/to/jaeger-sampling.json"
    reload_interval  = "10s"
  }
}
```

### Serve from another component

This example uses the output of a component to determine what sampling rules to serve:

```alloy
local.file "sampling" {
  filename  = "/path/to/jaeger-sampling.json"
}

otelcol.extension.jaeger_remote_sampling "example" {
  http {
  }
  source {
    content = local.file.sampling.content
  }
}
```

## Enable authentication

You can use `jaeger_remote_sampling` to authenticate requests.
This allows you to limit access to the sampling document.

{{< admonition type="note" >}}
Not all OpenTelemetry Collector authentication plugins support receiver authentication.
Refer to the [documentation](https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/otelcol/) for each `otelcol.auth.*` component to determine its compatibility.
{{< /admonition >}}

```alloy
otelcol.extension.jaeger_remote_sampling "default" {
  http {
    auth = otelcol.auth.basic.creds.handler
  }
  grpc {
     auth = otelcol.auth.basic.creds.handler
  }
}

otelcol.auth.basic "creds" {
    username = sys.env("USERNAME")
    password = sys.env("PASSWORD")
}
```

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.otlp/
aliases:
  - ../otelcol.receiver.otlp/ # /docs/alloy/latest/reference/otelcol.receiver.otlp/
description: Learn about otelcol.receiver.otlp
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.receiver.otlp
---

# `otelcol.receiver.otlp`

`otelcol.receiver.otlp` accepts OTLP-formatted data over the network and forwards it to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.otlp` is a wrapper over the upstream OpenTelemetry Collector [`otlp`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`otlp`]: https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/receiver/otlpreceiver
{{< /admonition >}}

Multiple `otelcol.receiver.otlp` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.receiver.otlp "<LABEL>" {
  grpc { ... }
  http { ... }

  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

The `otelcol.receiver.otlp` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `otelcol.receiver.otlp`:

| Block                                                             | Description                                                                | Required |
|-------------------------------------------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]                                                | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics]                                  | Configures the metrics that this component generates to monitor its state. | no       |
| [`grpc`][grpc]                                                    | Configures the gRPC server to receive telemetry data.                      | no       |
| `grpc` > [`keepalive`][keepalive]                                 | Configures keepalive settings for the configured server.                   | no       |
| `grpc` > `keepalive` > [`enforcement_policy`][enforcement_policy] | Enforcement policy for keepalive settings.                                 | no       |
| `grpc` > `keepalive` > [`server_parameters`][server_parameters]   | Server parameters used to configure keepalive settings.                    | no       |
| `grpc` > [`tls`][tls]                                             | Configures TLS for the gRPC server.                                        | no       |
| `grpc` > `tls` > [`tpm`][tpm]                                     | Configures TPM settings for the TLS key_file.                              | no       |
| [`http`][http]                                                    | Configures the HTTP server to receive telemetry data.                      | no       |
| `http` > [`cors`][cors]                                           | Configures CORS for the HTTP server.                                       | no       |
| `http` > [`tls`][tls]                                             | Configures TLS for the HTTP server.                                        | no       |
| `http` > `tls` > [`tpm`][tpm]                                     | Configures TPM settings for the TLS key_file.                              | no       |

The > symbol indicates deeper levels of nesting.
For example, `grpc` > `tls` refers to a `tls` block defined inside a `grpc` block.

[grpc]: #grpc
[tls]: #tls
[tpm]: #tpm
[keepalive]: #keepalive
[server_parameters]: #server_parameters
[enforcement_policy]: #enforcement_policy
[http]: #http
[cors]: #cors
[debug_metrics]: #debug_metrics
[output]: #output

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `grpc`

The `grpc` block configures the gRPC server used by the component.
If the `grpc` block isn't provided, a gRPC server isn't started.

The following arguments are supported:

| Name                     | Type                       | Description                                                                  | Default          | Required |
|--------------------------|----------------------------|------------------------------------------------------------------------------|------------------|----------|
| `auth`                   | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests. |                  | no       |
| `endpoint`               | `string`                   | `host:port` to listen for traffic on.                                        | `"0.0.0.0:4317"` | no       |
| `include_metadata`       | `bool`                     | Propagate incoming connection metadata to downstream consumers.              | `false`          | no       |
| `max_concurrent_streams` | `number`                   | Limit the number of concurrent streaming RPC calls.                          |                  | no       |
| `max_recv_msg_size`      | `string`                   | Maximum size of messages the server will accept.                             | `"4MiB"`         | no       |
| `read_buffer_size`       | `string`                   | Size of the read buffer the gRPC server will use for reading from clients.   | `"512KiB"`       | no       |
| `transport`              | `string`                   | Transport to use for the gRPC server.                                        | `"tcp"`          | no       |
| `write_buffer_size`      | `string`                   | Size of the write buffer the gRPC server will use for writing to clients.    |                  | no       |

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

### `tls`

The `tls` block configures TLS settings used for a server.
If the `tls` block isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `http`

The `http` block configures the HTTP server used by the component.
If the `http` block isn't specified, an HTTP server isn't started.

The following arguments are supported:

| Name                     | Type                       | Description                                                                  | Default                                                    | Required |
|--------------------------|----------------------------|------------------------------------------------------------------------------|------------------------------------------------------------|----------|
| `auth`                   | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests. |                                                            | no       |
| `compression_algorithms` | `list(string)`             | A list of compression algorithms the server can accept.                      | `["", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"]` | no       |
| `endpoint`               | `string`                   | `host:port` to listen for traffic on.                                        | `"0.0.0.0:4318"`                                           | no       |
| `include_metadata`       | `bool`                     | Propagate incoming connection metadata to downstream consumers.              | `false`                                                    | no       |
| `keep_alives_enabled`    | `boolean`                  | Whether or not HTTP keep-alives are enabled                                  | `true`                                                     | no       |
| `logs_url_path`          | `string`                   | The URL path to receive logs on.                                             | `"/v1/logs"`                                               | no       |
| `max_request_body_size`  | `string`                   | Maximum request body size the server will allow.                             | `"20MiB"`                                                  | no       |
| `metrics_url_path`       | `string`                   | The URL path to receive metrics on.                                          | `"/v1/metrics"`                                            | no       |
| `traces_url_path`        | `string`                   | The URL path to receive traces on.                                           | `"/v1/traces"`                                             | no       |

To send telemetry signals to `otelcol.receiver.otlp` with HTTP/JSON, POST to:

* `[endpoint][traces_url_path]` for traces.
* `[endpoint][metrics_url_path]` for metrics.
* `[endpoint][logs_url_path]` for logs.

### `cors`

The `cors` block configures CORS settings for an HTTP server.

The following arguments are supported:

| Name              | Type           | Description                                              | Default                | Required |
|-------------------|----------------|----------------------------------------------------------|------------------------|----------|
| `allowed_origins` | `list(string)` | Allowed values for the `Origin` header.                  |                        | no       |
| `allowed_headers` | `list(string)` | Accepted headers from CORS requests.                     | `["X-Requested-With"]` | no       |
| `max_age`         | `number`       | Configures the `Access-Control-Max-Age` response header. |                        | no       |

The `allowed_headers` argument specifies which headers are acceptable from a CORS request.
The following headers are always implicitly allowed:

* `Accept`
* `Accept-Language`
* `Content-Type`
* `Content-Language`

If `allowed_headers` includes `"*"`, all headers are permitted.

## Exported fields

`otelcol.receiver.otlp` doesn't export any fields.

## Component health

`otelcol.receiver.otlp` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.otlp` doesn't expose any component-specific debug information.

## Debug metrics

* `otelcol_receiver_accepted_spans_total` (counter): Number of spans successfully pushed into the pipeline.
* `otelcol_receiver_refused_spans_total` (counter): Number of spans that couldn't be pushed into the pipeline.
* `rpc_server_duration_milliseconds` (histogram): Duration of RPC requests from a gRPC server.
* `rpc_server_request_size_bytes` (histogram): Measures size of RPC request messages (uncompressed).
* `rpc_server_requests_per_rpc` (histogram): Measures the number of messages received per RPC. Should be 1 for all non-streaming RPCs.
* `rpc_server_response_size_bytes` (histogram): Measures size of RPC response messages (uncompressed).
* `rpc_server_responses_per_rpc` (histogram): Measures the number of messages received per RPC. Should be 1 for all non-streaming RPCs.

## Example

This example forwards received telemetry data through a batch processor before finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.otlp "default" {
  http {}
  grpc {}

  output {
    metrics = [otelcol.processor.batch.default.input]
    logs    = [otelcol.processor.batch.default.input]
    traces  = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

## Technical details

`otelcol.receiver.otlp` supports [Gzip](https://en.wikipedia.org/wiki/Gzip) for compression.

## Enable authentication

You can create a `otelcol.reciever.otlp` component that requires authentication for requests.
This is useful for limiting who can push data to the server.

{{< admonition type="note" >}}
Not all OpenTelemetry Collector authentication plugins support receiver authentication.
Refer to the [documentation](https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/otelcol/) for each `otelcol.auth.*` component to determine its compatibility.
{{< /admonition >}}

```alloy
otelcol.receiver.otlp "default" {
  http {
    auth = otelcol.auth.basic.creds.handler
  }
  grpc {
     auth = otelcol.auth.basic.creds.handler
  }

  output {
   ...
  }
}

otelcol.auth.basic "creds" {
    username = sys.env("<USERNAME>")
    password = sys.env("<PASSWORD>")
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.otlp` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

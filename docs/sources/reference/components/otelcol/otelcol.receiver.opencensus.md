---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.opencensus/
aliases:
  - ../otelcol.receiver.opencensus/ # /docs/alloy/latest/reference/otelcol.receiver.opencensus/
description: Learn about otelcol.receiver.opencensus
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.receiver.opencensus
---

# `otelcol.receiver.opencensus`

{{< admonition type="warning" >}}
The `otelcol.receiver.opencensus` component has been deprecated and will be removed in a future release.
Use `otelcol.reciver.otlp` instead.

{{< /admonition >}}

`otelcol.receiver.opencensus` accepts telemetry data via gRPC or HTTP using the [OpenCensus](https://opencensus.io/) format and forwards it to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.opencensus` is a wrapper over the upstream OpenTelemetry Collector [`opencensus`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`opencensus`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/opencensusreceiver
{{< /admonition >}}

You can specify multiple `otelcol.receiver.opencensus` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.opencensus "<LABEL>" {
  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.opencensus`:

| Name                     | Type                       | Description                                                                  | Default           | Required |
|--------------------------|----------------------------|------------------------------------------------------------------------------|-------------------|----------|
| `auth`                   | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests. |                   | no       |
| `cors_allowed_origins`   | `list(string)`             | A list of allowed Cross-Origin Resource Sharing (CORS) origins.              |                   | no       |
| `endpoint`               | `string`                   | `host:port` to listen for traffic on.                                        | `"0.0.0.0:55678"` | no       |
| `include_metadata`       | `bool`                     | Propagate incoming connection metadata to downstream consumers.              | `false`           | no       |
| `max_concurrent_streams` | `number`                   | Limit the number of concurrent streaming RPC calls.                          |                   | no       |
| `max_recv_msg_size`      | `string`                   | Maximum size of messages the server will accept.                             | `"4MiB"`          | no       |
| `read_buffer_size`       | `string`                   | Size of the read buffer the gRPC server will use for reading from clients.   | `"512KiB"`        | no       |
| `transport`              | `string`                   | Transport to use for the gRPC server.                                        | `"tcp"`           | no       |
| `write_buffer_size`      | `string`                   | Size of the write buffer the gRPC server will use for writing to clients.    |                   | no       |

`cors_allowed_origins` are the allowed [CORS](https://github.com/rs/cors) origins for HTTP/JSON requests.
An empty list means that CORS isn't enabled at all. A wildcard (*) can be used to match any origin or one or more characters of an origin.

The "endpoint" parameter is the same for both gRPC and HTTP/JSON, as the protocol is recognized and processed accordingly.

To write traces with HTTP/JSON, `POST` to `[address]/v1/trace`.
The JSON message format parallels the gRPC protobuf format.
For details, refer to its [OpenApi specification](https://github.com/census-instrumentation/opencensus-proto/blob/master/gen-openapi/opencensus/proto/agent/trace/v1/trace_service.swagger.json).

`max_recv_msg_size`, `read_buffer_size` and `write_buffer_size` are formatted in a way so that the units are included in the string, such as "512KiB" or "1024KB".

## Blocks

You can use the following blocks with `otelcol.receiver.opencensus`:

| Block                                                    | Description                                                                | Required |
|----------------------------------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]                                       | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics]                         | Configures the metrics that this component generates to monitor its state. | no       |
| [`keepalive`][keepalive]                                 | Configures keepalive settings for the configured server.                   | no       |
| `keepalive` > [`enforcement_policy`][enforcement_policy] | Enforcement policy for keepalive settings.                                 | no       |
| `keepalive` > [`server_parameters`][server_parameters]   | Server parameters used to configure keepalive settings.                    | no       |
| [`tls`][tls]                                             | Configures TLS for the gRPC server.                                        | no       |
| `tls` > [`tpm`][tpm]                                     | Configures TPM settings for the TLS key_file.                              | no       |

The > symbol indicates deeper levels of nesting.
For example, `keepalive` > `enforcesment_policy` refers to an `enforcement_policy` block defined inside a `keepalive` block.

[tls]: #tls
[tpm]: #tpm
[keepalive]: #keepalive
[server_parameters]: #server_parameters
[enforcement_policy]: #enforcement_policy
[debug_metrics]: #debug_metrics
[output]: #output

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `keepalive`

The `keepalive` block configures keepalive settings for connections to a gRPC server.

`keepalive` doesn't support any arguments and is configured fully through inner blocks.

### `enforcement_policy`

The `enforcement_policy` block configures the keepalive enforcement policy for gRPC servers.
The server closes connections from clients that violate the configured policy.

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

## Exported fields

`otelcol.receiver.opencensus` doesn't export any fields.

## Component health

`otelcol.receiver.opencensus` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.opencensus` doesn't expose any component-specific debug information.

## Example

This example forwards received telemetry data through a batch processor before finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.opencensus "default" {
    cors_allowed_origins = ["https://*.test.com", "https://test.com"]

    endpoint  = "0.0.0.0:9090"
    transport = "tcp"

    max_recv_msg_size      = "32KB"
    max_concurrent_streams = "16"
    read_buffer_size       = "1024KB"
    write_buffer_size      = "1024KB"
    include_metadata       = true

    tls {
        cert_file = "test.crt"
        key_file  = "test.key"
    }

    keepalive {
        server_parameters {
            max_connection_idle      = "11s"
            max_connection_age       = "12s"
            max_connection_age_grace = "13s"
            time                     = "30s"
            timeout                  = "5s"
        }

        enforcement_policy {
            min_time              = "10s"
            permit_without_stream = true
        }
    }

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

## Enable authentication

You can create a `otelcol.receiver.opencensus` component that requires authentication for requests.
This is useful for limiting who can push data to the server.

{{< admonition type="note" >}}
Not all OpenTelemetry Collector authentication plugins support receiver authentication.
Refer to the [documentation](https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/otelcol/) for each `otelcol.auth.*` component to determine its compatibility.
{{< /admonition >}}

```alloy
otelcol.receiver.opencensus "default" {
  auth = otelcol.auth.basic.creds.handler
}

otelcol.auth.basic "creds" {
    username = sys.env("USERNAME")
    password = sys.env("PASSWORD")
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.opencensus` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

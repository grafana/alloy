---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.opencensus/
aliases:
  - ../otelcol.receiver.opencensus/ # /docs/alloy/latest/reference/otelcol.receiver.opencensus/
description: Learn about otelcol.receiver.opencensus
title: otelcol.receiver.opencensus
---

# otelcol.receiver.opencensus

`otelcol.receiver.opencensus` accepts telemetry data via gRPC or HTTP
using the [OpenCensus](https://opencensus.io/) format and
forwards it to other `otelcol.*` components.

> **NOTE**: `otelcol.receiver.opencensus` is a wrapper over the upstream
> OpenTelemetry Collector `opencensus` receiver from the `otelcol-contrib`
> distribution. Bug reports or feature requests will be redirected to the
> upstream repository, if necessary.

Multiple `otelcol.receiver.opencensus` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.receiver.opencensus "LABEL" {
  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

`otelcol.receiver.opencensus` supports the following arguments:


Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`cors_allowed_origins` | `list(string)` | A list of allowed Cross-Origin Resource Sharing (CORS) origins. |  | no
`endpoint` | `string` | `host:port` to listen for traffic on. | `"localhost:55678"` | no
`transport` | `string` | Transport to use for the gRPC server. | `"tcp"` | no
`max_recv_msg_size` | `string` | Maximum size of messages the server will accept. | `"4MiB`" | no
`max_concurrent_streams` | `number` | Limit the number of concurrent streaming RPC calls. | | no
`read_buffer_size` | `string` | Size of the read buffer the gRPC server will use for reading from clients. | `"512KiB"` | no
`write_buffer_size` | `string` | Size of the write buffer the gRPC server will use for writing to clients. | | no
`include_metadata` | `boolean` | Propagate incoming connection metadata to downstream consumers. | | no

`cors_allowed_origins` are the allowed [CORS](https://github.com/rs/cors) origins for HTTP/JSON requests.
An empty list means that CORS is not enabled at all. A wildcard (*) can be
used to match any origin or one or more characters of an origin.

The "endpoint" parameter is the same for both gRPC and HTTP/JSON, as the protocol is recognized and processed accordingly.

To write traces with HTTP/JSON, `POST` to `[address]/v1/trace`. The JSON message format parallels the gRPC protobuf format. For details, refer to its [OpenApi specification](https://github.com/census-instrumentation/opencensus-proto/blob/master/gen-openapi/opencensus/proto/agent/trace/v1/trace_service.swagger.json).

Note that `max_recv_msg_size`, `read_buffer_size` and `write_buffer_size` are formatted in a way so that the units are included
in the string, such as "512KiB" or "1024KB".

## Blocks

The following blocks are supported inside the definition of
`otelcol.receiver.opencensus`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
tls | [tls][] | Configures TLS for the gRPC server. | no
keepalive | [keepalive][] | Configures keepalive settings for the configured server. | no
keepalive > server_parameters | [server_parameters][] | Server parameters used to configure keepalive settings. | no
keepalive > enforcement_policy | [enforcement_policy][] | Enforcement policy for keepalive settings. | no
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no
output | [output][] | Configures where to send received telemetry data. | yes

The `>` symbol indicates deeper levels of nesting. For example, `grpc > tls`
refers to a `tls` block defined inside a `grpc` block.

[tls]: #tls-block
[keepalive]: #keepalive-block
[server_parameters]: #server_parameters-block
[enforcement_policy]: #enforcement_policy-block
[debug_metrics]: #debug_metrics-block
[output]: #output-block

### tls block

The `tls` block configures TLS settings used for a server. If the `tls` block
isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### keepalive block

The `keepalive` block configures keepalive settings for connections to a gRPC
server.

`keepalive` doesn't support any arguments and is configured fully through inner
blocks.

### server_parameters block

The `server_parameters` block controls keepalive and maximum age settings for gRPC
servers.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`max_connection_idle` | `duration` | Maximum age for idle connections. | `"infinity"` | no
`max_connection_age` | `duration` | Maximum age for non-idle connections. | `"infinity"` | no
`max_connection_age_grace` | `duration` | Time to wait before forcibly closing connections. | `"infinity"` | no
`time` | `duration` | How often to ping inactive clients to check for liveness. | `"2h"` | no
`timeout` | `duration` | Time to wait before closing inactive clients that do not respond to liveness checks. | `"20s"` | no

### enforcement_policy block

The `enforcement_policy` block configures the keepalive enforcement policy for
gRPC servers. The server will close connections from clients that violate the
configured policy.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`min_time` | `duration` | Minimum time clients should wait before sending a keepalive ping. | `"5m"` | no
`permit_without_stream` | `boolean` | Allow clients to send keepalive pings when there are no active streams. | `false` | no

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.opencensus` does not export any fields.

## Component health

`otelcol.receiver.opencensus` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.receiver.opencensus` does not expose any component-specific debug
information.

## Example

This example forwards received telemetry data through a batch processor before
finally sending it to an OTLP-capable endpoint:

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
        endpoint = env("OTLP_ENDPOINT")
    }
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

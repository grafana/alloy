---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/otelcol.extension.jaeger_remote_sampling/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/otelcol.extension.jaeger_remote_sampling/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/otelcol.extension.jaeger_remote_sampling/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/otelcol.extension.jaeger_remote_sampling/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/otelcol.extension.jaeger_remote_sampling/
description: Learn about otelcol.extension.jaeger_remote_sampling
label:
  stage: experimental
title: otelcol.extension.jaeger_remote_sampling
---

# otelcol.extension.jaeger_remote_sampling

{{< docs/shared lookup="flow/stability/experimental.md" source="agent" version="<AGENT_VERSION>" >}}

`otelcol.extension.jaeger_remote_sampling` serves a specified Jaeger remote sampling
document.

> **NOTE**: `otelcol.extension.jaeger_remote_sampling` is a wrapper over the upstream OpenTelemetry
> Collector `jaegerremotesampling` extension. Bug reports or feature requests will be
> redirected to the upstream repository, if necessary.

Multiple `otelcol.extension.jaeger_remote_sampling` components can be specified by giving them
different labels.

## Usage

```river
otelcol.extension.jaeger_remote_sampling "LABEL" {
  source {
  }
}
```

## Arguments

`otelcol.extension.jaeger_remote_sampling` doesn't support any arguments and is configured fully
through inner blocks.

## Blocks

The following blocks are supported inside the definition of
`otelcol.extension.jaeger_remote_sampling`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
http | [http][] | Configures the http server to serve Jaeger remote sampling. | no
http > tls | [tls][] | Configures TLS for the HTTP server. | no
http > cors | [cors][] | Configures CORS for the HTTP server. | no
grpc | [grpc][] | Configures the grpc server to serve Jaeger remote sampling. | no
grpc > tls | [tls][] | Configures TLS for the gRPC server. | no
grpc > keepalive | [keepalive][] | Configures keepalive settings for the configured server. | no
grpc > keepalive > server_parameters | [server_parameters][] | Server parameters used to configure keepalive settings. | no
grpc > keepalive > enforcement_policy | [enforcement_policy][] | Enforcement policy for keepalive settings. | no
source | [source][] | Configures the Jaeger remote sampling document. | yes
source > remote | [remote][] | Configures the gRPC client used to retrieve the Jaeger remote sampling document. | no
source > remote > tls | [tls][] | Configures TLS for the gRPC client. | no
source > remote > keepalive | [keepalive][] | Configures keepalive settings for the gRPC client. | no

The `>` symbol indicates deeper levels of nesting. For example, `grpc > tls`
refers to a `tls` block defined inside a `grpc` block.

[http]: #http-block
[tls]: #tls-block
[cors]: #cors-block
[grpc]: #grpc-block
[keepalive]: #keepalive-block
[server_parameters]: #server_parameters-block
[enforcement_policy]: #enforcement_policy-block
[source]: #source-block
[remote]: #remote-block
[tls_client]: #tls-client-block
[keepalive_client]: #keepalive-client-block

### http block

The `http` block configures an HTTP server which serves the Jaeger remote 
sampling document.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`endpoint` | `string` | `host:port` to listen for traffic on. | `"0.0.0.0:5778"` | no
`max_request_body_size` | `string` | Maximum request body size the server will allow. No limit when unset. | | no
`include_metadata` | `boolean` | Propagate incoming connection metadata to downstream consumers. | | no

### tls block

The `tls` block configures TLS settings used for a server. If the `tls` block
isn't provided, TLS won't be used for connections to the server.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`ca_file` | `string` | Path to the CA file. | | no
`cert_file` | `string` | Path to the TLS certificate. | | no
`key_file` | `string` | Path to the TLS certificate key. | | no
`min_version` | `string` | Minimum acceptable TLS version for connections. | `"TLS 1.2"` | no
`max_version` | `string` | Maximum acceptable TLS version for connections. | `"TLS 1.3"` | no
`reload_interval` | `duration` | Frequency to reload the certificates. | | no
`client_ca_file` | `string` | Path to the CA file used to authenticate client certificates. | | no

### cors block

The `cors` block configures CORS settings for an HTTP server.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`allowed_origins` | `list(string)` | Allowed values for the `Origin` header. | | no
`allowed_headers` | `list(string)` | Accepted headers from CORS requests. | `["X-Requested-With"]` | no
`max_age` | `number` | Configures the `Access-Control-Max-Age` response header. | | no

The `allowed_headers` specifies which headers are acceptable from a CORS
request. The following headers are always implicitly allowed:

* `Accept`
* `Accept-Language`
* `Content-Type`
* `Content-Language`

If `allowed_headers` includes `"*"`, all headers will be permitted.

### grpc block

The `grpc` block configures a gRPC server which serves the Jaeger remote
 sampling document.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`endpoint` | `string` | `host:port` to listen for traffic on. | `"0.0.0.0:14250"` | no
`transport` | `string` | Transport to use for the gRPC server. | `"tcp"` | no
`max_recv_msg_size` | `string` | Maximum size of messages the server will accept. 0 disables a limit. | | no
`max_concurrent_streams` | `number` | Limit the number of concurrent streaming RPC calls. | | no
`read_buffer_size` | `string` | Size of the read buffer the gRPC server will use for reading from clients. | `"512KiB"` | no
`write_buffer_size` | `string` | Size of the write buffer the gRPC server will use for writing to clients. | | no
`include_metadata` | `boolean` | Propagate incoming connection metadata to downstream consumers. | | no

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

### source block

The `source` block configures the method of retrieving the Jaeger remote sampling document
that is served by the servers specified in the `grpc` and `http` blocks.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`file` | `string` | A local file containing a Jaeger remote sampling document. | `""` | no
`reload_interval` | `duration` | The interval at which to reload the specified file. Leave at 0 to never reload. | `0` | no
`content` | `string` | A string containing the Jaeger remote sampling contents directly. | `""` | no

Exactly one of the `file` argument, `content` argument or `remote` block must be specified. 

### remote block

The `remote` block configures the gRPC client used by the component.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`endpoint` | `string` | `host:port` to send telemetry data to. | | yes
`compression` | `string` | Compression mechanism to use for requests. | `"gzip"` | no
`read_buffer_size` | `string` | Size of the read buffer the gRPC client to use for reading server responses. | | no
`write_buffer_size` | `string` | Size of the write buffer the gRPC client to use for writing requests. | `"512KiB"` | no
`wait_for_ready` | `boolean` | Waits for gRPC connection to be in the `READY` state before sending data. | `false` | no
`headers` | `map(string)` | Additional headers to send with the request. | `{}` | no
`balancer_name` | `string` | Which gRPC client-side load balancer to use for requests. | `pick_first` | no
`authority` | `string` | Overrides the default `:authority` header in gRPC requests from the gRPC client. | | no
`auth` | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests. | | no

{{< docs/shared lookup="flow/reference/components/otelcol-compression-field.md" source="agent" version="<AGENT_VERSION>" >}}

{{< docs/shared lookup="flow/reference/components/otelcol-grpc-balancer-name.md" source="agent" version="<AGENT_VERSION>" >}}

{{< docs/shared lookup="flow/reference/components/otelcol-grpc-authority.md" source="agent" version="<AGENT_VERSION>" >}}

An HTTP proxy can be configured through the following environment variables:

* `HTTPS_PROXY`
* `NO_PROXY`

The `HTTPS_PROXY` environment variable specifies a URL to use for proxying
requests. Connections to the proxy are established via [the `HTTP CONNECT`
method][HTTP CONNECT].

The `NO_PROXY` environment variable is an optional list of comma-separated
hostnames for which the HTTPS proxy should _not_ be used. Each hostname can be
provided as an IP address (`1.2.3.4`), an IP address in CIDR notation
(`1.2.3.4/8`), a domain name (`example.com`), or `*`. A domain name matches
that domain and all subdomains. A domain name with a leading "."
(`.example.com`) matches subdomains only. `NO_PROXY` is only read when
`HTTPS_PROXY` is set.

Because `otelcol.extension.jaeger_remote_sampling` uses gRPC, the configured proxy server must be
able to handle and proxy HTTP/2 traffic.

[HTTP CONNECT]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods/CONNECT

### tls client block

The `tls` block configures TLS settings used for the connection to the gRPC
server.

{{< docs/shared lookup="flow/reference/components/otelcol-tls-config-block.md" source="agent" version="<AGENT_VERSION>" >}}

### keepalive client block

The `keepalive` block configures keepalive settings for gRPC client
connections.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`ping_wait` | `duration` | How often to ping the server after no activity. | | no
`ping_response_timeout` | `duration` | Time to wait before closing inactive connections if the server does not respond to a ping. | | no
`ping_without_stream` | `boolean` | Send pings even if there is no active stream request. | | no

## Component health

`otelcol.extension.jaeger_remote_sampling` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.extension.jaeger_remote_sampling` does not expose any component-specific debug information.

## Examples

### Serving from a file

This example configures the Jaeger remote sampling extension to load a local json document and
serve it over the default http port of 5778. Currently this config style exists for consistency
with upstream Opentelemetry Collector components and may be removed.

```river
otelcol.extension.jaeger_remote_sampling "example" {
  http {
  }
  source {
    file             = "/path/to/jaeger-sampling.json"
    reload_interval  = "10s"
  }
}
```

### Serving from another component


This example uses the output of a component to determine what sampling
rules to serve: 

```river
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

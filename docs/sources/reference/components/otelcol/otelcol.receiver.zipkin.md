---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol.receiver.zipkin/
description: Learn about otelcol.receiver.zipkin
title: otelcol.receiver.zipkin
---

# otelcol.receiver.zipkin

`otelcol.receiver.zipkin` accepts Zipkin-formatted traces over the network and
forwards it to other `otelcol.*` components.

> **NOTE**: `otelcol.receiver.zipkin` is a wrapper over the upstream
> OpenTelemetry Collector `zipkin` receiver. Bug reports or feature requests
> will be redirected to the upstream repository, if necessary.

Multiple `otelcol.receiver.zipkin` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.receiver.zipkin "LABEL" {
  output {
    traces = [...]
  }
}
```

## Arguments

`otelcol.receiver.zipkin` supports the following arguments:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`parse_string_tags` | `bool` | Parse string tags and binary annotations into non-string types. | `false` | no
`endpoint` | `string` | `host:port` to listen for traffic on. | `"0.0.0.0:9411"` | no
`max_request_body_size` | `string`   | Maximum request body size the server will allow.                   | `20MiB`          | no
`include_metadata` | `boolean` | Propagate incoming connection metadata to downstream consumers. | | no

If `parse_string_tags` is `true`, string tags and binary annotations are
converted to `int`, `bool`, and `float` if possible. String tags and binary
annotations that cannot be converted remain unchanged.

## Blocks

The following blocks are supported inside the definition of
`otelcol.receiver.zipkin`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
tls | [tls][] | Configures TLS for the HTTP server. | no
cors | [cors][] | Configures CORS for the HTTP server. | no
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no
output | [output][] | Configures where to send received traces. | yes

The `>` symbol indicates deeper levels of nesting. For example, `grpc > tls`
refers to a `tls` block defined inside a `grpc` block.

[tls]: #tls-block
[cors]: #cors-block
[debug_metrics]: #debug_metrics-block
[output]: #output-block

### tls block

The `tls` block configures TLS settings used for a server. If the `tls` block
isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### cors block

The `cors` block configures CORS settings for an HTTP server.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`allowed_origins` | `list(string)` | Allowed values for the `Origin` header. | | no
`allowed_headers` | `list(string)` | Accepted headers from CORS requests. | `["X-Requested-With"]` | no
`max_age` | `number` | Configures the `Access-Control-Max-Age` response header. | | no

The `allowed_headers` argument specifies which headers are acceptable from a
CORS request. The following headers are always implicitly allowed:

* `Accept`
* `Accept-Language`
* `Content-Type`
* `Content-Language`

If `allowed_headers` includes `"*"`, all headers are permitted.

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.zipkin` does not export any fields.

## Component health

`otelcol.receiver.zipkin` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.receiver.zipkin` does not expose any component-specific debug
information.

## Example

This example forwards received traces through a batch processor before finally
sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.zipkin "default" {
  output {
    traces = [otelcol.processor.batch.default.input]
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

`otelcol.receiver.zipkin` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
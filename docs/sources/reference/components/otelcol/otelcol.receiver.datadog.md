---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.datadog/
aliases:
  - ../otelcol.receiver.datadog/ # /docs/alloy/latest/reference/otelcol.receiver.datadog/
description: Learn about otelcol.receiver.datadog
title: otelcol.receiver.datadog
---

<span class="badge docs-labels__stage docs-labels__item">Experimental</span>

# otelcol.receiver.datadog

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.datadog` accepts Datadog metrics and traces over the network and forwards it to other `otelcol.*` components.

You can specify multiple `otelcol.receiver.datadog` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.datadog "LABEL" {
  output {
    metrics = [...]
    traces  = [...]
  }
}
```

## Arguments

`otelcol.receiver.datadog` supports the following arguments:

Name                     | Type       | Description                                                      | Default            | Required
------------------------  | ---------- | --------------------------------------------------------------- | ------------------ | --------
`endpoint`               | `string`   | `host:port` to listen for traffic on.                            | `"localhost:8126"` | no
`max_request_body_size`  | `string`   | Maximum request body size the server will allow.                 | `20MiB`            | no
`include_metadata`       | `boolean`  | Propagate incoming connection metadata to downstream consumers.  | `false`            | no
`read_timeout`           | `duration` | Read timeout for requests of the HTTP server.                    | `"60s"`            | no
`compression_algorithms` | `list(string)` | A list of compression algorithms the server can accept.      | `["", "gzip", "zstd", "zlib", "snappy", "deflate"]` | no

By default, `otelcol.receiver.datadog` listens for HTTP connections on `localhost`.
To expose the HTTP server to other machines on your network, configure `endpoint` with the IP address to listen on, or `0.0.0.0:8126` to listen on all network interfaces.

## Blocks

The following blocks are supported inside the definition of
`otelcol.receiver.datadog`:

Hierarchy     | Block             | Description                                                                | Required
------------- | ----------------- | -------------------------------------------------------------------------- | --------
tls           | [tls][]           | Configures TLS for the HTTP server.                                        | no
cors          | [cors][]          | Configures CORS for the HTTP server.                                       | no
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no
output        | [output][]        | Configures where to send received traces.                                  | yes

[tls]: #tls-block
[cors]: #cors-block
[debug_metrics]: #debug_metrics-block
[output]: #output-block

### tls block

The `tls` block configures TLS settings used for a server. If the `tls` block isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### cors block

The `cors` block configures CORS settings for an HTTP server.

The following arguments are supported:

Name              | Type           | Description                                              | Default                | Required
----------------- | -------------- | -------------------------------------------------------- | ---------------------- | --------
`allowed_origins` | `list(string)` | Allowed values for the `Origin` header.                  | `[]`                   | no
`allowed_headers` | `list(string)` | Accepted headers from CORS requests.                     | `["X-Requested-With"]` | no
`max_age`         | `number`       | Configures the `Access-Control-Max-Age` response header. | `0`                    | no

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

`otelcol.receiver.datadog` does not export any fields.

## Component health

`otelcol.receiver.datadog` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.receiver.datadog` does not expose any component-specific debug
information.

## Example

This example forwards received telemetry through a batch processor before finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.datadog "default" {
  output {
    metrics = [otelcol.processor.batch.default.input]
    traces  = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    metrics = [otelcol.exporter.otlp.default.input]
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

`otelcol.receiver.datadog` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

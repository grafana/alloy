---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.zipkin/
aliases:
  - ../otelcol.receiver.zipkin/ # /docs/alloy/latest/reference/otelcol.receiver.zipkin/
description: Learn about otelcol.receiver.zipkin
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.receiver.zipkin
---

# `otelcol.receiver.zipkin`

`otelcol.receiver.zipkin` accepts Zipkin-formatted traces over the network and forwards it to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.zipkin` is a wrapper over the upstream OpenTelemetry Collector [`zipkin`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`zipkin`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/zipkinreceiver
{{< /admonition >}}

You can specify multiple `otelcol.receiver.zipkin` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.zipkin "<LABEL>" {
  output {
    traces = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.zipkin`:

| Name                     | Type                       | Description                                                                  | Default                                                    | Required |
|--------------------------|----------------------------|------------------------------------------------------------------------------|------------------------------------------------------------|----------|
| `parse_string_tags`      | `bool`                     | Parse string tags and binary annotations into non-string types.              | `false`                                                    | no       |
| `endpoint`               | `string`                   | `host:port` to listen for traffic on.                                        | `"0.0.0.0:9411"`                                           | no       |
| `max_request_body_size`  | `string`                   | Maximum request body size the server will allow.                             | `"20MiB"`                                                  | no       |
| `include_metadata`       | `bool`                     | Propagate incoming connection metadata to downstream consumers.              | `false`                                                    | no       |
| `compression_algorithms` | `list(string)`             | A list of compression algorithms the server can accept.                      | `["", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"]` | no       |
| `auth`                   | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests. |                                                            | no       |

If `parse_string_tags` is `true`, string tags and binary annotations are converted to `int`, `bool`, and `float` if possible.
String tags and binary annotations that can't be converted remain unchanged.

## Blocks

You can use the following blocks with `otelcol.receiver.zipkin`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send received traces.                                  | yes      |
| [`cors`][cors]                   | Configures CORS for the HTTP server.                                       | no       |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |
| [`tls`][tls]                     | Configures TLS for the HTTP server.                                        | no       |
| `tls` > [`tpm`][tpm]             | Configures TPM settings for the TLS key_file.                              | no       |

The > symbol indicates deeper levels of nesting.
For example, `tls` > `tpm` refers to a `tpm` block defined inside a `tls` block.

[tls]: #tls
[tpm]: #tpm
[cors]: #cors
[debug_metrics]: #debug_metrics
[output]: #output

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `cors`

The `cors` block configures CORS settings for an HTTP server.

The following arguments are supported:

| Name              | Type           | Description                                              | Default                | Required |
|-------------------|----------------|----------------------------------------------------------|------------------------|----------|
| `allowed_headers` | `list(string)` | Accepted headers from CORS requests.                     | `["X-Requested-With"]` | no       |
| `allowed_origins` | `list(string)` | Allowed values for the `Origin` header.                  |                        | no       |
| `max_age`         | `number`       | Configures the `Access-Control-Max-Age` response header. |                        | no       |

The `allowed_headers` argument specifies which headers are acceptable from a CORS request.
The following headers are always implicitly allowed:

* `Accept`
* `Accept-Language`
* `Content-Type`
* `Content-Language`

If `allowed_headers` includes `"*"`, all headers are permitted.

### `tls`

The `tls` block configures TLS settings used for a server.
If the `tls` block isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.zipkin` doesn't export any fields.

## Component health

`otelcol.receiver.zipkin` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.zipkin` doesn't expose any component-specific debug information.

## Example

This example forwards received traces through a batch processor before finally sending it to an OTLP-capable endpoint:

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
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

## Enable authentication

You can create a `otelcol.receiver.zipkin` component that requires authentication for requests.
This is useful for limiting who can push data to the server.

{{< admonition type="note" >}}
Not all OpenTelemetry Collector authentication plugins support receiver authentication.
Refer to the [documentation](https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/otelcol/) for each `otelcol.auth.*` component to determine its compatibility.
{{< /admonition >}}

```alloy
otelcol.receiver.zipkin "default" {
  auth = otelcol.auth.basic.creds.handler
}

otelcol.auth.basic "creds" {
    username = sys.env("<USERNAME>")
    password = sys.env("<PASSWORD>")
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.zipkin` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
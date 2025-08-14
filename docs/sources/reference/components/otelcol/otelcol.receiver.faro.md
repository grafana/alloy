---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.faro/
description: Learn about otelcol.receiver.faro
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.faro
---

# `otelcol.receiver.faro`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.faro` accepts telemetry data from the [Grafana Faro Web SDK][faro-sdk] and forwards it to other `otelcol.*` components.

You can specify multiple `otelcol.receiver.faro` components by giving them different labels.

{{< docs/shared lookup="reference/components/otelcol-faro-component-note.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< admonition type="note" >}}
`otelcol.receiver.faro` is a wrapper over the upstream OpenTelemetry Collector [`faro`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`faro`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/faroreceiver
{{< /admonition >}}

## Usage

```alloy
otelcol.receiver.faro "<LABEL>" {
  output {
    logs   = [...]
    traces = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.faro`:

| Name                     | Type                       | Description                                                                  | Default                                                    | Required |
|--------------------------|----------------------------|------------------------------------------------------------------------------|------------------------------------------------------------|----------|
| `endpoint`               | `string`                   | `host:port` to listen for traffic on.                                        | `"localhost:8080"`                                         | no       |
| `max_request_body_size`  | `string`                   | Maximum request body size the server will allow.                             | `"20MiB"`                                                  | no       |
| `include_metadata`       | `bool`                     | Propagate incoming connection metadata to downstream consumers.              | `false`                                                    | no       |
| `read_timeout`           | `duration`                 | Read timeout for requests of the HTTP server.                                | `"60s"`                                                    | no       |
| `compression_algorithms` | `list(string)`             | A list of compression algorithms the server can accept.                      | `["", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"]` | no       |
| `auth`                   | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests. |                                                            | no       |

By default, `otelcol.receiver.faro` listens for HTTP connections on `localhost`.
To expose the HTTP server to other machines on your network, configure `endpoint` with the IP address to listen on, or `0.0.0.0:8080` to listen on all network interfaces.

## Blocks

You can use the following blocks with `otelcol.receiver.faro`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
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

### `cors`

The `cors` block configures CORS settings for an HTTP server.

The following arguments are supported:

| Name              | Type           | Description                                              | Default                | Required |
|-------------------|----------------|----------------------------------------------------------|------------------------|----------|
| `allowed_origins` | `list(string)` | Allowed values for the `Origin` header.                  | `[]`                   | no       |
| `allowed_headers` | `list(string)` | Accepted headers from CORS requests.                     | `["X-Requested-With"]` | no       |
| `max_age`         | `number`       | Configures the `Access-Control-Max-Age` response header. | `0`                    | no       |

The `allowed_headers` argument specifies which headers are acceptable from a CORS request.
The following headers are always implicitly allowed:

* `Accept`
* `Accept-Language`
* `Content-Type`
* `Content-Language`

If `allowed_headers` includes `"*"`, all headers are permitted.

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS settings used for a server.
If the `tls` block isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.faro` doesn't export any fields.

## Component health

`otelcol.receiver.faro` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.faro` doesn't expose any component-specific debug information.

## Examples

This example forwards received telemetry through a batch processor before finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.faro "default" {
  output {
    logs   = [otelcol.processor.batch.default.input]
    traces = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    logs   = [otelcol.exporter.faro.default.input]
    traces = [otelcol.exporter.faro.default.input]
  }
}

otelcol.exporter.faro "default" {
  client {
    endpoint = "<FARO_COLLECTOR_ADDRESS>"
  }
}
```

Replace the following:

* _`<FARO_COLLECTOR_ADDRESS>`_: The address of the Faro-compatible server to send data to.

### Enable TLS

You can configure `otelcol.receiver.faro` to use TLS for added security:

```alloy
otelcol.receiver.faro "default" {
  endpoint = "localhost:8443"
  
  tls {
    cert_file = "/path/to/cert.pem"
    key_file  = "/path/to/key.pem"
  }
  
  output {
    logs   = [otelcol.processor.batch.default.input]
    traces = [otelcol.processor.batch.default.input]
  }
}
```

### Enable authentication

You can create a `otelcol.receiver.faro` component that requires authentication for requests.
This is useful for limiting who can push data to the server.

{{< admonition type="note" >}}
Not all OpenTelemetry Collector authentication plugins support receiver authentication.
Refer to the [documentation](https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/otelcol/) for each `otelcol.auth.*` component to determine its compatibility.
{{< /admonition >}}

```alloy
otelcol.receiver.faro "default" {
  auth     = otelcol.auth.basic.creds.handler
  
  output {
    logs   = [otelcol.processor.batch.default.input]
    traces = [otelcol.processor.batch.default.input]
  }
}

otelcol.auth.basic "creds" {
  username = sys.env("FARO_USERNAME")
  password = sys.env("FARO_PASSWORD")
}
```

### Configure CORS

You can configure CORS settings to allow web applications to send telemetry data from browsers:

```alloy
otelcol.receiver.faro "default" {
  cors {
    allowed_origins = ["https://my-webapp.example.com", "https://staging.example.com"]
    allowed_headers = ["Content-Type", "X-Custom-Header"]
    max_age         = 3600
  }
  
  output {
    logs   = [otelcol.processor.batch.default.input]
    traces = [otelcol.processor.batch.default.input]
  }
}
```

[faro-sdk]: https://github.com/grafana/faro-web-sdk 

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.faro` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

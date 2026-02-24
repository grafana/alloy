---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.splunkhec/
description: Learn about otelcol.receiver.splunkhec
labels:
  stage: public-preview
  products:
    - oss
title: otelcol.receiver.splunkhec
---

# `otelcol.receiver.splunkhec`

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.splunkhec` accepts events in the [Splunk HEC format](https://docs.splunk.com/Documentation/Splunk/8.0.5/Data/FormateventsforHTTPEventCollector) and forwards them to other `otelcol.*` components.
The receiver accepts data formatted as JSON HEC events under any path or as EOL separated log raw data if sent to the `raw_path` path.

{{< admonition type="note" >}}
`otelcol.receiver.splunkhec` is a wrapper over the upstream OpenTelemetry Collector [`splunkhec`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`splunkhec`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/splunkhecreceiver
{{< /admonition >}}

You can specify multiple `otelcol.receiver.splunkhec` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.splunkhec "<LABEL>" {
  output {
    metrics = [...]
    logs    = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.splunkhec`:

| Name                       | Type                       | Description                                                                                                    | Default                                                    | Required |
|----------------------------|----------------------------|----------------------------------------------------------------------------------------------------------------|------------------------------------------------------------|----------|
| `access_token_passthrough` | `bool`                     | If enabled preserves incoming access token as a attribute `com.splunk.hec.access_token`                        | `false`                                                    | no       |
| `auth`                     | `capsule(otelcol.Handler)` | Handler from an `otelcol.auth` component to use for authenticating requests.                                   |                                                            | no       |
| `compression_algorithms`   | `list(string)`             | A list of compression algorithms the server can accept.                                                        | `["", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"]` | no       |
| `endpoint`                 | `string`                   | `host:port` to listen for traffic on.                                                                          | `"localhost:8088"`                                         | no       |
| `health_path`              | `string`                   | The path reporting health checks.                                                                              | `"/services/collector/health"`                             | no       |
| `include_metadata`         | `bool`                     | Propagate incoming connection metadata to downstream consumers.                                                | `false`                                                    | no       |
| `keep_alives_enabled`      | `boolean`                  | Whether or not HTTP keep-alives are enabled                                                                    | `true`                                                     | no       |
| `max_request_body_size`    | `string`                   | Maximum request body size the server will allow.                                                               | `"20MiB"`                                                  | no       |
| `raw_path`                 | `string`                   | The path accepting raw HEC events. Only applies when the receiver is used for logs.                            | `"/services/collector/raw"`                                | no       |
| `splitting`                | `string`                   | Defines the splitting strategy used by the receiver when ingesting raw events. Can be set to "line" or "none". | `"line"`                                                   | no       |

By default, `otelcol.receiver.splunkhec` listens for HTTP connections on `localhost:8088`.
To expose the HTTP server to other machines on your network, configure `endpoint` with the IP address to listen on, or `0.0.0.0:8088` to listen on all network interfaces.

If `access_token_passthrough` is enabled it will be preserved as a attribute `com.splunk.hec.access_token`.
If logs or metrics are exported with `otelcol.exporter.splunkhec` it will check for this attribute and if present forward it with outgoing request.

## Blocks

You can use the following blocks with `otelcol.receiver.splunkhec`:

| Block                                                      | Description                                                                | Required |
| ---------------------------------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`output`][output]                                         | Configures where to send received telemetry data.                          | yes      |
| [`cors`][cors]                                             | Configures CORS for the HTTP server.                                       | no       |
| [`debug_metrics`][debug_metrics]                           | Configures the metrics that this component generates to monitor its state. | no       |
| [`hec_metadata_to_otel_attrs`][hec_metadata_to_otel_attrs] | Configures OpenTelemetry attributes from HEC metadata.                     | no       |
| [`tls`][tls]                                               | Configures TLS for the HTTP server.                                        | no       |
| `tls` > [`tpm`][tpm]                                       | Configures TPM settings for the TLS `key_file`.                            | no       |

The > symbol indicates deeper levels of nesting.
For example, `tls` > `tpm` refers to a `tpm` block defined inside a `tls` block.

[tls]: #tls
[tpm]: #tpm
[cors]: #cors
[debug_metrics]: #debug_metrics
[output]: #output
[hec_metadata_to_otel_attrs]: #hec_metadata_to_otel_attrs

### `output`

{{< badge text="Required" >}}

The `output` block configures a set of components to forward resulting telemetry data to.

The following arguments are supported:

| Name      | Type                     | Description                           | Default | Required |
|-----------|--------------------------|---------------------------------------|---------|----------|
| `logs`    | `list(otelcol.Consumer)` | List of consumers to send logs to.    | `[]`    | no       |
| `metrics` | `list(otelcol.Consumer)` | List of consumers to send metrics to. | `[]`    | no       |

You must specify the `output` block, but all its arguments are optional.
By default, telemetry data is dropped.
Configure the `metrics` and `logs` arguments accordingly to send telemetry data to other components.

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

If `allowed_headers` includes `"*"`, all headers are permitted.

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `hec_metadata_to_otel_attrs`

The `hec_metadata_to_otel_attrs` block configures OpenTelemetry attributes from HEC metadata.

| Name         | Type     | Description                                                   | Default                 | Required |
|--------------|----------|---------------------------------------------------------------|-------------------------|----------|
| `host`       | `string` | Specifies the mapping of the host field to a attribute.       | `host.name`             | no       |
| `index`      | `string` | Specifies the mapping of the index field to a attribute.      | `com.splunk.index`      | no       |
| `source`     | `string` | Specifies the mapping of the source field to a attribute.     | `com.splunk.source`     | no       |
| `sourcetype` | `string` | Specifies the mapping of the sourcetype field to a attribute. | `com.splunk.sourcetype` | no       |

### `tls`

The `tls` block configures TLS settings used for a server.
If the `tls` block isn't provided, TLS isn't used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.splunkhec` doesn't export any fields.

## Component health

`otelcol.receiver.splunkhec` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.splunkhec` doesn't expose any component-specific debug information.

## Example

This example forwards received telemetry through a batch processor before finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.splunkhec "default" {
  output {
    logs    = [otelcol.processor.batch.default.input]
    metrics = [otelcol.processor.batch.default.input]
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
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

## Enable authentication

You can create a `otelcol.receiver.splunkhec` component that requires authentication for requests. This is useful for limiting who can push data to the server.

{{< admonition type="note" >}}
Not all OpenTelemetry Collector authentication plugins support receiver authentication.
Refer to the [documentation](https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/otelcol/) for each `otelcol.auth.*` component to determine its compatibility.
{{< /admonition >}}

```alloy
otelcol.receiver.splunkhec "default" {
  output {
    logs    = [otelcol.processor.batch.default.input]
    metrics = [otelcol.processor.batch.default.input]
  }
  auth = otelcol.auth.basic.creds.handler
}

otelcol.auth.basic "creds" {
    username = sys.env("<USERNAME>")
    password = sys.env("<PASSWORD>")
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.splunkhec` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

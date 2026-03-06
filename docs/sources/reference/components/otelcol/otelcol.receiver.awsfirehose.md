---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.awsfirehose/
description: Learn about otelcol.receiver.awsfirehose
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.awsfirehose
---

# `otelcol.receiver.awsfirehose`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.awsfirehose` receives metrics and logs from Amazon Data Firehose and forwards them to other `otelcol.*` components.

Make sure the receiver is accessible by AWS on port 443.

You can specify multiple `otelcol.receiver.awsfirehose` components by giving them different labels.

{{< admonition type="note" >}}
`otelcol.receiver.awsfirehose` is a wrapper over the upstream OpenTelemetry Collector [`awsfirehose`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`awsfirehose`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/awsfirehose
{{< /admonition >}}

## Usage

```alloy
otelcol.receiver.awsfirehose "<LABEL>" {
  endpoint = "HOST:PORT"
  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.awsfirehose`:

| Name                      | Type           | Description                                                                | Default                                                     | Required |
|---------------------------|----------------|----------------------------------------------------------------------------|-------------------------------------------------------------|----------|
| `access_key`              | `secret`       | The access key to be checked on each request received.                     |                                                             | no       |
| `compression_algorithms`  | `list(string)` | A list of compression algorithms the server can accept.                    | `["", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"]` | no       |
| `encoding`                | `string`       | Encoding of Firehose records.                                              |                                                             | no       |
| `endpoint`                | `string`       | `host:port` to listen for traffic on.                                      | `"0.0.0.0:4433"`                                            | no       |
| `include_metadata`        | `bool`         | Propagate incoming connection metadata to downstream consumers.            | `false`                                                     | no       |
| `keep_alives_enabled`     | `boolean`      | Whether or not HTTP keep-alives are enabled.                               | `true`                                                      | no       |
| `max_request_body_size`   | `string`       | Maximum request body size the HTTP server will allow. No limit when unset. |                                                             | no       |

`access_key` can be set when creating or updating the delivery stream. See the [AWS Firehose documentation](https://docs.aws.amazon.com/firehose/latest/dev/create-destination.html#create-destination-http) for more details.
Although the generic `auth` argument is available, it isn't needed here. AWS Firehose authenticates by sending an `access_key` in each request, making `access_key` the recommended way to secure this receiver.

If `encoding` is not set, the receiver defaults to a signal-specific encoding: `cwmetrics` for metrics and `cwlogs` for logs.

The supported values for `encoding` are:

* `cwmetrics`: The JSON encoding for CloudWatch metric streams (metrics only). See the [CloudWatch metric stream JSON documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-metric-streams-formats-json.html) for details.
* `otlp_v1`: The OpenTelemetry 1.0.0 encoding for CloudWatch metric streams (metrics only). See the [CloudWatch metric streams OpenTelemetry documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-metric-streams-formats-opentelemetry-100.html) for details.
* `cwlogs`: The encoding for CloudWatch log streams delivered via Firehose (logs only). See the [CloudWatch Logs via Firehose documentation](https://docs.aws.amazon.com/firehose/latest/dev/writing-with-cloudwatch-logs.html) for details.

## Blocks

You can use the following blocks with `otelcol.receiver.awsfirehose`:

| Block                            | Description                                                                | Required |
| -------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`cors`][cors]                   | Configures CORS for the HTTP server.                                       | no       |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |
| [`tls`][tls]                     | Configures TLS for the HTTP server.                                        | no       |
| `tls` > [`tpm`][tpm]             | Configures TPM settings for the TLS `key_file`.                            | no       |

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

`otelcol.receiver.awsfirehose` doesn't export any fields.

## Component health

`otelcol.receiver.awsfirehose` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.receiver.awsfirehose` doesn't expose any component-specific debug
information.

## Example

This example forwards received metrics through a batch processor before finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.awsfirehose "default" {
  endpoint = "0.0.0.0:4433"
  output {
    metrics = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    metrics = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = sys.env("OTLP_ENDPOINT")
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.awsfirehose` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol.exporter.awss3/
description: Learn about otelcol.exporter.awss3
title: otelcol.exporter.awss3
---

# otelcol.exporter.awss3

`otelcol.exporter.awss3` accepts telemetry data from other `otelcol` components
and writes them to an AWS S3 Bucket.

> **NOTE**: `otelcol.exporter.awss3` is a wrapper over the upstream
> OpenTelemetry Collector Contrib `awss3` exporter. Bug reports or feature requests will
> be redirected to the upstream repository, if necessary.

Multiple `otelcol.exporter.awss3` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.exporter.awss3 "LABEL" {
  s3_uploader {
    region = "REGION"
    s3_bucket = "BUCKET_NAME"
    s3_prefix = "PREFIX"
  }
}
```

## Arguments

`otelcol.exporter.awss3` supports the following arguments:

Name       | Type       | Description                                      | Default | Required
-----------|------------|--------------------------------------------------|---------|---------
`encoding` | `string`   | Encoding extension to use to marshal data. Overrides the `marshaler` configuration option if set. | `""`  | no
`encoding_file_ext` | `string` | file format extension suffix when using the `encoding` configuration option. May be left empty for no suffix to be appended. | `""` | no

## Blocks

The following blocks are supported inside the definition of
`otelcol.exporter.awss3`:

Hierarchy              | Block                | Description                                                                          | Required
-----------------------|----------------------|--------------------------------------------------------------------------------------|---------
s3_uploader            | [s3_uploader][]      | Configures the AWS S3 bucket details to send telemetry data to.                      | yes
marshaler              | [marshaler][]        | Marshaler used to produce output data.                                               | no
debug_metrics          | [debug_metrics][]    | Configures the metrics that this component generates to monitor its state.           | no

The `>` symbol indicates deeper levels of nesting. For example, `client > tls`
refers to a `tls` block defined inside a `client` block.

[s3_uploader]: #s3_uploader-block
[marshaler]: #marshaler-block
[debug_metrics]: #debug_metrics-block

### s3_uploader block

The `s3_uploader` block configures the AWS S3 bucket details used by the component.

The following arguments are supported:

Name                  | Type                       | Description                                                                      | Default      | Required
----------------------|----------------------------|----------------------------------------------------------------------------------|--------------|---------
`region`              | `string`                   | AWS region.                                                                      | `"us-east-1"`| no
`s3_bucket`           | `string`                   | S3 bucket                                                                        |              | yes
`s3_prefix`           | `string`                   | Prefix for the S3 key (root directory inside bucket).                            |              | yes
`s3_partition`        | `string`                   | Time granularity of S3 key: hour or minute                                       | `"minute"`   | no
`role_arn`            | `string`                   | The Role ARN to be assumed                                                       |              | no
`file_prefix`         | `string`                   | File prefix defined by user                                                      |              | no
`endpoint`            | `string`                   | Overrides the endpoint used by the exporter instead of constructing it from `region` and `s3_bucket` |      | no
`s3_force_path_style` | `boolean`                  | [Set this to `true` to force the request to use path-style addressing](http://docs.aws.amazon.com/AmazonS3/latest/dev/VirtualHosting.html) | `false`             | no
`disable_ssl`         | `boolean`                  | Set this to `true` to disable SSL when sending requests           |              | `false`
`compression`         | `string`                   | should the file be compressed                                                    | `none`      | no

### marshaler block

Marshaler determines the format of data sent to AWS S3. Currently, the following marshalers are implemented:

- `otlp_json` (default): the [OpenTelemetry Protocol format](https://github.com/open-telemetry/opentelemetry-proto), represented as json.
- `otlp_proto`: the [OpenTelemetry Protocol format](https://github.com/open-telemetry/opentelemetry-proto), represented as Protocol Buffers. A single protobuf message is written into each object.
- `sumo_ic`: the [Sumo Logic Installed Collector Archive format](https://help.sumologic.com/docs/manage/data-archiving/archive/).
  **This format is supported only for logs.**
- `body`: export the log body as string.
  **This format is supported only for logs.**

The following arguments are supported:

Name                    | Type       | Description                                                                                | Default | Required
------------------------|------------|--------------------------------------------------------------------------------------------|---------|---------
`type`                  | `string`   | Marshaler used to produce output data                                                      | `"otlp_json"`   | no

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### Encoding

Encoding overrides marshaler if present and sets to use an encoding extension defined in the collector configuration.

See [https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/encoding](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/encoding).

### Compression

- `none` (default): No compression will be applied
- `gzip`: Files will be compressed with gzip. **This does not support `sumo_ic`marshaler.**

## Component health

`otelcol.exporter.awss3` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.exporter.awss3` does not expose any component-specific debug
information.

## Debug metrics

* `exporter_sent_spans_ratio_total` (counter): Number of spans successfully sent to destination.
* `exporter_send_failed_spans_ratio_total` (counter): Number of spans in failed attempts to send to destination.
* `exporter_queue_capacity_ratio` (gauge): Fixed capacity of the retry queue (in batches)
* `exporter_queue_size_ratio` (gauge): Current size of the retry queue (in batches)
* `rpc_client_duration_milliseconds` (histogram): Measures the duration of inbound RPC.
* `rpc_client_request_size_bytes` (histogram): Measures size of RPC request messages (uncompressed).
* `rpc_client_requests_per_rpc` (histogram): Measures the number of messages received per RPC. Should be 1 for all non-streaming RPCs.
* `rpc_client_response_size_bytes` (histogram): Measures size of RPC response messages (uncompressed).
* `rpc_client_responses_per_rpc` (histogram): Measures the number of messages received per RPC. Should be 1 for all non-streaming RPCs.

## Examples

The following examples show you how to create an exporter to send data to different destinations.

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.awss3/
aliases:
  - ../otelcol.exporter.awss3/ # /docs/alloy/latest/reference/components/otelcol.exporter.awss3/
description: Learn about otelcol.exporter.awss3
labels:
  stage: experimental
  products:
    - oss
title: otelcol.exporter.awss3
---

# `otelcol.exporter.awss3`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.awss3` accepts telemetry data from other `otelcol` components and writes them to an AWS S3 bucket.

{{< admonition type="note" >}}
`otelcol.exporter.awss3` is a wrapper over the upstream OpenTelemetry Collector [`awss3`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository if necessary.

[`awss3`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/awss3exporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.awss3` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.awss3 "<LABEL>" {
  s3_uploader {
    region = "<AWS_REGION>"
    s3_bucket = "<S3_BUCKET_NAME>"
    s3_prefix = "<PREFIX_FOR_S3_KEY>"
  }
}
```

## Arguments

You can use the following argument with `otelcol.exporter.awss3`:

| Name      | Type       | Description                                      | Default | Required |
|-----------|------------|--------------------------------------------------|---------|----------|
| `timeout` | `duration` | Time to wait before marking a request as failed. | `"5s"`  | no       |

## Blocks

You can use the following blocks with `otelcol.exporter.awss3`:

| Block                                          | Description                                                                                              | Required |
|------------------------------------------------|----------------------------------------------------------------------------------------------------------|----------|
| [`s3_uploader`][s3_uploader]                   | Configures the AWS S3 bucket details to send telemetry data to.                                          | yes      |
| [`debug_metrics`][debug_metrics]               | Configures the metrics that this component generates to monitor its state.                               | no       |
| [`marshaler`][marshaler]                       | Marshaler used to produce output data.                                                                   | no       |
| [`resource_attrs_to_s3`][resource_attrs_to_s3] | Configures the mapping of S3 configuration values to resource attribute values for uploading operations. | no       |
| [`sending_queue`][sending_queue]               | Configures batching of data before sending.                                                              | no       |
| `sending_queue` > [`batch`][batch]             | Configures batching requests based on a timeout and a minimum number of items.                           | no       |

[s3_uploader]: #s3_uploader
[marshaler]: #marshaler
[debug_metrics]: #debug_metrics
[sending_queue]: #sending_queue
[batch]: #batch
[resource_attrs_to_s3]: #resource_attrs_to_s3-block

### `s3_uploader`

{{< badge text="Required" >}}

The `s3_uploader` block configures the AWS S3 bucket details used by the component.

The following arguments are supported:

| Name                  | Type       | Description                                                                                           | Default                                       | Required |
|-----------------------|------------|-------------------------------------------------------------------------------------------------------|-----------------------------------------------|----------|
| `s3_bucket`           | `string`   | The S3 bucket.                                                                                        |                                               | yes      |
| `s3_prefix`           | `string`   | Prefix for the S3 key (root directory inside the bucket).                                             |                                               | yes      |
| `acl`                 | `string`   | The canned ACL to use when uploading objects.                                                         | `"private"`                                   | no       |
| `compression`         | `string`   | File compression method, `none` or `gzip`                                                             | `"none"`                                      | no       |
| `disable_ssl`         | `boolean`  | Set this to `true` to disable SSL when sending requests.                                              | `false`                                       | no       |
| `endpoint`            | `string`   | Overrides the endpoint used by the exporter instead of constructing it from `region` and `s3_bucket`. |                                               | no       |
| `file_prefix`         | `string`   | The file prefix defined by the user.                                                                  |                                               | no       |
| `region`              | `string`   | The AWS region.                                                                                       | `"us-east-1"`                                 | no       |
| `retry_max_attempts`  | `int`      | The max number of attempts for retrying a request.                                                    | `3`                                           | no       |
| `retry_max_backoff`   | `duration` | The max backoff delay that can occur before retrying a request.                                       | `20s`                                         | no       |
| `retry_mode`          | `string`   | The retryer implementation.                                                                           | `"standard"`                                  | no       |
| `role_arn`            | `string`   | The Role ARN to be assumed.                                                                           |                                               | no       |
| `s3_force_path_style` | `boolean`  | Set this to `true` to force the request to use [path-style requests][]                                | `false`                                       | no       |
| `s3_partition_format` | `string`   | Filepath formatting for the partition; Refer to [`strftime`][strftime] for format specification.      | `"year=%Y/month=%m/day=%d/hour=%H/minute=%M"` | no       |
| `storage_class`       | `string`   | The storage class to use when uploading objects.                                                      | `"STANDARD"`                                  | no       |

`retry_mode` must be one of `standard`, `adaptive`, or `nop`.
If `retry_mode` is set to `nop`, the `aws.NopRetryer` implementation effectively disables the retry.
Setting `retry_max_attempts` to 0 will allow the SDK to retry all retryable errors until the request succeeds, or a non-retryable error is returned.

[path-style requests]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html#path-style-access
[strftime]: https://www.man7.org/linux/man-pages/man3/strftime.3.html

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `marshaler`

Marshaler determines the format of data sent to AWS S3. Currently, the following marshalers are implemented:

* `otlp_json` (default): the [OpenTelemetry Protocol format](https://github.com/open-telemetry/opentelemetry-proto), represented as JSON.
* `otlp_proto`: the [OpenTelemetry Protocol format](https://github.com/open-telemetry/opentelemetry-proto), represented as Protocol Buffers.
  A single protobuf message is written into each object.
* `sumo_ic`: the [Sumo Logic Installed Collector Archive format](https://help.sumologic.com/docs/manage/data-archiving/archive/).
  **This format is supported only for logs.**
* `body`: export the log body as string.
  **This format is supported only for logs.**

The following arguments are supported:

| Name   | Type     | Description                            | Default       | Required |
|--------|----------|----------------------------------------|---------------|----------|
| `type` | `string` | Marshaler used to produce output data. | `"otlp_json"` | no       |

### `sending_queue`

The `sending_queue` block configures queueing and batching for the exporter.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.
By default, the `batch` block is not used.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### resource_attrs_to_s3 block

The following arguments are supported:

| Name        | Type     | Description                                                                  | Default | Required |
|-------------|----------|------------------------------------------------------------------------------|---------|----------|
| `s3_prefix` | `string` | Configures which resource attribute's value should be used as the S3 prefix. |         | yes      |

When `s3_prefix` is set, it dynamically overrides [`s3_uploader`][s3_uploader] > `s3_prefix`.
If the specified resource attribute exists in the data, its value will be used as the prefix.
Otherwise, [`s3_uploader`][s3_uploader] > `s3_prefix` will serve as the fallback.

### Compression

* `none` (default): File compression isn't used.
* `gzip`: Files are compressed with Gzip.
  **This doesn't support `sumo_ic`marshaler.**

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.awss3` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.awss3` doesn't expose any component-specific debug information.

## Debug metrics

* `otelcol_exporter_queue_capacity` (gauge): Fixed capacity of the retry queue (in batches).
* `otelcol_exporter_queue_size` (gauge): Current size of the retry queue (in batches).
* `otelcol_exporter_send_failed_spans_total` (counter): Number of spans in failed attempts to send to destination.
* `otelcol_exporter_sent_spans_total` (counter): Number of spans successfully sent to destination.
* `rpc_client_duration_milliseconds` (histogram): Measures the duration of inbound RPC.
* `rpc_client_request_size_bytes` (histogram): Measures size of RPC request messages (uncompressed).
* `rpc_client_requests_per_rpc` (histogram): Measures the number of messages received per RPC. Should be 1 for all non-streaming RPCs.
* `rpc_client_response_size_bytes` (histogram): Measures size of RPC response messages (uncompressed).
* `rpc_client_responses_per_rpc` (histogram): Measures the number of messages received per RPC. Should be 1 for all non-streaming RPCs.

## Example

This example forwards scrape logs to an AWS S3 Bucket:

```alloy
local.file_match "logs" {
  path_targets = [{
    __address__ = "localhost",
    __path__    = "/var/log/{syslog,messages,*.log}",
    instance    = constants.hostname,
    job         = "integrations/node_exporter",
  }]
}

loki.source.file "logs" {
  targets    = local.file_match.logs.targets
  forward_to = [otelcol.receiver.loki.default.receiver]
}

otelcol.receiver.loki "default" {
  output {
    logs = [otelcol.exporter.awss3.logs.input]
  }
}

otelcol.exporter.awss3 "logs" {
  s3_uploader {
    region = "us-east-1"
    s3_bucket = "logs_bucket"
    s3_prefix = "logs"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.awss3` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

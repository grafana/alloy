---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol.exporter.awss3/
description: Learn about otelcol.exporter.awss3
title: otelcol.exporter.awss3
---

<span class="badge docs-labels__stage docs-labels__item">Experimental</span>

# otelcol.exporter.awss3

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.awss3` accepts telemetry data from other `otelcol` components and writes them to an AWS S3 bucket.

{{< admonition type="note" >}}
`otelcol.exporter.awss3` is a wrapper over the upstream OpenTelemetry Collector Contrib `awss3` exporter.
Bug reports or feature requests will be redirected to the upstream repository if necessary.
{{< /admonition >}}

You can specify multiple `otelcol.exporter.awss3` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.awss3 "<LABEL>" {
  s3_uploader {
    region = "<REGION>"
    s3_bucket = "<BUCKET_NAME>"
    s3_prefix = "<PREFIX>"
  }
}
```

Replace the following:

* _`<LABEL>`_: The label for the `otelcol.exporter.awss3` component.
* _`<REGION>`_: The AWS region.
* _`<BUCKET_NAME>`_: The S3 bucket.
* _`<PREFIX>`_: The prefix for the S3 key.

## Arguments

`otelcol.exporter.awss3` supports the following arguments:

Name       | Type       | Description                                      | Default | Required
-----------|------------|--------------------------------------------------|---------|---------
`encoding` | `string`   | Encoding extension to use to marshal data. Overrides the `marshaler` configuration option if set. | `""`  | no
`encoding_file_ext` | `string` | File format extension suffix when using the `encoding` configuration option. It can be left empty if a suffix shouldn't be appended. | `""` | no

## Blocks

The following blocks are supported inside the definition of
`otelcol.exporter.awss3`:

Hierarchy              | Block                | Description                                                                          | Required
-----------------------|----------------------|--------------------------------------------------------------------------------------|---------
s3_uploader            | [s3_uploader][]      | Configures the AWS S3 bucket details to send telemetry data to.                      | yes
marshaler              | [marshaler][]        | Marshaler used to produce output data.                                               | no
debug_metrics          | [debug_metrics][]    | Configures the metrics that this component generates to monitor its state.           | no

[s3_uploader]: #s3_uploader-block
[marshaler]: #marshaler-block
[debug_metrics]: #debug_metrics-block

### s3_uploader block

The `s3_uploader` block configures the AWS S3 bucket details used by the component.

The following arguments are supported:

Name                  | Type                       | Description                                                                      | Default      | Required
----------------------|----------------------------|----------------------------------------------------------------------------------|--------------|---------
`region`              | `string`                   | The AWS region.                                                                      | `"us-east-1"`| no
`s3_bucket`           | `string`                   | The S3 bucket.                                                                        |              | yes
`s3_prefix`           | `string`                   | Prefix for the S3 key (root directory inside the bucket).                            |              | yes
`s3_partition`        | `string`                   | Time granularity of S3 key: hour or minute.                                       | `"minute"`   | no
`role_arn`            | `string`                   | The Role ARN to be assumed.                                                       |              | no
`file_prefix`         | `string`                   | The file prefix defined by the user.                                                      |              | no
`endpoint`            | `string`                   | Overrides the endpoint used by the exporter instead of constructing it from `region` and `s3_bucket`. |      | no
`s3_force_path_style` | `boolean`                  |  Set this to `true` to force the request to use [path-style requests](https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html#path-style-access) | `false`             | no
`disable_ssl`         | `boolean`                  |  Set this to `true` to disable SSL when sending requests.           |              | `false`
`compression`         | `string`                   | How should the file be compressed? `none`, `gzip`                                                    | `none`      | no

[path-style requests]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html#path-style-access

### marshaler block

Marshaler determines the format of data sent to AWS S3. Currently, the following marshalers are implemented:

- `otlp_json` (default): the [OpenTelemetry Protocol format](https://github.com/open-telemetry/opentelemetry-proto), represented as JSON.
- `otlp_proto`: the [OpenTelemetry Protocol format](https://github.com/open-telemetry/opentelemetry-proto), represented as Protocol Buffers. A single protobuf message is written into each object.
- `sumo_ic`: the [Sumo Logic Installed Collector Archive format](https://help.sumologic.com/docs/manage/data-archiving/archive/).
  **This format is supported only for logs.**
- `body`: export the log body as string.
  **This format is supported only for logs.**

The following arguments are supported:

Name                    | Type       | Description                                                                                | Default | Required
------------------------|------------|--------------------------------------------------------------------------------------------|---------|---------
`type`                  | `string`   | Marshaler used to produce output data.                                                      | `"otlp_json"`   | no

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### Encoding

Encoding overrides the marshaler if it's present and sets it to use the encoding extension defined in the collector configuration.

Refer to the Open Telemetry [encoding extensions][] documentation for more information.

[encoding]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/encoding

### Compression

- `none` (default): File compression isn't used.
- `gzip`: Files are compressed with Gzip. **This doesn't support `sumo_ic`marshaler.**

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics,
logs, or traces).

## Component health

`otelcol.exporter.awss3` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.exporter.awss3` doesn't expose any component-specific debug
information.

## Debug metrics

* `exporter_sent_spans_ratio_total` (counter): Number of spans successfully sent to destination.
* `exporter_send_failed_spans_ratio_total` (counter): Number of spans in failed attempts to send to destination.
* `exporter_queue_capacity_ratio` (gauge): Fixed capacity of the retry queue (in batches).
* `exporter_queue_size_ratio` (gauge): Current size of the retry queue (in batches).
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

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.kafka/
aliases:
  - ../otelcol.exporter.kafka/ # /docs/alloy/latest/reference/components/otelcol.exporter.kafka/
description: Learn about otelcol.exporter.kafka
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.exporter.kafka
---

# `otelcol.exporter.kafka`

`otelcol.exporter.kafka` accepts logs, metrics, and traces telemetry data from other `otelcol` components and sends it to Kafka.

It's important to use `otelcol.exporter.kafka` together with `otelcol.processor.batch` to make sure `otelcol.exporter.kafka` doesn't slow down due to sending Kafka a huge number of small payloads.

{{< admonition type="note" >}}
`otelcol.exporter.kafka` is a wrapper over the upstream OpenTelemetry Collector [`kafka`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`kafka`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/kafkaexporter
{{< /admonition >}}

Multiple `otelcol.exporter.kafka` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.exporter.kafka "LABEL" {
  protocol_version = "PROTOCOL_VERSION"
}
```

## Arguments

You can use the following arguments with `otelcol.exporter.kafka`:

| Name                                       | Type           | Description                                                                                                                 | Default              | Required |
|--------------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------|----------------------|----------|
| `protocol_version`                         | `string`       | Kafka protocol version to use.                                                                                              |                      | yes      |
| `brokers`                                  | `list(string)` | Kafka brokers to connect to.                                                                                                | `["localhost:9092"]` | no       |
| `allow_auto_topic_creation`                | `bool`         | Whether to allow automatic topic creation.                                                                                  | `true`               | no       |
| `client_id`                                | `string`       | Consumer client ID to use. The ID will be used for all produce requests.                                                    | `"otel-collector"`   | no       |
| `encoding`                                 | `string`       | (Deprecated) Encoding of payload read from Kafka.                                                                           | `"otlp_proto"`       | no       |
| `include_metadata_keys`                    | `list(string)` | List of metadata keys to propagate as Kafka message headers.                                                                | `[]`                 | no       |
| `partition_metrics_by_resource_attributes` | `bool`         | Whether to include the hash of sorted resource attributes as the message partitioning key in metric messages sent to Kafka. | `false`              | no       |
| `partition_logs_by_resource_attributes`    | `bool`         | Whether to include the hash of sorted resource attributes as the message partitioning key in log messages sent to Kafka.    | `false`              | no       |
| `partition_logs_by_trace_id`               | `bool`         | Whether to use the 16-bit hex string of the trace ID as the message partitioning key in log messages sent to Kafka.         | `false`              | no       |
| `partition_traces_by_id`                   | `bool`         | Whether to include the trace ID as the message key in trace messages sent to Kafka.                                         | `false`              | no       |
| `resolve_canonical_bootstrap_servers_only` | `bool`         | Whether to resolve then reverse-lookup broker IPs during startup.                                                           | `false`              | no       |
| `timeout`                                  | `duration`     | The timeout for every attempt to send data to the backend.                                                                  | `"5s"`               | no       |
| `topic_from_attribute`                     | `string`       | A resource attribute whose value should be used as the message's topic.                                                     | `""`                 | no       |
| `topic`                                    | `string`       | (Deprecated) Kafka topic to send to.                                                                                        | _See below_          | no       |

{{< admonition type="warning" >}}
The `topic` and `encoding` arguments are deprecated in favor of the [`logs`][logs], [`metrics`][metrics], and [`traces`][traces] blocks.
{{< /admonition >}}

When `topic_from_metadata_key` is set in a signal-specific block, it will take precedence over `topic_from_attribute` and `topic` arguments.
When `topic_from_attribute` is set, it will take precedence over the `topic` arguments in [`logs`][logs], [`metrics`][metrics], and [`traces`][traces] blocks.

`partition_traces_by_id` doesn't have any effect on Jaeger encoding exporters since Jaeger exporters include trace ID as the message key by default.

`partition_logs_by_resource_attributes` and `partition_logs_by_trace_id` are mutually exclusive and can't both be `true`.

`include_metadata_keys` specifies metadata keys to propagate as Kafka message headers. If one or more keys aren't found in the metadata, they are ignored. The keys also partition the data before export if `sending_queue.batch` is defined.

## Blocks

You can use the following blocks with `otelcol.exporter.kafka`:

| Block                                                   | Description                                                                    | Required |
|---------------------------------------------------------|--------------------------------------------------------------------------------|----------|
| [`authentication`][authentication]                      | Configures authentication for connecting to Kafka brokers.                     | no       |
| `authentication` > [`kerberos`][kerberos]               | Authenticates against Kafka brokers with Kerberos.                             | no       |
| `authentication` > [`plaintext`][plaintext]             | Authenticates against Kafka brokers with plaintext.                            | no       |
| `authentication` > [`sasl`][sasl]                       | Authenticates against Kafka brokers with SASL.                                 | no       |
| `authentication` > `sasl` > [`aws_msk`][aws_msk]        | Additional SASL parameters when using AWS_MSK_IAM_OAUTHBEARER.                 | no       |
| `authentication` > [`tls`][tls]                         | Configures TLS for connecting to the Kafka brokers.                            | no       |
| `authentication` > `tls` > [`tpm`][tpm]                 | Configures TPM for the TLS `key_file.                                          | no       |
| [`debug_metrics`][debug_metrics]                        | Configures the metrics which this component generates to monitor its state.    | no       |
| [`logs`][logs]                                          | Configures how to send logs to Kafka brokers.                                  | no       |
| [`metadata`][metadata]                                  | Configures how to retrieve metadata from Kafka brokers.                        | no       |
| `metadata` > [`retry`][retry]                           | Configures how to retry metadata retrieval.                                    | no       |
| [`metrics`][metrics]                                    | Configures how to send metrics to Kafka brokers.                               | no       |
| [`producer`][producer]                                  | Kafka producer configuration,                                                  | no       |
| `producer` > [`compression_params`][compression_params] | Configures the compression parameters for the kafka producer.                  | no       |
| [`retry_on_failure`][retry_on_failure]                  | Configures retry mechanism for failed requests.                                | no       |
| [`sending_queue`][sending_queue]                        | Configures batching of data before sending.                                    | no       |
| `sending_queue` > [`batch`][batch]                      | Configures batching requests based on a timeout and a minimum number of items. | no       |
| [`tls`][tls]                                            | Configures TLS for connecting to the Kafka brokers.                            | no       |
| `tls` > [`tpm`][tpm]                                    | Configures TPM settings for the TLS key_file.                                  | no       |
| [`traces`][traces]                                      | Configures how to send traces to Kafka brokers.                                | no       |

The > symbol indicates deeper levels of nesting.
For example, `authentication` > `tls` refers to a `tls` block defined inside an `authentication` block.

[logs]: #logs
[metrics]: #metrics
[traces]: #traces
[authentication]: #authentication
[plaintext]: #plaintext
[sasl]: #sasl
[aws_msk]: #aws_msk
[tls]: #tls
[tpm]: #tpm
[kerberos]: #kerberos
[metadata]: #metadata
[retry]: #retry
[retry_on_failure]: #retry_on_failure
[sending_queue]: #sending_queue
[batch]: #batch
[producer]: #producer
[compression_params]: #compression_params
[debug_metrics]: #debug_metrics

### `logs`

The `logs` block configures how to send logs to Kafka brokers.

| Name                     | Type     | Description                                                                  | Default        | Required |
| ------------------------ | -------- | ---------------------------------------------------------------------------- | -------------- | -------- |
| `encoding`               | `string` | The encoding for logs. Refer to [Supported encodings](#supported-encodings). | `"otlp_proto"` | no       |
| `topic`                  | `string` | The name of the Kafka topic to which logs will be exported.                  | `"otlp_logs"`  | no       |
| `topic_from_metadata_key` | `string` | The name of the metadata key whose value should be used as the message's topic. Takes precedence over `topic_from_attribute` and `topic` settings. | `""`           | no       |

### `metrics`

The `metrics` block configures how to send metrics to Kafka brokers.

| Name                     | Type     | Description                                                                  | Default          | Required |
| ------------------------ | -------- | ---------------------------------------------------------------------------- | ---------------- | -------- |
| `encoding`               | `string` | The encoding for logs. Refer to [Supported encodings](#supported-encodings). | `"otlp_proto"`   | no       |
| `topic`                  | `string` | The name of the Kafka topic to which metrics will be exported.               | `"otlp_metrics"` | no       |
| `topic_from_metadata_key` | `string` | The name of the metadata key whose value should be used as the message's topic. Takes precedence over `topic_from_attribute` and `topic` settings. | `""`             | no       |

### `traces`

The `traces` block configures how to send traces to Kafka brokers.

| Name                     | Type     | Description                                                                  | Default        | Required |
| ------------------------ | -------- | ---------------------------------------------------------------------------- | -------------- | -------- |
| `encoding`               | `string` | The encoding for logs. Refer to [Supported encodings](#supported-encodings). | `"otlp_proto"` | no       |
| `topic`                  | `string` | The name of the Kafka topic to which traces will be exported.                | `"otlp_spans"` | no       |
| `topic_from_metadata_key` | `string` | The name of the metadata key whose value should be used as the message's topic. Takes precedence over `topic_from_attribute` and `topic` settings. | `""`           | no       |

### `authentication`

{{< docs/shared lookup="reference/components/otelcol-kafka-authentication.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `kerberos`

{{< docs/shared lookup="reference/components/otelcol-kafka-authentication-kerberos.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `plaintext`

{{< docs/shared lookup="reference/components/otelcol-kafka-authentication-plaintext.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sasl`

{{< docs/shared lookup="reference/components/otelcol-kafka-authentication-sasl.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `aws_msk`

{{< docs/shared lookup="reference/components/otelcol-kafka-authentication-sasl-aws_msk.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS settings used for connecting to the Kafka brokers.
If the `tls` block isn't provided, TLS won't be used for communication.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `metadata`

{{< docs/shared lookup="reference/components/otelcol-kafka-metadata.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `retry`

{{< docs/shared lookup="reference/components/otelcol-kafka-metadata-retry.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `producer`

The `producer` block configures how to retry retrieving metadata when retrieval fails.

The following arguments are supported:

| Name                 | Type     | Description                                         | Default   | Required |
| -------------------- | -------- | --------------------------------------------------- | --------- | -------- |
| `compression`        | `string` | Time to wait between retries.                       | `"none"`  | no       |
| `flush_max_messages` | `number` | Time to wait between retries.                       | `0`       | no       |
| `max_message_bytes`  | `number` | The maximum permitted size of a message in bytes.   | `1000000` | no       |
| `required_acks`      | `number` | Controls when a message is regarded as transmitted. | `1`       | no       |

Refer to the [Go sarama documentation][RequiredAcks] for more information on `required_acks`.

`compression` could be set to either `none`, `gzip`, `snappy`, `lz4`, or `zstd`.
Refer to the [Go sarama documentation][CompressionCodec] for more information.

[RequiredAcks]: https://pkg.go.dev/github.com/IBM/sarama@v1.43.2#RequiredAcks
[CompressionCodec]: https://pkg.go.dev/github.com/IBM/sarama@v1.43.2#CompressionCodec

### `compression_params`

The `compression_params` block configures the producer compression parameters.

The following argument is supported:

| Name                 | Type     | Description                                         | Default   | Required |
| -------------------- | -------- | --------------------------------------------------- | --------- | -------- |
| `level`              | `int`    | The level of compression to use on messages.        | `0`       | no       |

The following levels are valid combinations of `compression` and `level`:

| Compression | Value | Description            |
|-------------|-------|------------------------|
| `gzip`      | `1`   | BestSpeed              |
| `gzip`      | `9`   | BestCompression        |
| `gzip`      | `-1`  | DefaultCompression     |
| `zstd`      | `1`   | SpeedFastest           |
| `zstd`      | `3`   | SpeedDefault           |
| `zstd`      | `6`   | SpeedBetterCompression |
| `zstd`      | `11`  | SpeedBestCompression   |


`lz4` and `snappy` do not currently support compression levels in this component.

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to Kafka are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures queueing and batching for the exporter.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Supported encodings

`otelcol.exporter.kafka` supports encoding extensions, as well as the following built-in encodings.

Available for all signals:

* `otlp_proto`: Data is encoded as OTLP Protobuf.
* `otlp_json`: Data is encoded as OTLP JSON.

Available only for traces:

* `jaeger_proto`: The payload is serialized to a single Jaeger proto `Span`, and keyed by TraceID.
* `jaeger_json`: The payload is serialized to a single Jaeger JSON Span using `jsonpb`, and keyed by TraceID.
* `zipkin_proto`: The payload is serialized to Zipkin v2 proto Span.
* `zipkin_json`: The payload is serialized to Zipkin v2 JSON Span.

Available only for logs:

* `raw`: If the log record body is a byte array, it is sent as is.
   Otherwise, it is serialized to JSON.
   Resource and record attributes are discarded.

## Component health

`otelcol.exporter.kafka` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.kafka` doesn't expose any component-specific debug information.

## Example

This example forwards telemetry data through a batch processor before finally sending it to Kafka:

```alloy
otelcol.receiver.otlp "default" {
  http {}
  grpc {}

  output {
    metrics = [otelcol.processor.batch.default.input]
    logs    = [otelcol.processor.batch.default.input]
    traces  = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    metrics = [otelcol.exporter.kafka.default.input]
    logs    = [otelcol.exporter.kafka.default.input]
    traces  = [otelcol.exporter.kafka.default.input]
  }
}

otelcol.exporter.kafka "default" {
  brokers          = ["localhost:9092"]
  protocol_version = "2.0.0"
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.kafka` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

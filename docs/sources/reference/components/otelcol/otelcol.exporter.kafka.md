---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol.exporter.kafka/
description: Learn about otelcol.exporter.kafka
title: otelcol.exporter.kafka
---

# otelcol.exporter.kafka

`otelcol.exporter.kafka` accepts logs, metrics, and traces telemetry data from 
other `otelcol` components and sends it to Kafka.

It is important to use `otelcol.exporter.kafka` together with `otelcol.processor.batch`
to make sure `otelcol.exporter.kafka` doesn't slow down due to sending Kafka a huge number of small payloads.

{{< admonition type="note" >}}
`otelcol.exporter.kafka` is a wrapper over the upstream OpenTelemetry Collector `kafka` exporter from the `otelcol-contrib`  distribution.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
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

The following arguments are supported:

Name                                       | Type            | Description                                                                         | Default              | Required
------------------------------------------ | --------------- | ----------------------------------------------------------------------------------- | -------------------- | --------
`protocol_version`                         | `string`        | Kafka protocol version to use.                                                      |                      | yes
`brokers`                                  | `list(string)`  | Kafka brokers to connect to.                                                        | `["localhost:9092"]` | no
`topic`                                    | `string`        | Kafka topic to send to.                                                             |  _See below_         | no
`topic_from_attribute`                     | `string`        | A resource attribute whose value should be used as the message's topic.             |  `""`                | no
`encoding`                                 | `string`        | Encoding of payload read from Kafka.                                                | `"otlp_proto"`       | no
`client_id`                                | `string`        | Consumer client ID to use. The ID will be used for all produce requests.            | `"sarama"`           | no
`timeout`                                  | `duration`      | The timeout for every attempt to send data to the backend.                          | `"5s"`               | no
`resolve_canonical_bootstrap_servers_only` | `bool`          | Whether to resolve then reverse-lookup broker IPs during startup.                   | `"false"`            | no
`partition_traces_by_id`                   | `bool`          | Whether to include the trace ID as the message key in trace messages sent to Kafka. | `"false"`            | no
`partition_metrics_by_resource_attributes` | `bool`          | Whether to include the hash of sorted resource attributes as the message partitioning key in metric messages sent to Kafka. | `"false"`            | no

If `topic` is not set, different topics will be used for different telemetry signals:

* Metrics will be sent to an `otlp_metrics` topic.
* Traces will be sent to an `otlp_spans` topic.
* Logs will be sent to an `otlp_logs` topic.

If topic is set, the same topic will be used for all telemetry signals - metrics, logs, and traces.

When `topic_from_attribute` is set, it will take precedence over `topic`.

The `encoding` argument determines how to encode messages sent to Kafka.
`encoding` must be one of the following strings:
* Encodings which work for traces, logs, and metrics:
  * `"otlp_proto"`: Encode messages as OTLP protobuf. 
  * `"otlp_json"`: Encode messages as OTLP JSON.
* Encodings which work only for traces:
  * `"jaeger_proto"`: The payload is serialized to a single Jaeger proto `Span`, and keyed by TraceID.
  * `"jaeger_json"`: The payload is serialized to a single Jaeger JSON Span using `jsonpb`, and keyed by TraceID.
  * `"zipkin_proto"`: The payload is serialized to Zipkin v2 proto Span.
  * `"zipkin_json"`: The payload is serialized to Zipkin v2 JSON Span.
* Encodings which work only for logs:
  * `"raw"`: If the log record body is a byte array, it is sent as is. Otherwise, it is serialized to JSON. Resource and record attributes are discarded.

`partition_traces_by_id` does not have any effect on Jaeger encoding exporters since Jaeger exporters include trace ID as the message key by default.

## Blocks

The following blocks are supported inside the definition of `otelcol.exporter.kafka`:

Hierarchy                        | Block               | Description                                                                 | Required
-------------------------------- | ------------------- | --------------------------------------------------------------------------- | --------
authentication                   | [authentication][]   | Configures authentication for connecting to Kafka brokers.                  | no
authentication > plaintext       | [plaintext][]        | Authenticates against Kafka brokers with plaintext.                         | no
authentication > sasl            | [sasl][]             | Authenticates against Kafka brokers with SASL.                              | no
authentication > sasl > aws_msk  | [aws_msk][]          | Additional SASL parameters when using AWS_MSK_IAM.                          | no
authentication > tls             | [tls][]              | Configures TLS for connecting to the Kafka brokers.                         | no
authentication > kerberos        | [kerberos][]         | Authenticates against Kafka brokers with Kerberos.                          | no
metadata                         | [metadata][]         | Configures how to retrieve metadata from Kafka brokers.                     | no
metadata > retry                 | [retry][]            | Configures how to retry metadata retrieval.                                 | no
retry_on_failure                 | [retry_on_failure][] | Configures retry mechanism for failed requests.                             | no
queue                            | [queue][]            | Configures batching of data before sending.                                 | no
producer                         | [producer][]         | Kafka producer configuration,                                               | no
debug_metrics                    | [debug_metrics][]    | Configures the metrics which this component generates to monitor its state. | no

The `>` symbol indicates deeper levels of nesting. 
For example, `authentication > tls` refers to a `tls` block defined inside an `authentication` block.

[authentication]: #authentication-block
[plaintext]: #plaintext-block
[sasl]: #sasl-block
[aws_msk]: #aws_msk-block
[tls]: #tls-block
[kerberos]: #kerberos-block
[metadata]: #metadata-block
[retry]: #retry-block
[retry_on_failure]: #retry_on_failure-block
[queue]: #queue-block
[producer]: #producer-block
[debug_metrics]: #debug_metrics-block

### authentication block

{{< docs/shared lookup="reference/components/otelcol-kafka-authentication.md" source="alloy" version="<ALLOY_VERSION>" >}}

### plaintext block

{{< docs/shared lookup="reference/components/otelcol-kafka-authentication-plaintext.md" source="alloy" version="<ALLOY_VERSION>" >}}

### sasl block

{{< docs/shared lookup="reference/components/otelcol-kafka-authentication-sasl.md" source="alloy" version="<ALLOY_VERSION>" >}}

### aws_msk block

{{< docs/shared lookup="reference/components/otelcol-kafka-authentication-sasl-aws_msk.md" source="alloy" version="<ALLOY_VERSION>" >}}

### tls block

The `tls` block configures TLS settings used for connecting to the Kafka
brokers. If the `tls` block isn't provided, TLS won't be used for
communication.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### kerberos block

{{< docs/shared lookup="reference/components/otelcol-kafka-authentication-kerberos.md" source="alloy" version="<ALLOY_VERSION>" >}}

### metadata block

{{< docs/shared lookup="reference/components/otelcol-kafka-metadata.md" source="alloy" version="<ALLOY_VERSION>" >}}

### retry block

{{< docs/shared lookup="reference/components/otelcol-kafka-metadata-retry.md" source="alloy" version="<ALLOY_VERSION>" >}}

### retry_on_failure block

The `retry_on_failure` block configures how failed requests to Kafka are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### queue block

The `queue` block configures an in-memory buffer of batches before data is sent to the gRPC server.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### producer block

The `producer` block configures how to retry retrieving metadata when retrieval fails.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`max_message_bytes` | `number` | The maximum permitted size of a message in bytes. | `1000000` | no
`required_acks` | `number` | Controls when a message is regarded as transmitted.   | `1` | no
`compression` | `string` | Time to wait between retries. | `"none"` | no
`flush_max_messages` | `number` | Time to wait between retries. | `0` | no

Refer to the [sarama documentation][RequiredAcks] for more information on `required_acks`.

`compression` could be set to either `none`, `gzip`, `snappy`, `lz4`, or `zstd`.
Refer to the [Sarama documentation][CompressionCodec] for more information.

[RequiredAcks]: https://pkg.go.dev/github.com/IBM/sarama@v1.43.2#RequiredAcks
[CompressionCodec]: https://pkg.go.dev/github.com/IBM/sarama@v1.43.2#CompressionCodec

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.kafka` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.exporter.kafka` does not expose any component-specific debug
information.

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

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
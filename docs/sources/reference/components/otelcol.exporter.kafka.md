---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol.exporter.kafka/
description: Learn about otelcol.exporter.kafka
title: otelcol.exporter.kafka
---

# otelcol.exporter.kafka

`otelcol.exporter.kafka` accepts logs, metrics, and traces telemetry data from 
other `otelcol` components and sends it to Kafka.

> **NOTE**: `otelcol.exporter.kafka` is a wrapper over the upstream
> OpenTelemetry Collector `kafka` exporter from the `otelcol-contrib`
> distribution. Bug reports or feature requests will be redirected to the
> upstream repository, if necessary.

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
`topic`                                    | `string`        | Kafka topic to read from.                                                           |  _See below_         | no
`topic_from_attribute`                     | `string`        | A resource attribute whose value should be used as the message's topic.             |  `""`                | no
`encoding`                                 | `string`        | Encoding of payload read from Kafka.                                                | `"otlp_proto"`       | no
`client_id`                                | `string`        | Consumer client ID to use. The ID will be used for all produce requests.            | `"sarama"`           | no
`timeout`                                  | `duration`      | The timeout for every attempt to send data to the backend.                          | `"5s"`               | no
`resolve_canonical_bootstrap_servers_only` | `bool`          | Whether to resolve then reverse-lookup broker IPs during startup.                   | `"false"`            | no
`partition_traces_by_id`                   | `bool`          | Whether to include the trace ID as the message key in trace messages sent to Kafka. | `"false"`            | no
`partition_metrics_by_resource_attributes` | `bool`          | Whether to include the hash of sorted resource attributes as the message partitioning key in metric messages sent to Kafka. | `"false"`            | no

If `topic` is not set, different topics will be used for different telemetry signals:

* Metrics will be received from an `otlp_metrics` topic.
* Traces will be received from an `otlp_spans` topic.
* Logs will be received from an `otlp_logs` topic.

If `topic` is set to a specific value, then only the signal type that corresponds to the data stored in the topic must be set in the output block.
For example, if `topic` is set to `"my_telemetry"`, then the `"my_telemetry"` topic can only contain either metrics, logs, or traces. 
If it contains only metrics, then `otelcol.exporter.kafka` should be configured to process only metrics.

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

The `authentication` block holds the definition of different authentication
mechanisms to use when connecting to Kafka brokers. It doesn't support any
arguments and is configured fully through inner blocks.

### plaintext block

The `plaintext` block configures `PLAIN` authentication against Kafka brokers.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`username` | `string` | Username to use for `PLAIN` authentication. | | yes
`password` | `secret` | Password to use for `PLAIN` authentication. | | yes

### sasl block

The `sasl` block configures SASL authentication against Kafka brokers.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`username` | `string` | Username to use for SASL authentication. | | yes
`password` | `secret` | Password to use for SASL authentication. | | yes
`mechanism` | `string` | SASL mechanism to use when authenticating. | | yes
`version` | `number` | Version of the SASL Protocol to use when authenticating. | `0` | no

The `mechanism` argument can be set to one of the following strings:

* `"PLAIN"`
* `"AWS_MSK_IAM"`
* `"SCRAM-SHA-256"`
* `"SCRAM-SHA-512"`

When `mechanism` is set to `"AWS_MSK_IAM"`, the [`aws_msk` child block][aws_msk] must also be provided.

The `version` argument can be set to either `0` or `1`.

### aws_msk block

The `aws_msk` block configures extra parameters for SASL authentication when
using the `AWS_MSK_IAM` mechanism.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`region` | `string` | AWS region the MSK cluster is based in. | | yes
`broker_addr` | `string` | MSK address to connect to for authentication. | | yes

### tls block

The `tls` block configures TLS settings used for connecting to the Kafka
brokers. If the `tls` block isn't provided, TLS won't be used for
communication.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### kerberos block

The `kerberos` block configures Kerberos authentication against the Kafka
broker.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`service_name` | `string` | Kerberos service name. | | no
`realm` | `string` | Kerberos realm. | | no
`use_keytab` | `string` | Enables using keytab instead of password. | | no
`username` | `string` | Kerberos username to authenticate as. | | yes
`password` | `secret` | Kerberos password to authenticate with. | | no
`config_file` | `string` | Path to Kerberos location (for example, `/etc/krb5.conf`). | | no
`keytab_file` | `string` | Path to keytab file (for example, `/etc/security/kafka.keytab`). | | no

When `use_keytab` is `false`, the `password` argument is required. When
`use_keytab` is `true`, the file pointed to by the `keytab_file` argument is
used for authentication instead. At most one of `password` or `keytab_file`
must be provided.

### metadata block

The `metadata` block configures how to retrieve and store metadata from the
Kafka broker.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`include_all_topics` | `bool` | When true, maintains metadata for all topics. | `true` | no

If the `include_all_topics` argument is `true`, `otelcol.exporter.kafka`
maintains a full set of metadata for all topics rather than the minimal set
that has been necessary so far. Including the full set of metadata is more
convenient for users but can consume a substantial amount of memory if you have
many topics and partitions.

Retrieving metadata may fail if the Kafka broker is starting up at the same
time as the `otelcol.exporter.kafka` component. The [`retry` child
block][retry] can be provided to customize retry behavior.

### retry block

The `retry` block configures how to retry retrieving metadata when retrieval
fails.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`max_retries` | `number` | How many times to reattempt retrieving metadata. | `3` | no
`backoff` | `duration` | Time to wait between retries. | `"250ms"` | no

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
Refer to the [sarama documentation][CompressionCodec] for more information.

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
---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.kafka/
aliases:
  - ../otelcol.receiver.kafka/ # /docs/alloy/latest/reference/otelcol.receiver.kafka/
description: Learn about otelcol.receiver.kafka
title: otelcol.receiver.kafka
---

# otelcol.receiver.kafka

`otelcol.receiver.kafka` accepts telemetry data from a Kafka broker and
forwards it to other `otelcol.*` components.

> **NOTE**: `otelcol.receiver.kafka` is a wrapper over the upstream
> OpenTelemetry Collector `kafka` receiver from the `otelcol-contrib`
> distribution. Bug reports or feature requests will be redirected to the
> upstream repository, if necessary.

Multiple `otelcol.receiver.kafka` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.receiver.kafka "LABEL" {
  brokers          = ["BROKER_ADDR"]
  protocol_version = "PROTOCOL_VERSION"

  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`brokers` | `array(string)` | Kafka brokers to connect to. | | yes
`protocol_version` | `string` | Kafka protocol version to use. | | yes
`topic` | `string` | Kafka topic to read from. | _See below_ | no
`encoding` | `string` | Encoding of payload read from Kafka. | `"otlp_proto"` | no
`group_id` | `string` | Consumer group to consume messages from. | `"otel-collector"` | no
`client_id` | `string` | Consumer client ID to use. | `"otel-collector"` | no
`initial_offset` | `string` | Initial offset to use if no offset was previously committed. | `"latest"` | no
`resolve_canonical_bootstrap_servers_only` | `bool` | Whether to resolve then reverse-lookup broker IPs during startup. | `"false"` | no

If `topic` is not set, different topics will be used for different telemetry signals:

* Metrics will be received from an `otlp_metrics` topic.
* Traces will be received from an `otlp_spans` topic.
* Logs will be received from an `otlp_logs` topic.

If `topic` is set to a specific value, then only the signal type that corresponds to the data stored in the topic must be set in the output block.
For example, if `topic` is set to `"my_telemetry"`, then the `"my_telemetry"` topic can only contain either metrics, logs, or traces. 
If it contains only metrics, then `otelcol.receiver.kafka` should be configured to output only metrics.

The `encoding` argument determines how to decode messages read from Kafka.
`encoding` must be one of the following strings:

* `"otlp_proto"`: Decode messages as OTLP protobuf.
* `"jaeger_proto"`: Decode messages as a single Jaeger protobuf span.
* `"jaeger_json"`: Decode messages as a single Jaeger JSON span.
* `"zipkin_proto"`: Decode messages as a list of Zipkin protobuf spans.
* `"zipkin_json"`: Decode messages as a list of Zipkin JSON spans.
* `"zipkin_thrift"`: Decode messages as a list of Zipkin Thrift spans.
* `"raw"`: Copy the log message bytes into the body of a log record.
* `"text"`: Decode the log message as text and insert it into the body of a log record.
  By default, UTF-8 is used to decode. A different encoding can be chosen by using `text_<ENCODING>`. For example, `text_utf-8` or `text_shift_jis`.
* `"json"`: Decode the JSON payload and insert it into the body of a log record.
* `"azure_resource_logs"`: The payload is converted from Azure Resource Logs format to an OTLP log.

`"otlp_proto"` must be used to read all telemetry types from Kafka; other
encodings are signal-specific.

`initial_offset` must be either `"latest"` or `"earliest"`.

## Blocks

The following blocks are supported inside the definition of
`otelcol.receiver.kafka`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
authentication | [authentication][] | Configures authentication for connecting to Kafka brokers. | no
authentication > plaintext | [plaintext][] | Authenticates against Kafka brokers with plaintext. | no
authentication > sasl | [sasl][] | Authenticates against Kafka brokers with SASL. | no
authentication > sasl > aws_msk | [aws_msk][] | Additional SASL parameters when using AWS_MSK_IAM. | no
authentication > tls | [tls][] | Configures TLS for connecting to the Kafka brokers. | no
authentication > kerberos | [kerberos][] | Authenticates against Kafka brokers with Kerberos. | no
metadata | [metadata][] | Configures how to retrieve metadata from Kafka brokers. | no
metadata > retry | [retry][] | Configures how to retry metadata retrieval. | no
autocommit | [autocommit][] | Configures how to automatically commit updated topic offsets to back to the Kafka brokers. | no
message_marking | [message_marking][] | Configures when Kafka messages are marked as read. | no
header_extraction | [header_extraction][] | Extract headers from Kafka records. | no
debug_metrics | [debug_metrics][] | Configures the metrics which this component generates to monitor its state. | no
output | [output][] | Configures where to send received telemetry data. | yes

The `>` symbol indicates deeper levels of nesting. For example,
`authentication > tls` refers to a `tls` block defined inside an
`authentication` block.

[authentication]: #authentication-block
[plaintext]: #plaintext-block
[sasl]: #sasl-block
[aws_msk]: #aws_msk-block
[tls]: #tls-block
[kerberos]: #kerberos-block
[metadata]: #metadata-block
[retry]: #retry-block
[autocommit]: #autocommit-block
[message_marking]: #message_marking-block
[header_extraction]: #header_extraction-block
[debug_metrics]: #debug_metrics-block
[output]: #output-block

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

### autocommit block

The `autocommit` block configures how to automatically commit updated topic
offsets back to the Kafka brokers.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`enable` | `bool` | Enable autocommitting updated topic offsets. | `true` | no
`interval` | `duration` | How frequently to autocommit. | `"1s"` | no

### message_marking block

The `message_marking` block configures when Kafka messages are marked as read.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`after_execution` | `bool` | Mark messages after forwarding telemetry data to other components. | `false` | no
`include_unsuccessful` | `bool` | Whether failed forwards should be marked as read. | `false` | no

By default, a Kafka message is marked as read immediately after it is retrieved
from the Kafka broker. If the `after_execution` argument is true, messages are
only read after the telemetry data is forwarded to components specified in [the
`output` block][output].

When `after_execution` is true, messages are only marked as read when they are
decoded successfully and components where the data was forwarded did not return
an error. If the `include_unsuccessful` argument is true, messages are marked
as read even if decoding or forwarding failed. Setting `include_unsuccessful`
has no effect if `after_execution` is `false`.

> **WARNING**: Setting `after_execution` to `true` and `include_unsuccessful`
> to `false` can block the entire Kafka partition if message processing returns
> a permanent error, such as failing to decode.

### header_extraction block

The `header_extraction` block configures how to extract headers from Kafka records.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`extract_headers` | `bool` | Enables attaching header fields to resource attributes. | `false` | no
`headers` | `list(string)` | A list of headers to extract from the Kafka record. | `[]` | no

Regular expressions are not allowed in the `headers` argument. Only exact matching will be performed.

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.kafka` does not export any fields.

## Component health

`otelcol.receiver.kafka` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.receiver.kafka` does not expose any component-specific debug
information.

## Example

This example forwards read telemetry data through a batch processor before
finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.kafka "default" {
  brokers          = ["localhost:9092"]
  protocol_version = "2.0.0"

  output {
    metrics = [otelcol.processor.batch.default.input]
    logs    = [otelcol.processor.batch.default.input]
    traces  = [otelcol.processor.batch.default.input]
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
    endpoint = env("OTLP_ENDPOINT")
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.kafka` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
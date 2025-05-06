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
`topic` | `string` | (Deprecated) Kafka topic to read from. | `""` | no
`encoding` | `string` | (Deprecated) Encoding of payload read from Kafka. | `""` | no
`group_id` | `string` | Consumer group to consume messages from. | `"otel-collector"` | no
`client_id` | `string` | Consumer client ID to use. | `"otel-collector"` | no
`initial_offset` | `string` | Initial offset to use if no offset was previously committed. | `"latest"` | no
`resolve_canonical_bootstrap_servers_only` | `bool` | Whether to resolve then reverse-lookup broker IPs during startup. | `"false"` | no
`session_timeout` | `duration` | The request timeout for detecting client failures when using Kafka group management. | `"10s"` | no
`heartbeat_interval` | `duration` | The expected time between heartbeats to the consumer coordinator when using Kafka group management. | `"3s"` | no
`min_fetch_size` | `int` | The minimum number of message bytes to fetch in a request. | `1` | no
`default_fetch_size` | `int` | The default number of message bytes to fetch in a request. | `1048576` | no
`max_fetch_size` | `int` | The maximum number of message bytes to fetch in a request. | `0` | no
`max_fetch_wait` | `duration` | The maximum amount of time the broker should wait for `min_fetch_size` bytes to be available before returning anyway. | `"250ms"` | no
`group_rebalance_strategy` | `string` | The strategy used to assign partitions to consumers within a consumer group. | `"range"` | no
`group_instance_id` | `string` | A unique identifier for the consumer instance within a consumer group. | `""` | no

For `max_fetch_size`, the value `0` means no limit.

`initial_offset` must be either `"latest"` or `"earliest"`.

The `group_rebalance_strategy` argument determines how Kafka distributes topic partitions among the consumers in the group during rebalances. 
Supported strategies are:
- `range`: This strategy assigns partitions to consumers based on a range. 
  It aims to distribute partitions evenly across consumers, but it can lead to uneven distribution if the number of partitions is not a multiple of the number of consumers. 
  For more information, refer to the Kafka RangeAssignor documentation, see [RangeAssignor][].
- `roundrobin`: This strategy assigns partitions to consumers in a round-robin fashion. 
  It ensures a more even distribution of partitions across consumers, especially when the number of partitions is not a multiple of the number of consumers. 
  For more information, refer to the Kafka RoundRobinAssignor documentation, see [RoundRobinAssignor][].
- `sticky`: This strategy aims to maintain the same partition assignments during rebalances as much as possible. 
  It minimizes the number of partition movements, which can be beneficial for stateful consumers. 
  For more information, refer to the Kafka StickyAssignor documentation, see [StickyAssignor][].

Using a `group_instance_id` is useful for stateful consumers or when you need to ensure that a specific consumer instance is always assigned the same set of partitions.
- If `group_instance_id` is set to a non-empty string, the consumer is treated as a static member of the group. 
  This means that the consumer will maintain its partition assignments across restarts and rebalances, as long as it rejoins the group with the same `group_instance_id`.
- If `group_instance_id` is set to an empty string (or not set), the consumer is treated as a dynamic member. 
  In this case, the consumer's partition assignments may change during rebalances.

[RangeAssignor]: https://kafka.apache.org/31/javadoc/org/apache/kafka/clients/consumer/RangeAssignor.html
[RoundRobinAssignor]: https://kafka.apache.org/31/javadoc/org/apache/kafka/clients/consumer/RoundRobinAssignor.html
[StickyAssignor]: https://kafka.apache.org/31/javadoc/org/apache/kafka/clients/consumer/StickyAssignor.html

## Blocks

The following blocks are supported inside the definition of
`otelcol.receiver.kafka`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
logs | [logs][] | Configures receive logs from Kafka brokers. | no
metrics | [metrics][] | Configures receive metrics from Kafka brokers. | no
traces | [traces][] | Configures receive traces from Kafka brokers. | no
authentication | [authentication][] | Configures authentication for connecting to Kafka brokers. | no
authentication > plaintext | [plaintext][] | (Deprecated) Authenticates against Kafka brokers with plaintext. | no
authentication > sasl | [sasl][] | Authenticates against Kafka brokers with SASL. | no
authentication > sasl > aws_msk | [aws_msk][] | Additional SASL parameters when using AWS_MSK_IAM. | no
authentication > tls | [tls][] | (Deprecated) Configures TLS for connecting to the Kafka brokers. | no
authentication > kerberos | [kerberos][] | Authenticates against Kafka brokers with Kerberos. | no
metadata | [metadata][] | Configures how to retrieve metadata from Kafka brokers. | no
metadata > retry | [retry][] | Configures how to retry metadata retrieval. | no
autocommit | [autocommit][] | Configures how to automatically commit updated topic offsets to back to the Kafka brokers. | no
message_marking | [message_marking][] | Configures when Kafka messages are marked as read. | no
header_extraction | [header_extraction][] | Extract headers from Kafka records. | no
tls | [tls][] | Configures TLS for connecting to the Kafka brokers. | no
debug_metrics | [debug_metrics][] | Configures the metrics which this component generates to monitor its state. | no
error_backoff | [error_backoff][] | Configures how to handle errors when receiving messages from Kafka. | no
output | [output][] | Configures where to send received telemetry data. | yes

The `>` symbol indicates deeper levels of nesting. For example,
`authentication > tls` refers to a `tls` block defined inside an
`authentication` block.

[logs]: #logs-block
[metrics]: #metrics-block
[traces]: #traces-block
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
[error_backoff]: #error_backoff-block

### logs block

The `logs` block configures how to send logs to Kafka brokers.

Name       | Type     | Description                                                                | Default        | Required
---------- | -------- | -------------------------------------------------------------------------- | -------------- | --------
`topic`    | `string` | The name of the Kafka topic to which logs will be exported.                | `"otlp_logs"`  | no
`encoding` | `string` | The encoding for logs. See [Supported encodings](#supported-encodings).    | `"otlp_proto"` | no

### metrics block

The `logs` block configures how to send metrics to Kafka brokers.

Name       | Type     | Description                                                                | Default           | Required
---------- | -------- | -------------------------------------------------------------------------- | ----------------- | --------
`topic`    | `string` | The name of the Kafka topic to which metrics will be exported.             | `"otlp_metrics"`  | no
`encoding` | `string` | The encoding for logs. See [Supported encodings](#supported-encodings).    | `"otlp_proto"`    | no

### traces block

The `logs` block configures how to send traces to Kafka brokers.

Name       | Type     | Description                                                                | Default          | Required
---------- | -------- | -------------------------------------------------------------------------- | ---------------- | --------
`topic`    | `string` | The name of the Kafka topic to which traces will be exported.              | `"otlp_traces"`  | no
`encoding` | `string` | The encoding for logs. See [Supported encodings](#supported-encodings).    | `"otlp_proto"`   | no

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

### error_backoff block

The `error_backoff` block configures how failed requests to Kafka are retried.

The following arguments are supported:

Name                   | Type       | Description                                            | Default | Required
-----------------------|------------|--------------------------------------------------------|---------|---------
`enabled`              | `boolean`  | Enables retrying failed requests.                      | `false` | no
`initial_interval`     | `duration` | Initial time to wait before retrying a failed request. | `"0s"`  | no
`max_elapsed_time`     | `duration` | Maximum time to wait before discarding a failed batch. | `"0s"`  | no
`max_interval`         | `duration` | Maximum time to wait between retries.                  | `"0s"`  | no
`multiplier`           | `number`   | Factor to grow wait time before retrying.              | `0`     | no
`randomization_factor` | `number`   | Factor to randomize wait time before retrying.         | `0`     | no

When `enabled` is `true`, failed batches are retried after a given interval.
The `initial_interval` argument specifies how long to wait before the first retry attempt.
If requests continue to fail, the time to wait before retrying increases by the factor specified by the `multiplier` argument, which must be greater than `1.0`.
The `max_interval` argument specifies the upper bound of how long to wait between retries.

The `randomization_factor` argument is useful for adding jitter between retrying Alloy instances.
If `randomization_factor` is greater than `0`, the wait time before retries is multiplied by a random factor in the range `[ I - randomization_factor * I, I + randomization_factor * I]`, where `I` is the current interval.

If a batch hasn't been sent successfully, it's discarded after the time specified by `max_elapsed_time` elapses.
If `max_elapsed_time` is set to `"0s"`, failed requests are retried forever until they succeed.

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.kafka` does not export any fields.

## Supported encodings

`otelcol.receiver.kafka` supports encoding extensions, as well as the following built-in encodings.

Available for all signals:

- `otlp_proto`: the payload is decoded as OTLP Protobuf
- `otlp_json`: the payload is decoded as OTLP JSON

Available only for traces:

- `jaeger_proto`: the payload is deserialized to a single Jaeger proto `Span`.
- `jaeger_json`: the payload is deserialized to a single Jaeger JSON Span using `jsonpb`.
- `zipkin_proto`: the payload is deserialized into a list of Zipkin proto spans.
- `zipkin_json`: the payload is deserialized into a list of Zipkin V2 JSON spans.
- `zipkin_thrift`: the payload is deserialized into a list of Zipkin Thrift spans.

Available only for logs:

- `raw`: the payload's bytes are inserted as the body of a log record.
- `text`: the payload are decoded as text and inserted as the body of a log record. By default, it uses UTF-8 to decode. You can use `text_<ENCODING>`, like `text_utf-8`, `text_shift_jis`, etc., to customize this behavior.
- `json`: the payload is decoded as JSON and inserted as the body of a log record.
- `azure_resource_logs`: the payload is converted from Azure Resource Logs format to OTel format.

## Message header propagation

`otelcol.receiver.kafka` will extract Kafka message headers and include them as request metadata (context).
This metadata can then be used throughout the pipeline, for example to set attributes using `otelcol.processor.attributes`.

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
    endpoint = sys.env("OTLP_ENDPOINT")
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
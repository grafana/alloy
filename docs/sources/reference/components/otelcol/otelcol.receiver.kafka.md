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
`topic` | `string` | Kafka topic to read from. | `"otlp_spans"` | no
`encoding` | `string` | Encoding of payload read from Kafka. | `"otlp_proto"` | no
`group_id` | `string` | Consumer group to consume messages from. | `"otel-collector"` | no
`client_id` | `string` | Consumer client ID to use. | `"otel-collector"` | no
`initial_offset` | `string` | Initial offset to use if no offset was previously committed. | `"latest"` | no
`resolve_canonical_bootstrap_servers_only` | `bool` | Whether to resolve then reverse-lookup broker IPs during startup. | `"false"` | no

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

If the `include_all_topics` argument is `true`, `otelcol.receiver.kafka`
maintains a full set of metadata for all topics rather than the minimal set
that has been necessary so far. Including the full set of metadata is more
convenient for users but can consume a substantial amount of memory if you have
many topics and partitions.

Retrieving metadata may fail if the Kafka broker is starting up at the same
time as the `otelcol.receiver.kafka` component. The [`retry` child
block][retry] can be provided to customize retry behavior.

### retry block

The `retry` block configures how to retry retrieving metadata when retrieval
fails.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`max_retries` | `number` | How many times to reattempt retrieving metadata. | `3` | no
`backoff` | `duration` | Time to wait between retries. | `"250ms"` | no

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
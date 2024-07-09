---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.kafka/
aliases:
  - ../loki.source.kafka/ # /docs/alloy/latest/reference/components/loki.source.kafka/
description: Learn about loki.source.kafka
title: loki.source.kafka
---

# loki.source.kafka

`loki.source.kafka` reads messages from Kafka using a consumer group
and forwards them to other `loki.*` components.

The component starts a new Kafka consumer group for the given arguments
and fans out incoming entries to the list of receivers in `forward_to`.

Before using `loki.source.kafka`, Kafka should have at least one producer
writing events to at least one topic. Follow the steps in the
[Kafka Quick Start](https://kafka.apache.org/documentation/#quickstart)
to get started with Kafka.

Multiple `loki.source.kafka` components can be specified by giving them different labels.

## Usage

```alloy
loki.source.kafka "LABEL" {
  brokers    = BROKER_LIST
  topics     = TOPIC_LIST
  forward_to = RECEIVER_LIST
}
```

## Arguments

`loki.source.kafka` supports the following arguments:

 Name                     | Type                 | Description                                              | Default               | Required
--------------------------|----------------------|----------------------------------------------------------|-----------------------|----------
 `brokers`                | `list(string)`       | The list of brokers to connect to Kafka.                 |                       | yes
 `topics`                 | `list(string)`       | The list of Kafka topics to consume.                     |                       | yes
 `group_id`               | `string`             | The Kafka consumer group id.                             | `"loki.source.kafka"` | no
 `assignor`               | `string`             | The consumer group rebalancing strategy to use.          | `"range"`             | no
 `version`                | `string`             | Kafka version to connect to.                             | `"2.2.1"`             | no
 `use_incoming_timestamp` | `bool`               | Whether or not to use the timestamp received from Kafka. | `false`               | no
 `labels`                 | `map(string)`        | The labels to associate with each received Kafka event.  | `{}`                  | no
 `forward_to`             | `list(LogsReceiver)` | List of receivers to send log entries to.                |                       | yes
 `relabel_rules`          | `RelabelRules`       | Relabeling rules to apply on log entries.                | `{}`                  | no

`assignor` values can be either `"range"`, `"roundrobin"`, or `"sticky"`.

Labels from the `labels` argument are applied to every message that the component reads.

The `relabel_rules` field can make use of the `rules` export value from a
[loki.relabel][] component to apply one or more relabeling rules to log entries
before they're forwarded to the list of receivers in `forward_to`.

In addition to custom labels, the following internal labels prefixed with `__` are available:

- `__meta_kafka_message_key`
- `__meta_kafka_message_offset`
- `__meta_kafka_topic`
- `__meta_kafka_partition`
- `__meta_kafka_member_id`
- `__meta_kafka_group_id`

All labels starting with `__` are removed prior to forwarding log entries. To
keep these labels, relabel them using a [loki.relabel][] component and pass its
`rules` export to the `relabel_rules` argument.

[loki.relabel]: ../loki.relabel/

## Blocks

The following blocks are supported inside the definition of `loki.source.kafka`:

 Hierarchy                                   | Name             | Description                                               | Required
---------------------------------------------|------------------|-----------------------------------------------------------|----------
 authentication                              | [authentication] | Optional authentication configuration with Kafka brokers. | no
 authentication > tls_config                 | [tls_config]     | Optional authentication configuration with Kafka brokers. | no
 authentication > sasl_config                | [sasl_config]    | Optional authentication configuration with Kafka brokers. | no
 authentication > sasl_config > tls_config   | [tls_config]     | Optional authentication configuration with Kafka brokers. | no
 authentication > sasl_config > oauth_config | [oauth_config]   | Optional authentication configuration with Kafka brokers. | no

[authentication]: #authentication-block
[tls_config]: #tls_config-block
[sasl_config]: #sasl_config-block
[oauth_config]: #oauth_config-block

### authentication block

The `authentication` block defines the authentication method when communicating with the Kafka event brokers.

 Name   | Type     | Description             | Default  | Required
--------|----------|-------------------------|----------|----------
 `type` | `string` | Type of authentication. | `"none"` | no

`type` supports the values `"none"`, `"ssl"`, and `"sasl"`. If `"ssl"` is used,
you must set the `tls_config` block. If `"sasl"` is used, you must set the `sasl_config` block.

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### sasl_config block

The `sasl_config` block defines the listen address and port where the listener
expects Kafka messages to be sent to.

 Name        | Type     | Description                                                                   | Default    | Required
-------------|----------|--------------------------------------------------------------------|----------|-----------------------
 `mechanism` | `string` | Specifies the SASL mechanism the client uses to authenticate with the broker. | `"PLAIN""` | no
 `user`      | `string` | The user name to use for SASL authentication.                                 | `""`       | no
 `password`  | `secret` | The password to use for SASL authentication.                                  | `""`       | no
 `use_tls`   | `bool`   | If true, SASL authentication is executed over TLS.                            | `false`    | no

### oauth_config block

The `oauth_config` is required when the SASL mechanism is set to `OAUTHBEARER`.

 Name             | Type           | Description                                                            | Default | Required
------------------|----------------|------------------------------------------------------------------------|---------|----------
 `token_provider` | `string`       | The OAuth provider to be used. The only supported provider is `azure`. | `""`    | yes
 `scopes`         | `list(string)` | The scopes to set in the access token                                  | `[]`    | yes

## Exported fields

`loki.source.kafka` does not export any fields.

## Component health

`loki.source.kafka` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.source.kafka` does not expose additional debug info.

## Example

This example consumes Kafka events from the specified brokers and topics
then forwards them to a `loki.write` component using the Kafka timestamp.

```alloy
loki.source.kafka "local" {
  brokers                = ["localhost:9092"]
  topics                 = ["quickstart-events"]
  labels                 = {component = "loki.source.kafka"}
  forward_to             = [loki.relabel.kafka.receiver]
  use_incoming_timestamp = true
  relabel_rules          = loki.relabel.kafka.rules
}

loki.relabel "kafka" {
  forward_to      = [loki.write.local.receiver]

  rule {
    source_labels = ["__meta_kafka_topic"]
    target_label  = "topic"
  }
}

loki.write "local" {
  endpoint {
    url = "loki:3100/api/v1/push"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.kafka` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

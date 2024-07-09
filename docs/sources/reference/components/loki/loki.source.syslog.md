---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.syslog/
aliases:
  - ../loki.source.syslog/ # /docs/alloy/latest/reference/components/loki.source.syslog/
description: Learn about loki.source.syslog
title: loki.source.syslog
---

# loki.source.syslog

`loki.source.syslog` listens for syslog messages over TCP or UDP connections
and forwards them to other `loki.*` components. The messages must be compliant
with the [RFC5424](https://www.rfc-editor.org/rfc/rfc5424) format.

The component starts a new syslog listener for each of the given `config`
blocks and fans out incoming entries to the list of receivers in `forward_to`.

Multiple `loki.source.syslog` components can be specified by giving them different labels.

## Usage

```alloy
loki.source.syslog "LABEL" {
  listener {
    address = "LISTEN_ADDRESS"
  }
  ...

  forward_to = RECEIVER_LIST
}
```

## Arguments

`loki.source.syslog` supports the following arguments:

Name            | Type                 | Description                               | Default | Required
----------------|----------------------|-------------------------------------------|---------|---------
`forward_to`    | `list(LogsReceiver)` | List of receivers to send log entries to. |         | yes
`relabel_rules` | `RelabelRules`       | Relabeling rules to apply on log entries. | "{}"    | no

The `relabel_rules` field can make use of the `rules` export value from a
[loki.relabel][] component to apply one or more relabeling rules to log entries
before they're forwarded to the list of receivers in `forward_to`.

[loki.relabel]: ../loki.relabel/

## Blocks

The following blocks are supported inside the definition of
`loki.source.syslog`:

Hierarchy             | Name           | Description                                                                 | Required
----------------------|----------------|-----------------------------------------------------------------------------|---------
listener              | [listener][]   | Configures a listener for IETF Syslog (RFC5424) messages.                   | no
listener > tls_config | [tls_config][] | Configures TLS settings for connecting to the endpoint for TCP connections. | no

The `>` symbol indicates deeper levels of nesting. For example, `config > tls_config`
refers to a `tls_config` block defined inside a `config` block.

[listener]: #listener-block
[tls_config]: #tls_config-block

### listener block

The `listener` block defines the listen address and protocol where the listener
expects syslog messages to be sent to, as well as its behavior when receiving
messages.

The following arguments can be used to configure a `listener`. Only the
`address` field is required and any omitted fields take their default
values.

Name                     | Type          | Description                                                                   | Default  | Required
-------------------------|---------------|-------------------------------------------------------------------------------|----------|---------
`address`                | `string`      | The `<host:port>` address to listen to for syslog messages.                   |          | yes
`protocol`               | `string`      | The protocol to listen to for syslog messages. Must be either `tcp` or `udp`. | `tcp`    | no
`idle_timeout`           | `duration`    | The idle timeout for tcp connections.                                         | `"120s"` | no
`label_structured_data`  | `bool`        | Whether to translate syslog structured data to loki labels.                   | `false`  | no
`labels`                 | `map(string)` | The labels to associate with each received syslog record.                     | `{}`     | no
`use_incoming_timestamp` | `bool`        | Whether to set the timestamp to the incoming syslog record timestamp.         | `false`  | no
`use_rfc5424_message`    | `bool`        | Whether to forward the full RFC5424-formatted syslog message.                 | `false`  | no
`max_message_length`     | `int`         | The maximum limit to the length of syslog messages.                           | `8192`   | no

By default, the component assigns the log entry timestamp as the time it was processed.

The `labels` map is applied to every message that the component reads.

All header fields from the parsed RFC5424 messages are brought in as
internal labels, prefixed with `__syslog_`.

If `label_structured_data` is set, structured data in the syslog header is also translated to internal labels in the form of `__syslog_message_sd_<ID>_<KEY>`.
For example, a  structured data entry of `[example@99999 test="yes"]` becomes the label `__syslog_message_sd_example_99999_test` with the value `"yes"`.

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`loki.source.syslog` does not export any fields.

## Component health

`loki.source.syslog` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`loki.source.syslog` exposes some debug information per syslog listener:
* Whether the listener is currently running.
* The listen address.
* The labels that the listener applies to incoming log entries.

## Debug metrics
* `loki_source_syslog_entries_total` (counter): Total number of successful entries sent to the syslog component.
* `loki_source_syslog_parsing_errors_total` (counter): Total number of parsing errors while receiving syslog messages.
* `loki_source_syslog_empty_messages_total` (counter): Total number of empty messages received from the syslog component.

## Example

This example listens for Syslog messages in valid RFC5424 format over TCP and
UDP in the specified ports and forwards them to a `loki.write` component.

```alloy
loki.source.syslog "local" {
  listener {
    address  = "127.0.0.1:51893"
    labels   = { component = "loki.source.syslog", protocol = "tcp" }
  }

  listener {
    address  = "127.0.0.1:51898"
    protocol = "udp"
    labels   = { component = "loki.source.syslog", protocol = "udp"}
  }

  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "loki:3100/api/v1/push"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.syslog` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

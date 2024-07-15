---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.gelf/
aliases:
  - ../loki.source.gelf/ # /docs/alloy/latest/reference/components/loki.source.gelf/
description: Learn about loki.source.gelf
title: loki.source.gelf
---

# loki.source.gelf

`loki.source.gelf` reads [Graylog Extended Long Format (GELF) logs](https://github.com/Graylog2/graylog2-server) from a UDP listener and forwards them to other
`loki.*` components.

Multiple `loki.source.gelf` components can be specified by giving them
different labels and ports.

## Usage

```alloy
loki.source.gelf "LABEL" {
  forward_to    = RECEIVER_LIST
}
```

## Arguments
The component starts a new UDP listener and fans out
log entries to the list of receivers passed in `forward_to`.

`loki.source.gelf` supports the following arguments:

Name                     | Type           | Description                                                                | Default         | Required
-------------------------|----------------|----------------------------------------------------------------------------|-----------------|---------
`listen_address`         | `string`       | UDP address and port to listen for Graylog messages.                       | `0.0.0.0:12201` | no
`use_incoming_timestamp` | `bool`         | When false, assigns the current timestamp to the log when it was processed | `false`         | no
`relabel_rules`          | `RelabelRules` | Relabeling rules to apply on log entries.                                  | "{}"            | no


> **NOTE**: GELF logs can be sent uncompressed or compressed with GZIP or ZLIB.
> A `job` label is added with the full name of the component `loki.source.gelf.LABEL`.

The `relabel_rules` argument can make use of the `rules` export from a
[loki.relabel][] component to apply one or more relabling rules to log entries
before they're forward to the list of receivers specified in `forward_to`.

Incoming messages have the following internal labels available:

* `__gelf_message_level`: The GELF level as a string.
* `__gelf_message_host`: The host sending the GELF message.
* `__gelf_message_host`: The GELF level message version sent by the client.
* `__gelf_message_facility`: The GELF facility.

All labels starting with `__` are removed prior to forwarding log entries. To
keep these labels, relabel them using a [loki.relabel][] component and pass its
`rules` export to the `relabel_rules` argument.

[loki.relabel]: ../loki.relabel/

## Component health

`loki.source.gelf` is only reported as unhealthy if given an invalid
configuration.

## Debug Metrics

* `gelf_target_entries_total` (counter): Total number of successful entries sent to the GELF target.
* `gelf_target_parsing_errors_total` (counter): Total number of parsing errors while receiving GELF messages.

## Example

```alloy
loki.relabel "gelf" {
  rule {
    source_labels = ["__gelf_message_host"]
    target_label  = "host"
  }
}

loki.source.gelf "listen"  {
  forward_to    = [loki.write.endpoint.receiver]
  relabel_rules = loki.relabel.gelf.rules
}

loki.write "endpoint" {
  endpoint {
    url ="loki:3100/api/v1/push"
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.gelf` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

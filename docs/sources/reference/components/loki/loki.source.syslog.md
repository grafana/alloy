---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.syslog/
aliases:
  - ../loki.source.syslog/ # /docs/alloy/latest/reference/components/loki.source.syslog/
description: Learn about loki.source.syslog
labels:
  stage: general-availability
  products:
    - oss
title: loki.source.syslog
---

# `loki.source.syslog`

`loki.source.syslog` listens for syslog messages over TCP or UDP connections and forwards them to other `loki.*` components.
The messages must be compliant with the [RFC5424](https://www.rfc-editor.org/rfc/rfc5424) syslog protocol or the [RFC3164](https://datatracker.ietf.org/doc/html/rfc3164) BSD syslog protocol.
For a detailed example, refer to the [Monitor RFC5424-compliant syslog messages with Grafana Alloy](https://grafana.com/docs/alloy/latest/monitor/monitor-syslog-messages/) scenario.

{{< admonition type="note" >}}
If your messages aren't RFC5424 compliant, you can use `raw` syslog format in combination with the [`loki.process`](./loki.process.md) component.

Please note, that the `raw` syslog format is an [experimental][] feature.

[experimental]: https://grafana.com/docs/release-life-cycle/
{{< /admonition >}}

The component starts a new syslog listener for each of the given `config` blocks and fans out incoming entries to the list of receivers in `forward_to`.

You can specify multiple `loki.source.syslog` components by giving them different labels.

## Usage

```alloy
loki.source.syslog "<LABEL>" {
  listener {
    address = "<LISTEN_ADDRESS>"
  }
  ...

  forward_to = <RECEIVER_LIST>
}
```

## Arguments

You can use the following arguments with `loki.source.syslog`:

| Name            | Type                 | Description                               | Default | Required |
|-----------------|----------------------|-------------------------------------------|---------|----------|
| `forward_to`    | `list(LogsReceiver)` | List of receivers to send log entries to. |         | yes      |
| `relabel_rules` | `RelabelRules`       | Relabeling rules to apply on log entries. | `{}`    | no       |

The `relabel_rules` field can make use of the `rules` export value from a [`loki.relabel`][loki.relabel] component to apply one or more relabeling rules to log entries before they're forwarded to the list of receivers in `forward_to`.

`loki.source.syslog` applies the following labels to log entries from the client information if possible.

* `__syslog_connection_ip_address`
* `__syslog_connection_hostname`

`loki.source.syslog` applies the following labels to log entries if they have been parsed from the syslog message.

* `__syslog_message_severity`
* `__syslog_message_facility`
* `__syslog_message_hostname`
* `__syslog_message_app_name`
* `__syslog_message_proc_id`
* `__syslog_message_msg_id`
* `__syslog_message_msg_counter`
* `__syslog_message_sequence`

If there is [RFC5424](https://www.rfc-editor.org/rfc/rfc5424) compliant structured data in the parsed message, it will be applied to the log entry as a label with prefix `__syslog_message_sd_`.
For example, if the structured data provided is `[example@99999 test="value"]`, the log entry will have the label `__syslog_message_sd_example_99999_test` with a value of `value`.

Before passing log entries to the next component in the pipeline, the syslog source will remove any labels with a `__` prefix.
To retain the `__syslog_` labels on the log entries, you must use rules in the `relabel_rules` argument to move them to labels that do not have a `__` prefix.
The following relabel example retains all `__syslog_` labels on the log entry when the entries are passed to the next component in the pipeline.

```alloy
loki.relabel "syslog" {
  rule {
    action = "labelmap"
    regex = "__syslog_(.+)"
  }
}
```

[loki.relabel]: ../loki.relabel/

## Blocks

You can use the following blocks with `loki.source.syslog`:

| Name                                                        | Description                                                                 | Required |
|-------------------------------------------------------------|-----------------------------------------------------------------------------|----------|
| [`listener`][listener]                                      | Configures a listener for Syslog messages.                                  | no       |
| `listener` > [`raw_format_options`][raw_format_options]     | Configures `raw` syslog format behavior.                                    | no       |
| `listener` > [`rfc3164_cisco_components`][cisco_components] | Configures parsing of non-standard Cisco IOS syslog extensions.             | no       |
| `listener` > [`tls_config`][tls_config]                     | Configures TLS settings for connecting to the endpoint for TCP connections. | no       |

The > symbol indicates deeper levels of nesting.
For example, `listener` > `tls_config` refers to a `tls_config` block defined inside a `listener` block.

[listener]: #listener
[tls_config]: #tls_config
[raw_format_options]: #raw_format_options
[cisco_components]: #rfc3164_cisco_components

### `listener`

The `listener` block defines the listen address and protocol where the listener expects syslog messages to be sent to, as well as its behavior when receiving messages.

The following arguments can be used to configure a `listener`.
Only the `address` field is required and any omitted fields take their default values.

| Name                              | Type          | Description                                                                            | Default     | Required |
|-----------------------------------|---------------|----------------------------------------------------------------------------------------|-------------|----------|
| `address`                         | `string`      | The `<host:port>` address to listen to for syslog messages.                            |             | yes      |
| `idle_timeout`                    | `duration`    | The idle timeout for TCP connections.                                                  | `"120s"`    | no       |
| `label_structured_data`           | `bool`        | Whether to translate syslog structured data to Loki labels.                            | `false`     | no       |
| `labels`                          | `map(string)` | The labels to associate with each received syslog record.                              | `{}`        | no       |
| `max_message_length`              | `int`         | The maximum limit to the length of syslog messages.                                    | `8192`      | no       |
| `protocol`                        | `string`      | The protocol to listen to for syslog messages. Must be either `tcp` or `udp`.          | `"tcp"`     | no       |
| `rfc3164_default_to_current_year` | `bool`        | Whether to default the incoming timestamp of an `rfc3164` message to the current year. | `false`     | no       |
| `syslog_format`                   | `string`      | The format for incoming messages. See [supported formats](#supported-formats).         | `"rfc5424"` | no       |
| `use_incoming_timestamp`          | `bool`        | Whether to set the timestamp to the incoming syslog record timestamp.                  | `false`     | no       |
| `use_rfc5424_message`             | `bool`        | Whether to forward the full RFC5424-formatted syslog message.                          | `false`     | no       |

By default, the component assigns the log entry timestamp as the time it was processed.

The `labels` map is applied to every message that the component reads.

All header fields from the parsed RFC5424 messages are brought in as internal labels, prefixed with `__syslog_`.

If `label_structured_data` is set, structured data in the syslog header is also translated to internal labels in the form of `__syslog_message_sd_<ID>_<KEY>`.
For example, a  structured data entry of `[example@99999 test="yes"]` becomes the label `__syslog_message_sd_example_99999_test` with the value `"yes"`.

The `rfc3164_default_to_current_year` argument is only relevant when `use_incoming_timestamp` is also set to `true`.
`rfc3164` message timestamps don't contain a year, and this component's default behavior is to mimic Promtail behavior and leave the year as 0.
Setting `rfc3164_default_to_current_year` to `true` sets the year of the incoming timestamp to the current year using the local time of the {{< param "PRODUCT_NAME" >}} instance.

{{< admonition type="note" >}}
The `rfc3164_default_to_current_year`, `use_incoming_timestamp` and `use_rfc5424_message` fields cannot be used when `syslog_format` is set to `raw`.
{{< /admonition >}}

#### Supported formats

* **`rfc3164`**
  A legacy syslog format, also known as BSD syslog.
  Example: `<34>Oct 11 22:14:15 my-server-01 sshd[1234]: Failed password for root from 192.168.1.10 port 22 ssh2`
* **`rfc5424`**
  A modern, structured syslog format. Uses ISO 8601 for timestamps.
  Example: `<165>1 2025-12-18T00:33:00Z web01 nginx - - [audit@123 id="456"] Login failed`.
* **`raw`**
  Disables log line parsing. This format allows receiving non-RFC5424 compliant logs, such as [CEF][cef].
  Raw logs can be forwarded to [`loki.process`](./loki.process.md) component for parsing.

{{< admonition type="note" >}}
The `raw` format is an [experimental][] feature.
Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.
To enable and use an experimental feature, you must set the `stability.level` [flag][] to `experimental`.
{{< /admonition >}}

[flag]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/cli/run/
[experimental]: https://grafana.com/docs/release-life-cycle/

[cef]: https://www.splunk.com/en_us/blog/learn/common-event-format-cef.html

### `raw_format_options`

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `raw_format_options` block configures the `raw` syslog format behavior.

{{< admonition type="note" >}}
This block can only be used when you set `syslog_format` to `raw`.
{{< /admonition >}}

The following argument is supported:

| Name                            | Type   | Description                                                                 | Default | Required |
|---------------------------------|--------|-----------------------------------------------------------------------------|---------|----------|
| `use_null_terminator_delimiter` | `bool` | Use null-terminator (`\0`) instead of line break (`\n`) to split log lines. | `false` | no       |

### `rfc3164_cisco_components`

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `rfc3164_cisco_components` configures parsing of non-standard Cisco IOS syslog extensions. 

{{< admonition type="note" >}}
This block can only be used when you set `syslog_format` to `rfc3164`.
{{< /admonition >}}

The following argument is supported:

| Name               | Type   | Description                                     | Default | Required |
|--------------------|--------|-------------------------------------------------|---------|----------|
| `enable_all`       | `bool` | Enables all components below.                   | `false` | no       |
| `message_counter`  | `bool` | Enables syslog message counter field parsing.   | `false` | no       |
| `sequence_number`  | `bool` | Enables service sequence number field parsing.  | `false` | no       |
| `hostname`         | `bool` | Enables origin hostname fleld parsing.          | `false` | no       |
| `second_fractions` | `bool` | Enables miliseconds parsing in timestamp field. | `false` | no       |

{{< admonition type="note" >}}
At-least one option has to be enabled if `enable_all` is set to `false`.
{{< /admonition >}}

{{< admonition type="caution" >}}
The `rfc3164_cisco_components` configuration must match your Cisco device configuration. 
The `loki.source.syslog` component cannot auto-detect which components are present because they share similar formats.
{{< /admonition >}}

#### Cisco Device Configuration

```
conf t

! Enable message counter (on by default for remote logging)
logging host 10.0.0.10

! Add service sequence numbers
service sequence-numbers

! Add origin hostname
logging origin-id hostname

! Enable millisecond timestamps
service timestamps log datetime msec localtime

! Recommended: Enable NTP to remove asterisk
ntp server <your-ntp-server>
```

#### Current Limitations

* **Component Ordering**: When Cisco components are selectively disabled on the device but the parser expects them, parsing will fail or produce incorrect results. 
  Always match your parser configuration to your device configuration.
* **Structured Data**: Messages with RFC5424-style structured data blocks (from `logging host X session-id` or `sequence-num-session`) are not currently supported.
  See the [upstream issue][go-syslog-issue] for details.

[go-syslog-issue]: https://github.com/leodido/go-syslog/issues/35

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`loki.source.syslog` doesn't export any fields.

## Component health

`loki.source.syslog` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`loki.source.syslog` exposes some debug information per syslog listener:

* Whether the listener is running.
* The listen address.
* The labels that the listener applies to incoming log entries.

## Debug metrics

* `loki_source_syslog_empty_messages_total` (counter): Total number of empty messages received from the syslog component.
* `loki_source_syslog_entries_total` (counter): Total number of successful entries sent to the syslog component.
* `loki_source_syslog_parsing_errors_total` (counter): Total number of parsing errors while receiving syslog messages.

## Example

This example listens for syslog messages in valid RFC5424 format over TCP and UDP in the specified ports and forwards them to a `loki.write` component.

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

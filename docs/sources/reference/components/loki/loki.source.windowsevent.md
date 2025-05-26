---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.windowsevent/
aliases:
  - ../loki.source.windowsevent/ # /docs/alloy/latest/reference/components/loki.source.windowsevent/
description: Learn about loki.windowsevent
labels:
  stage: general-availability
  products:
    - oss
title: loki.source.windowsevent
---

# `loki.source.windowsevent`

`loki.source.windowsevent` reads events from Windows Event Logs and forwards them to other `loki.*` components.

You can specify multiple `loki.source.windowsevent` components by giving them different labels.

## Usage

```alloy
loki.source.windowsevent "<LABEL>" {
  eventlog_name = "<EVENTLOG_NAME>"
  forward_to    = <RECEIVER_LIST>
}
```

## Arguments

The component starts a reader and fans out log entries to the list of receivers passed in `forward_to`.

You can use the following arguments with `loki.source.windowsevent`:

| Name                     | Type                 | Description                                                 | Default                    | Required  |
| ------------------------ | -------------------- | ----------------------------------------------------------- | -------------------------- | --------- |
| `forward_to`             | `list(LogsReceiver)` | List of receivers to send log entries to.                   |                            | yes       |
| `eventlog_name`          | `string`             | Event log to read from.                                     |                            | See below |
| `xpath_query`            | `string`             | XPath query to select events.                               | `"*"`                      | See below |
| `bookmark_path`          | `string`             | Keeps position in event log.                                | `"DATA_PATH/bookmark.xml"` | no        |
| `exclude_event_data`     | `bool`               | Exclude event data.                                         | `false`                    | no        |
| `exclude_event_message`  | `bool`               | Exclude the human-friendly event message.                   | `false`                    | no        |
| `exclude_user_data`      | `bool`               | Exclude user data.                                          | `false`                    | no        |
| `labels`                 | `map(string)`        | The labels to associate with incoming logs.                 | `{}`                       | no        |
| `locale`                 | `number`             | Locale ID for event rendering. 0 default is Windows Locale. | `0`                        | no        |
| `poll_interval`          | `duration`           | How often to poll the event log.                            | `"3s"`                     | no        |
| `use_incoming_timestamp` | `bool`               | When false, assigns the current timestamp to the log.       | `false`                    | no        |

{{< admonition type="note" >}}
`eventlog_name` is required if `xpath_query` doesn't specify the event log.
You can define `xpath_query` in [short or XML form](https://docs.microsoft.com/windows/win32/wes/consuming-events).
When you use the XML form you can specify `event_log` in the `xpath_query`.
If you use the short form, you must define `eventlog_name`.
{{< /admonition >}}

{{< admonition type="note" >}}
`legacy_bookmark_path` converts the legacy Grafana Agent Static bookmark to a {{< param "PRODUCT_NAME" >}} bookmark, if `bookmark_path` doesn't exist.
{{< /admonition >}}

## Blocks

The `loki.source.windowsevent` component doesn't support any blocks. You can configure this component with arguments.

## Component health

`loki.source.windowsevent` is only reported as unhealthy if given an invalid configuration.

## Example

This example collects log entries from the Event Log specified in `eventlog_name` and forwards them to a `loki.write` component.

```alloy
loki.source.windowsevent "application"  {
    eventlog_name = "Application"
    forward_to = [loki.write.endpoint.receiver]
}

loki.write "endpoint" {
    endpoint {
        url ="loki:3100/api/v1/push"
    }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.windowsevent` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/loki.source.windowsevent/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/loki.source.windowsevent/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/loki.source.windowsevent/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/loki.source.windowsevent/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/loki.source.windowsevent/
description: Learn about loki.windowsevent
title: loki.source.windowsevent
---

# loki.source.windowsevent

`loki.source.windowsevent` reads events from Windows Event Logs and forwards them to other
`loki.*` components.

Multiple `loki.source.windowsevent` components can be specified by giving them
different labels.

## Usage

```river
loki.source.windowsevent "LABEL" {
  eventlog_name = EVENTLOG_NAME
  forward_to    = RECEIVER_LIST
}
```

## Arguments
The component starts a new reader and fans out
log entries to the list of receivers passed in `forward_to`.

`loki.source.windowsevent` supports the following arguments:

Name                     | Type                 | Description                                                                    | Default                    | Required
------------------------ |----------------------|--------------------------------------------------------------------------------|----------------------------| --------
`locale`                 | `number`             | Locale ID for event rendering. 0 default is Windows Locale.                    | `0`                        | no
`eventlog_name`          | `string`             | Event log to read from.                                                        |                            | See below.
`xpath_query`            | `string`             | Event log to read from.                                                        | `"*"`                      | See below.
`bookmark_path`          | `string`             | Keeps position in event log.                                                   | `"DATA_PATH/bookmark.xml"` | no
`poll_interval`          | `duration`           | How often to poll the event log.                                               | `"3s"`                     | no
`exclude_event_data`     | `bool`               | Exclude event data.                                                            | `false`                    | no
`exclude_user_data`      | `bool`               | Exclude user data.                                                             | `false`                    | no
`exclude_event_message`  | `bool`               | Exclude the human-friendly event message.                                      | `false`                    | no
`use_incoming_timestamp` | `bool`               | When false, assigns the current timestamp to the log when it was processed.    | `false`                    | no
`forward_to`             | `list(LogsReceiver)` | List of receivers to send log entries to.                                      |                            | yes
`labels`                 | `map(string)`        | The labels to associate with incoming logs.                                    |                            | no 


> **NOTE**: `eventlog_name` is required if `xpath_query` does not specify the event log.
> You can define `xpath_query` in [short or xml form](https://docs.microsoft.com/en-us/windows/win32/wes/consuming-events).
> When using the XML form you can specify `event_log` in the `xpath_query`.
> If using short form, you must define `eventlog_name`.


## Component health

`loki.source.windowsevent` is only reported as unhealthy if given an invalid
configuration.

## Example

This example collects log entries from the Event Log specified in `eventlog_name` and
forwards them to a `loki.write` component so they are written to Loki.

```river
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

- Components that export [Loki `LogsReceiver`](../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

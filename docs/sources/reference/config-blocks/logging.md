---
aliases:
- /docs/grafana-cloud/agent/flow/reference/config-blocks/logging/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/config-blocks/logging/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/config-blocks/logging/
- /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/logging/
canonical: https://grafana.com/docs/agent/latest/flow/reference/config-blocks/logging/
description: Learn about the logging configuration block
menuTitle: logging
title: logging block
---

# logging block

`logging` is an optional configuration block used to customize how {{< param "PRODUCT_NAME" >}} produces log messages.
`logging` is specified without a label and can only be provided once per configuration file.

## Example

```river
logging {
  level  = "info"
  format = "logfmt"
}
```

## Arguments

The following arguments are supported:

Name       | Type                 | Description                                | Default    | Required
-----------|----------------------|--------------------------------------------|------------|---------
`level`    | `string`             | Level at which log lines should be written | `"info"`   | no
`format`   | `string`             | Format to use for writing log lines        | `"logfmt"` | no
`write_to` | `list(LogsReceiver)` | List of receivers to send log entries to   |            | no

### Log level

The following strings are recognized as valid log levels:

* `"error"`: Only write logs at the _error_ level.
* `"warn"`: Only write logs at the _warn_ level or above.
* `"info"`: Only write logs at _info_ level or above.
* `"debug"`: Write all logs, including _debug_ level logs.

### Log format

The following strings are recognized as valid log line formats:

* `"logfmt"`: Write logs as [logfmt][] lines.
* `"json"`: Write logs as JSON objects.

[logfmt]: https://brandur.org/logfmt

### Log receivers

The `write_to` argument allows {{< param "PRODUCT_NAME" >}} to tee its log entries to one or more `loki.*` component log receivers in addition to the default [location][].
This, for example can be the export of a `loki.write` component to ship log entries directly to Loki, or a `loki.relabel` component to add a certain label first.

[location]: #log-location

## Log location

{{< param "PRODUCT_NAME" >}} writes all logs to `stderr`.

When running {{< param "PRODUCT_NAME" >}} as a systemd service, view logs written to `stderr` through `journald`.

When running {{< param "PRODUCT_NAME" >}} as a container, view logs written to `stderr` through `docker logs` or `kubectl logs`, depending on whether Docker or Kubernetes was used for deploying {{< param "PRODUCT_NAME" >}}.

When running {{< param "PRODUCT_NAME" >}} as a Windows service, logs are instead written as event logs. You can view the logs through Event Viewer.

In other cases, redirect `stderr` of the {{< param "PRODUCT_NAME" >}} process to a file for logs to persist on disk.

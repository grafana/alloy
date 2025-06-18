---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/logging/
description: Learn about the logging configuration block
labels:
  stage: general-availability
  products:
    - oss
title: logging
---

# `logging`

`logging` is an optional configuration block used to customize how {{< param "PRODUCT_NAME" >}} produces log messages.
`logging` is specified without a label and can only be provided once per configuration file.

## Usage

```alloy
logging {

}
```

## Arguments

You can use the following arguments with `logging`:

| Name       | Type                 | Description                                | Default    | Required |
| ---------- | -------------------- | ------------------------------------------ | ---------- | -------- |
| `format`   | `string`             | Format to use for writing log lines        | `"logfmt"` | no       |
| `level`    | `string`             | Level at which log lines should be written | `"info"`   | no       |
| `write_to` | `list(LogsReceiver)` | List of receivers to send log entries to   | `[]`       | no       |

### Log level

The following strings are recognized as valid log levels:

* `"error"`: Only write logs at the _error_ level.
* `"warn"`: Only write logs at the _warn_ level or above.
* `"info"`: Only write logs at _info_ level or above.
* `"debug"`: Write all logs, including _debug_ level logs.

### Log format

The following strings are recognized as valid log line formats:

* `"json"`: Write logs as JSON objects.
* `"logfmt"`: Write logs as [`logfmt`][logfmt] lines.

### Log receivers

The `write_to` argument allows {{< param "PRODUCT_NAME" >}} to tee its log entries to one or more `loki.*` component log receivers in addition to the default [location][].
This, for example can be the export of a `loki.write` component to send log entries directly to Loki, or a `loki.relabel` component to add a certain label first.

## Log location

{{< param "PRODUCT_NAME" >}} writes all logs to `stderr`.

When you run {{< param "PRODUCT_NAME" >}} as a systemd service, you can view logs written to `stderr` through `journald`.

When you run {{< param "PRODUCT_NAME" >}} as a container, you can view logs written to `stderr` through `docker logs` or `kubectl logs`, depending on whether Docker or Kubernetes was used for deploying {{< param "PRODUCT_NAME" >}}.

When you run {{< param "PRODUCT_NAME" >}} as a Windows service, logs are written as event logs.
You can view the logs through Event Viewer.

In other cases, redirect `stderr` of the {{< param "PRODUCT_NAME" >}} process to a file for logs to persist on disk.

## Example

```alloy
logging {
  level  = "info"
  format = "logfmt"
}
```

[logfmt]: https://brandur.org/logfmt
[location]: #log-location

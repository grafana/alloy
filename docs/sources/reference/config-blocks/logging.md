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

| Name                | Type                 | Description                                | Default    | Required |
| ------------------- | -------------------- | ------------------------------------------ | ---------- | -------- |
| `format`            | `string`             | Format to use for writing log lines        | `"logfmt"` | no       |
| `level`             | `string`             | Level at which log lines should be written | `"info"`   | no       |
| `write_to`          | `list(LogsReceiver)` | List of receivers to send log entries to   | `[]`       | no       |
| `windows_event_log` | `bool`               | Write logs to Windows Event Log (Windows only) | `false`    | no       |

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

### Windows Event Log

The `windows_event_log` argument is only available on Windows and allows {{< param "PRODUCT_NAME" >}} to write logs directly to the Windows Event Log instead of `stderr`.

When `windows_event_log` is set to `true`:
- Log output is sent to the Windows Event Log
- Logs are not written to `stderr` or `stdout`
- The `write_to` argument continues to work, sending logs to specified receivers
- Log messages appear in Event Viewer under the "Alloy" source
- Log levels are mapped to appropriate Windows Event Log levels:
  - `debug` and `info` → Information
  - `warn` → Warning  
  - `error` → Error

## Log location

By default, {{< param "PRODUCT_NAME" >}} writes all logs to `stderr`.

When you run {{< param "PRODUCT_NAME" >}} as a systemd service, you can view logs written to `stderr` through `journald`.

When you run {{< param "PRODUCT_NAME" >}} as a container, you can view logs written to `stderr` through `docker logs` or `kubectl logs`, depending on whether Docker or Kubernetes was used for deploying {{< param "PRODUCT_NAME" >}}.

On Windows, you can configure {{< param "PRODUCT_NAME" >}} to write logs directly to the Windows Event Log by setting `windows_event_log = true` in the logging configuration. When enabled, logs are written exclusively to the Windows Event Log and not to `stderr`. You can view these logs through Event Viewer under the "Alloy" source.

In other cases, redirect `stderr` of the {{< param "PRODUCT_NAME" >}} process to a file for logs to persist on disk.

## Retrieve logs

You can retrieve the logs in different ways depending on your platform and installation method:

**Linux:**

* If you're running {{< param "PRODUCT_NAME" >}} with systemd, use `journalctl -u alloy`.
* If you're running {{< param "PRODUCT_NAME" >}} in a Docker container, use `docker logs CONTAINER_ID`.

**macOS:**

* If you're running {{< param "PRODUCT_NAME" >}} with Homebrew as a service, use `brew services info grafana/grafana/alloy` to check status and `tail -f $(brew --prefix)/var/log/alloy.log` for logs.
* If you're running {{< param "PRODUCT_NAME" >}} with launchd, use `log show --predicate 'process == "alloy"' --info` or check `/usr/local/var/log/alloy.log`.
* If you're running {{< param "PRODUCT_NAME" >}} in a Docker container, use `docker logs CONTAINER_ID`.

**Windows:**

* If you're running {{< param "PRODUCT_NAME" >}} as a Windows service, check the Windows Event Viewer under **Windows Logs** > **Application** for Alloy-related events.
* If you're running {{< param "PRODUCT_NAME" >}} that is manually installed, check the log files in `%PROGRAMDATA%\Grafana\Alloy\logs\` or the directory specified in your configuration.
* If you're running {{< param "PRODUCT_NAME" >}} in a Docker container, use `docker logs CONTAINER_ID`.

**All platforms:**

* {{< param "PRODUCT_NAME" >}} writes logs to `stderr` if started directly without a service manager.

## Examples

### Basic logging configuration

```alloy
logging {
  level  = "info"
  format = "logfmt"
}
```

### Windows Event Log configuration

```alloy
logging {
  level             = "info"
  format            = "logfmt"
  windows_event_log = true
}
```

This configuration writes logs to the Windows Event Log instead of `stderr`. The logs will appear in Event Viewer under the "Alloy" source.

### Windows Event Log with log forwarding

```alloy
logging {
  level             = "info"
  format            = "logfmt"
  windows_event_log = true
  write_to          = [loki.write.default.receiver]
}

loki.write "default" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
```

This configuration writes logs to both the Windows Event Log and forwards them to a Loki instance. Logs are not written to `stderr`.

[logfmt]: https://brandur.org/logfmt
[location]: #log-location

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

| Name          | Type                 | Description                                  | Default       | Required |
| ------------- | -------------------- | -------------------------------------------- | ------------- | -------- |
| `destination` | `string`             | Primary log destination.                     | __See below__ | no       |
| `format`      | `string`             | Format to use for writing log lines.         | `"logfmt"`    | no       |
| `level`       | `string`             | Level at which log lines should be written.  | `"info"`      | no       |
| `write_to`    | `list(LogsReceiver)` | List of receivers to send log entries to.    | `[]`          | no       |

### `level`

The following strings are recognized as valid log levels:

* `"error"`: Only write logs at the _error_ level.
* `"warn"`: Only write logs at the _warn_ level or above.
* `"info"`: Only write logs at _info_ level or above.
* `"debug"`: Write all logs, including _debug_ level logs.

### `format`

The following strings are recognized as valid log line formats:

* `"json"`: Write logs as JSON objects.
* `"logfmt"`: Write logs as [`logfmt`][logfmt] lines.

### `write_to`

The `write_to` argument allows {{< param "PRODUCT_NAME" >}} to tee its log entries to one or more `loki.*` component log receivers in addition to the default [location][].
This, for example can be the export of a `loki.write` component to send log entries directly to Loki, or a `loki.relabel` component to add a certain label first.

### `destination`

The following strings are recognized as valid log destinations:

* `"stderr"`: Write logs to `stderr`.
* `"windows_event_log"`:  Windows only. Write logs to the Windows Event Log under the "Alloy" source.

The default value of `destination` is set to `"windows_event_log"` when {{< param "PRODUCT_NAME" >}} runs as a Windows service.
Otherwise, `destination` defaults to `"stderr"`.

{{< param "PRODUCT_NAME" >}} fails to start if `destination` is set to `"windows_event_log"` and {{< param "PRODUCT_NAME" >}} is not running on Windows.

## Blocks

You can use the following blocks with `logging`:

| Block             | Description                                    | Required |
| ----------------- | ---------------------------------------------- | -------- |
| [`rate_limiting`][rate_limiting] | Configure per-message rate limiting and sampling. | no       |

### `rate_limiting`

The `rate_limiting` block enables per-message rate limiting and sampling of repeated log lines.

| Name | Type | Description | Default | Required |
|------|------|-------------|---------|----------|
| `enabled` | `bool` | Enable per-message rate limiting. | `true` | no |
| `tick` | `duration` | Sampling window. | `"1s"` | no |
| `threshold` | `number` | Identical lines admitted per (component, level, message) per tick before sampling. | `10` | no |
| `rate` | `number` | Fraction (0–1) of the over-threshold tail still admitted; `0` drops all excess. | `0` | no |
| `max_signatures` | `number` | Distinct signatures tracked; least-recently-used is evicted when full. | `1000` | no |

Rate limiting keys on the component, the log level, and the log message text (not attributes/fields).
Only identical repeated lines from the same component at the same level are throttled; distinct components/messages are independent (LRU-bounded by `max_signatures`).

Because keying is on the message text, log lines that share a constant message but differ only in attributes are treated as the same signature and throttled together.

Log lines with an empty message, such as some `go-kit`-style logs emitted without a `msg` or `message` field, bypass rate limiting entirely and are always written.

After suppression begins, the first admitted line of each new window carries a `slog_sampling.dropped_count` attribute.

Dropped lines are counted by the `alloy_logging_suppressed_lines_total` metric (labeled by `level` and `component_id`).

Set `enabled = false` to disable. Omitting the `rate_limiting` block leaves limiting enabled with defaults.

[rate_limiting]: #rate_limiting

## Retrieve logs

You can retrieve the logs in different ways depending on your platform and installation method:

**Linux:**

* If you're running {{< param "PRODUCT_NAME" >}} with systemd, use `journalctl -u alloy`.

**Docker:**

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
  Redirect `stderr` of the {{< param "PRODUCT_NAME" >}} process to a file for logs to persist on disk.

## Example

```alloy
logging {
  level  = "info"
  format = "logfmt"
}
```

[logfmt]: https://brandur.org/logfmt
[location]: #log-location

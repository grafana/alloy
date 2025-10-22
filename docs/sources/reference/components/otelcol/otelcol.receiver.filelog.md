---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.filelog/
description: Learn about otelcol.receiver.filelog
labels:
  stage: public-preview
  products:
    - oss
title: otelcol.receiver.filelog
---

# `otelcol.receiver.filelog`

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.filelog` reads log entries from files and forwards them to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.filelog` is a wrapper over the upstream OpenTelemetry Collector [`filelog`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`filelog`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/filelogreceiver
{{< /admonition >}}

You can specify multiple `otelcol.receiver.filelog` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.filelog "<LABEL>" {
  include = [...]
  output {
    logs    = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.filelog`:

| Name                            | Type                       | Description                                                                                | Default   | Required |
|---------------------------------|----------------------------|--------------------------------------------------------------------------------------------|-----------|----------|
| `include`                       | `list(string)`             | A list of glob patterns to include files.                                                  |           | yes      |
| `acquire_fs_lock`               | `bool`                     | Whether to acquire a file system lock while reading the file (Unix only).                  | `false`   | no       |
| `attributes`                    | `map(string)`              | A map of attributes to add to each log entry.                                              | `{}`      | no       |
| `compression`                   | `string`                   | The compression type used for the log file.                                                | ``        | no       |
| `delete_after_read`             | `bool`                     | Whether to delete the file after reading.                                                  | `false`   | no       |
| `encoding`                      | `string`                   | The encoding of the log file.                                                              | `"utf-8"` | no       |
| `exclude_older_than`            | `duration`                 | Exclude files with a modification time older than the specified duration.                  | `"0s"`    | no       |
| `exclude`                       | `list(string)`             | A list of glob patterns to exclude files that would be included by the `include` patterns. | `[]`      | no       |
| `fingerprint_size`              | `units.Base2Bytes`         | The size of the fingerprint used to detect file changes.                                   | `1KiB`    | no       |
| `force_flush_period`            | `duration`                 | The period after which logs are flushed even if the buffer isn't full.                     | `"500ms"` | no       |
| `include_file_name_resolved`    | `bool`                     | Whether to include the resolved filename in the log entry.                                 | `false`   | no       |
| `include_file_name`             | `bool`                     | Whether to include the filename in the log entry.                                          | `true`    | no       |
| `include_file_owner_group_name` | `bool`                     | Whether to include the file owner's group name in the log entry.                           | `false`   | no       |
| `include_file_owner_name`       | `bool`                     | Whether to include the file owner's name in the log entry.                                 | `false`   | no       |
| `include_file_path_resolved`    | `bool`                     | Whether to include the resolved file path in the log entry.                                | `false`   | no       |
| `include_file_path`             | `bool`                     | Whether to include the file path in the log entry.                                         | `false`   | no       |
| `include_file_record_number`    | `bool`                     | Whether to include the file record number in the log entry.                                | `false`   | no       |
| `max_batches`                   | `int`                      | The maximum number of batches to process concurrently.                                     | `10`      | no       |
| `max_concurrent_files`          | `int`                      | The maximum number of files to read concurrently.                                          | `10`      | no       |
| `max_log_size`                  | `units.Base2Bytes`         | The maximum size of a log entry.                                                           | `1MiB`    | no       |
| `operators`                     | `list(map(string))`        | A list of operators used to parse the log entries.                                         | `[]`      | no       |
| `poll_interval`                 | `duration`                 | The interval at which the file is polled for new entries.                                  | `"200ms"` | no       |
| `preserve_leading_whitespaces`  | `bool`                     | Preserves leading whitespace in messages when set to `true`.                               | `false`   | no       |
| `preserve_trailing_whitespaces` | `bool`                     | Preserves trailing whitespace in messages when set to `true`.                              | `false`   | no       |
| `resource`                      | `map(string)`              | A map of resource attributes to associate with each log entry.                             | `{}`      | no       |
| `start_at`                      | `string`                   | The position to start reading the file from.                                               | `"end"`   | no       |
| `storage`                       | `capsule(otelcol.Handler)` | Handler from an `otelcol.storage` component to use for persisting state.                   |           | no       |

`encoding` must be one of `utf-8`, `utf8-raw`, `utf-16le`, `utf-16be`, `ascii`, `big5`, or `nop`.
Refer to the upstream receiver [documentation][encoding-documentation] for more details.

`start_at` must be one of `beginning` or `end`. The `header` block may only be used if `start_at` is `beginning`.

`compression` must be either `""`, `gzip`, or `auto`. `auto` automatically detects file compression type and ingests data.
Currently, only gzip compressed files are auto detected. This allows for mix of compressed and uncompressed files to be ingested with the same filelogreceiver.

To persist state between restarts of the {{< param "PRODUCT_NAME" >}} process, set the `storage` attribute to the `handler` exported from an `otelcol.storage.*` component.

[encoding-documentation]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/receiver/filelogreceiver/README.md#supported-encodings

### `operators`

The `operators` list is a list of stanza [operators][] that transform the log entries after they have been read.

For example, if container logs are being collected you may want to utilize the stanza `container` parser operator to add relevant attributes to the log entries.

```alloy
otelcol.receiver.filelog "default" {
    ...
    operators = [
      {
        type = "container"
      }
    ]
}

```

[operators]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/stanza/docs/operators/README.md#what-operators-are-available

## Blocks

You can use the following blocks with `otelcol.receiver.filelog`:

| Block                                      | Description                                                                                     | Required |
|--------------------------------------------|-------------------------------------------------------------------------------------------------|----------|
| [`output`][output]                         | Configures where to send received telemetry data.                                               | yes      |
| [`debug_metrics`][debug_metrics]           | Configures the metrics that this component generates to monitor its state.                      | no       |
| [`header`][header]                         | Configures rules for parsing a log header line                                                  | no       |
| [`multiline`][multiline]                   | Configures rules for multiline parsing of log messages                                          | no       |
| [`ordering_criteria`][ordering_criteria]   | Configures the order in which log files are processed.                                          | no       |
| `ordering_criteria` > [`sort_by`][sort_by] | Configures the fields to sort by within the ordering criteria.                                  | yes      |
| [`retry_on_failure`][retry_on_failure]     | Configures the retry behavior when the receiver encounters an error downstream in the pipeline. | no       |

The > symbol indicates deeper levels of nesting.
For example, `ordering_criteria` > `sort_by` refers to a `sort_by` block defined inside a `ordering_criteria` block.

[output]: #output
[debug_metrics]: #debug_metrics
[header]: #header
[multiline]: #multiline
[ordering_criteria]: #ordering_criteria
[sort_by]: #sort_by
[retry_on_failure]: #retry_on_failure

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `header`

The `header` block configures logic for parsing a log header line into additional attributes added to each log entry.
It may only be used when `start_at` is set to `beginning`.
The following arguments are supported:

| Name                 | Type                | Description                                                 | Default | Required |
|----------------------|---------------------|-------------------------------------------------------------|---------|----------|
| `metadata_operators` | `lists(map(string)` | A list of operators used to parse metadata from the header. |         | yes      |
| `pattern`            | `string`            | A regular expression that matches the header line.          |         | yes      |

If a `header` block isn't set, no log lines will be treated as header metadata.

The `metadata_operators` list is a list of stanza [operators][] that parses metadata from the header.
Any attributes created from the embedded operators pipeline will be applied to all log entries in the file.

For example, you might use a `regex_parser` to process a header line that has been identified by the `pattern` expression.
The following example shows a fictitious header line, and then the `header` block that would parse an `environment` attribute from it.

```text
HEADER_IDENTIFIER env="production"
...
```

```alloy
otelcol.receiver.filelog "default" {
    ...
    header {
      pattern = '^HEADER_IDENTIFIER .*$'
      metadata_operators = [
        {
          type = "regex_parser"
          regex = 'env="(?P<environment>.+)"'
        }
      ]
    }
}
```

### `multiline`

The `multiline` block configures logic for splitting incoming log entries.
The following arguments are supported:

| Name                 | Type     | Description                                                     | Default | Required |
|----------------------|----------|-----------------------------------------------------------------|---------|----------|
| `line_end_pattern`   | `string` | A regular expression that matches the end of a log entry.       |         | yes*     |
| `line_start_pattern` | `string` | A regular expression that matches the beginning of a log entry. |         | yes*     |
| `omit_pattern`       | `bool`   | Omit the start/end pattern from the split log entries.          | `false` | no       |

A `multiline` block must contain either `line_start_pattern` or `line_end_pattern`.

If a `multiline` block isn't set, log entries won't be split.

### `ordering_criteria`

The `ordering_criteria` block configures the order in which log files discovered will be processed.
The following arguments are supported:

| Name       | Type     | Description                                                                            | Default | Required |
|------------|----------|----------------------------------------------------------------------------------------|---------|----------|
| `group_by` | `string` | A named capture group from the `regex` attribute used for grouping pre-sort.           | `""`    | no       |
| `regex`    | `string` | A regular expression to capture elements of log files to use in ordering calculations. | `""`    | no       |
| `top_n`    | `int`    | The number of top log files to track when using file ordering.                         | `1`     | no       |

### `sort_by`

The `sort_by` repeatable block configures the way the fields parsed in the `ordering_criteria` block will be applied to sort the discovered log files.
The following arguments are supported:

| Name        | Type     | Description                                                                  | Default | Required |
|-------------|----------|------------------------------------------------------------------------------|---------|----------|
| `sort_type` | `string` | The type of sorting to apply.                                                |         | yes      |
| `ascending` | `bool`   | Whether to sort in ascending order.                                          | `true`  | no       |
| `layout`    | `string` | The layout of the timestamp to be parsed from a named `regex` capture group. | `""`    | no       |
| `location`  | `string` | The location of the timestamp.                                               | `"UTC"` | no       |
| `regex_key` | `string` | The named capture group from the `regex` attribute to use for sorting.       | `""`    | no       |

`sort_type` must be one of `numeric`, `lexicographic`, `timestamp`, or `mtime`.
When using `numeric`, `lexicographic`, or `timestamp` `sort_type`, a named capture group defined in the `regex` attribute in `ordering_criteria` must be provided in `regex_key`.
When using `mtime` `sort_type`, the file's modified time will be used to sort.

The `location` and `layout` arguments are only applicable when `sort_type` is `timestamp`.

The `location` argument specifies a Time Zone identifier. The available locations depend on the local IANA Time Zone database.
Refer to the [list of tz database time zones][tz-wiki] in Wikipedia for a non-comprehensive list.

[tz-wiki]: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones

### `retry_on_failure`

The `retry_on_failure` block configures the retry behavior when the receiver encounters an error downstream in the pipeline.
A backoff algorithm is used to delay the retry upon subsequent failures.
The following arguments are supported:

| Name               | Type       | Description                                                                                                               | Default | Required |
|--------------------|------------|---------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `enabled`          | `bool`     | If set to `true` and an error occurs, the receiver will pause reading the log files and resend the current batch of logs. | `false` | no       |
| `initial_interval` | `duration` | The time to wait after first failure to retry.                                                                            | `"1s"`  | no       |
| `max_elapsed_time` | `duration` | The maximum age of a message before the data is discarded.                                                                | `"5m"`  | no       |
| `max_interval`     | `duration` | The maximum time to wait after applying backoff logic.                                                                    | `"30s"` | no       |

If `max_elapsed_time` is set to `0` data is never discarded.

## Exported fields

`otelcol.receiver.filelog` doesn't export any fields.

## Component health

`otelcol.receiver.filelog` is only reported as unhealthy if given an invalid configuration.

## Debug metrics

`otelcol.receiver.filelog` doesn't expose any component-specific debug metrics.

## Example

This example reads log entries using the `otelcol.receiver.filelog` receiver and they're logged by a `otelcol.exporter.debug` component.
It expects the logs to start with an ISO8601 compatible timestamp and parses it from the log using the `regex_parser` operator.

```alloy
otelcol.receiver.filelog "default" {
  include = ["/var/log/*.log"]
  operators = [{
    type = "regex_parser",
    regex = "^(?P<timestamp>\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}\\.\\d{3,6}Z)",
    timestamp = {
      parse_from = "attributes.timestamp",
      layout = "%Y-%m-%dT%H:%M:%S.%fZ",
      location = "UTC",
    },
  }]
  output {
      logs = [otelcol.exporter.debug.default.input]
  }
}

otelcol.exporter.debug "default" {}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.filelog` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

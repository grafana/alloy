---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.file/
aliases:
  - ../loki.source.file/ # /docs/alloy/latest/reference/components/loki.source.file/
description: Learn about loki.source.file
labels:
  stage: general-availability
  products:
    - oss
title: loki.source.file
---

# `loki.source.file`

`loki.source.file` reads log entries from files and forwards them to other `loki.*` components.
New log entries are forwarded whenever a log entry line ends with the `\n` character.

You can specify multiple `loki.source.file` components by giving them different labels.

{{< admonition type="note" >}}
By default, `loki.source.file` requires absolute file paths in the `targets` argument.
To discover files using glob patterns, you can either enable the [`file_match`](#file_match) block (recommended) for built-in file discovery, or use the `local.file_match` component for file discovery and pass its targets to `loki.source.file`.

Refer to the [File globbing](#file-globbing) example for more information.
{{< /admonition >}}

## Usage

```alloy
loki.source.file "<LABEL>" {
  targets    = <TARGET_LIST>
  forward_to = <RECEIVER_LIST>
}
```

## Arguments

The component starts a new reader for each of the given `targets` and fans out log entries to the list of receivers passed in `forward_to`.

You can use the following arguments with `loki.source.file`:

| Name                      | Type                 | Description                                                    | Default                    | Required |
|---------------------------|----------------------|----------------------------------------------------------------|----------------------------|----------|
| `forward_to`              | `list(LogsReceiver)` | List of receivers to send log entries to.                      |                            | yes      |
| `targets`                 | `list(map(string))`  | List of files to read from.                                    |                            | yes      |
| `encoding`                | `string`             | The encoding to convert from when reading files.               | `""`                       | no       |
| `legacy_positions_file`   | `string`             | Allows conversion from legacy positions file.                  | `""`                       | no       |
| `on_positions_file_error` | `string`             | How to handle a corrupt positions file entry for a given file. | `"restart_from_beginning"` | no       |
| `tail_from_end`           | `bool`               | Whether to tail from end if a stored position isn't found.     | `false`                    | no       |

The `encoding` argument must be a valid [IANA encoding][] name and if not set, it defaults to UTF-8. {{< param "PRODUCT_NAME" >}} can automatically change
the encoding to `UTF-16` if the file includes a Byte Order Mark (BOM) for either `UTF-16BE` or `UTF-16LE`.
The BOM will be taken into account even if {{< param "PRODUCT_NAME" >}} resumes tailing a file from the middle of the file. This can happen after {{< param "PRODUCT_NAME" >}} is restarted.

You can use the `tail_from_end` argument when you want to tail a large file without reading its entire content.
When set to true, only new logs are read, ignoring the existing ones.

The `on_positions_file_error` argument must be one of `"skip"`, `"restart_from_end"`, or `"restart_from_beginning"`.
This attribute defines the behavior if the positions file entry for a given file is corrupted.
`"restart_from_end"` will mimic the `tail_from_end` flag and set the position to the end of the file. This will reduce the likelihood of duplicate logs, but may cause some logs to not be sent.
`"restart_from_beginning"` will reset the position to `0`, causing the whole file to be read and processed. This may cause duplicate logs to be sent.
`"skip"` will cause the tailer to skip the file and not collect its logs.

`tail_from_end` and a `on_positions_file_error` value of `"restart_from_end"` are not supported when `decompression` is enabled.

The `legacy_positions_file` argument is used when you are transitioning from Grafana Agent Static Mode to Grafana Alloy. 
The format of the positions file is different in Grafana Alloy, so this will convert it to the new format.
This operation only occurs if the new positions file doesn't exist and the `legacy_positions_file` is valid.
When `legacy_positions_file` is set, Alloy will try to find previous positions for a given file by matching the path and labels, falling back to matching on path only if no match is found.


## Blocks

You can use the following blocks with `loki.source.file`:

| Name                             | Description                                                                  | Required |
| -------------------------------- | ---------------------------------------------------------------------------- | -------- |
| [`decompression`][decompression] | Configure reading logs from compressed files.                                | no       |
| [`file_match`][file_match]       | Configure file discovery using glob patterns for automatic target discovery. | no       |
| [`file_watch`][file_watch]       | Configure how often files should be polled from disk for changes.            | no       |

[decompression]: #decompression
[file_watch]: #file_watch
[file_match]: #file_match

### `decompression`

The `decompression` block contains configuration for reading logs from compressed files.
The following arguments are supported:

| Name            | Type       | Description                                                     | Default | Required |
| --------------- | ---------- | --------------------------------------------------------------- | ------- | -------- |
| `enabled`       | `bool`     | Whether decompression is enabled.                               |         | yes      |
| `format`        | `string`   | Compression format.                                             |         | yes      |
| `initial_delay` | `duration` | Time to wait before starting to read from new compressed files. | 0       | no       |

If you compress a file under a folder being scraped, `loki.source.file` might try to ingest your file before you finish compressing it.
To avoid it, pick an `initial_delay` that's long enough to avoid it.

Currently supported compression formats are:

* `gz` - for Gzip
* `z` - for zlib
* `bz2` - for bzip2

The component can only support one compression format at a time.
To handle multiple formats, you must create multiple components.

### `file_watch`

The `file_watch` block configures how often log files are polled from disk for changes.
The following arguments are supported:

| Name                 | Type       | Description                          | Default | Required |
| -------------------- | ---------- | ------------------------------------ | ------- | -------- |
| `max_poll_frequency` | `duration` | Maximum frequency to poll for files. | 250ms   | no       |
| `min_poll_frequency` | `duration` | Minimum frequency to poll for files. | 250ms   | no       |

If no file changes are detected, the poll frequency doubles until a file change is detected or the poll frequency reaches the `max_poll_frequency`.

If file changes are detected, the poll frequency is reset to `min_poll_frequency`.

### `file_match`

The `file_match` block enables built-in file discovery directly within `loki.source.file`.
When enabled, the component discovers files on the local filesystem using glob patterns and the [doublestar][] library.

The following arguments are supported:

| Name                | Type                | Description                                                 | Default | Required |
|---------------------|---------------------|-------------------------------------------------------------|---------|----------|
| `enabled`           | `bool`              | Enable file discovery using glob patterns.                  | `false` | no       |
| `ignore_older_than` | `duration`          | Ignores files with modification times before this duration. | `"0s"`  | no       |
| `sync_period`       | `duration`          | How often to sync filesystem and targets.                   | `"10s"` | no       |

Benefits of using `file_match` over `local.file_match`:

- Simplified configuration: No need for a separate `local.file_match` component
- Reduced overhead: File discovery is integrated directly into `loki.source.file`, eliminating the need for component-to-component communication
- Same functionality: Supports the same glob patterns and features, including `__path_exclude__` for excluding files

When `enabled` is set to `true`, you can use glob patterns, for example, `/tmp/*.log` or `/var/log/**/*.log`, directly in the `targets` argument's `__path__` label.
The component periodically scans the filesystem based on `sync_period` and automatically discovers new files, removes deleted files, and ignores files older than `ignore_older_than` if specified.

## Exported fields

`loki.source.file` doesn't export any fields.

## Component health

`loki.source.file` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.source.file` exposes some target-level debug information per reader:

* The tailed path.
* Whether the reader is running.
* The last recorded read offset in the positions file.

## Debug metrics

* `loki_source_file_encoding_failures_total` (counter): Number of encoding failures.
* `loki_source_file_file_bytes_total` (gauge): Number of bytes total.
* `loki_source_file_files_active_total` (gauge): Number of active files.
* `loki_source_file_read_bytes_total` (gauge): Number of bytes read.
* `loki_source_file_read_lines_total` (counter): Number of lines read.

## Component behavior

If the decompression feature is deactivated, the component continuously monitors and tails the files.
The component remains active after reaching the end of a file, and reads new entries in real-time as they're appended to the file.

Each element in the list of `targets` as a set of key-value pairs called _labels_.
The set of targets can either be _static_, or dynamically provided periodically by a service discovery component.
The special label `__path__` _must always_ be present and must contain the absolute path of the file to read from.

<!-- TODO(@tpaschalis) refer to local.file_match -->

The `__path__` value is available as the `filename` label to each log entry the component reads.
All other labels starting with a double underscore are considered _internal_ and are removed from the log entries before they're passed to other `loki.*` components.

The component uses its data path, a directory named after the domain's fully qualified name, to store its _positions file_.
The positions file stores read offsets, so that if a component or {{< param "PRODUCT_NAME" >}} restarts, `loki.source.file` can pick up tailing from the same spot.

The data path is inside the directory configured by the `--storage.path` [command line argument][cmd-args].

If a file is removed from the `targets` list, its positions file entry is also removed.
When it's added back on, `loki.source.file` starts reading it from the beginning.

[cmd-args]: ../../../cli/run/

## Examples

The following examples demonstrate how you can collect log entries with `loki.source.file`.

### Static targets

This example collects log entries from the files specified in the targets argument and forwards them to a `loki.write` component.

```alloy
loki.source.file "tmpfiles" {
  targets    = [
    {__path__ = "/tmp/foo.txt", "color" = "pink"},
    {__path__ = "/tmp/bar.txt", "color" = "blue"},
    {__path__ = "/tmp/baz.txt", "color" = "grey"},
  ]
  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "loki:3100/api/v1/push"
  }
}
```

### File globbing with `file_match` (recommended)

This example collects log entries from files matching the `*.log` pattern using the built-in `file_match` block.
When files appear or disappear, the list of targets is automatically updated based on the `sync_period` configuration.

```alloy
loki.source.file "tmpfiles" {
  targets    = [
    {__path__ = "/tmp/*.log"},
  ]
  forward_to = [loki.write.local.receiver]
  file_match {
    enabled = true
    sync_period = "10s"
  }
}

loki.write "local" {
  endpoint {
    url = "loki:3100/api/v1/push"
  }
}
```

### File globbing with `local.file_match`

This example demonstrates the alternative approach using a separate `local.file_match` component.
This approach is still supported but using the `file_match` block is recommended for better performance and simpler configuration.

```alloy
local.file_match "logs" {
  path_targets = [
    {__path__ = "/tmp/*.log"},
  ]
}

loki.source.file "tmpfiles" {
  targets    = local.file_match.logs.targets
  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "loki:3100/api/v1/push"
  }
}
```

### Decompression

This example collects log entries from compressed files matching the `*.gz` pattern using the built-in `file_match` block with decompression enabled.

```alloy
loki.source.file "tmpfiles" {
  targets    = [
    {__path__ = "/tmp/*.gz"},
  ]
  forward_to = [loki.write.local.receiver]
  file_match {
    enabled = true
    sync_period = "10s"
  }
  decompression {
    enabled       = true
    initial_delay = "10s"
    format        = "gz"
  }
}

loki.write "local" {
  endpoint {
    url = "loki:3100/api/v1/push"
  }
}
```

[IANA encoding]: https://www.iana.org/assignments/character-sets/character-sets.xhtml
[doublestar]: https://github.com/bmatcuk/doublestar

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.file` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.file/
aliases:
  - ../loki.source.file/ # /docs/alloy/latest/reference/components/loki.source.file/
description: Learn about loki.source.file
title: loki.source.file
---

# loki.source.file

`loki.source.file` reads log entries from files and forwards them to other `loki.*` components.

You can specify multiple `loki.source.file` components by giving them different labels.

{{< admonition type="note" >}}
`loki.source.file` doesn't handle file discovery. You can use `local.file_match` for file discovery.
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

`loki.source.file` supports the following arguments:

| Name                    | Type                 | Description                                                                 | Default | Required |
|-------------------------|----------------------|-----------------------------------------------------------------------------|---------|----------|
| `targets`               | `list(map(string))`  | List of files to read from.                                                 |         | yes      |
| `forward_to`            | `list(LogsReceiver)` | List of receivers to send log entries to.                                   |         | yes      |
| `encoding`              | `string`             | The encoding to convert from when reading files.                            | `""`    | no       |
| `tail_from_end`         | `bool`               | Whether a log file is tailed from the end if a stored position isn't found. | `false` | no       |
| `legacy_positions_file` | `string`             | Allows conversion from legacy positions file.                               | `""`    | no       |

The `encoding` argument must be a valid [IANA encoding][] name. If not set, it
defaults to UTF-8.

You can use the `tail_from_end` argument when you want to tail a large file without reading its entire content.
When set to true, only new logs will be read, ignoring the existing ones.


{{< admonition type="note" >}}
The `legacy_positions_file` argument is used when you are transitioning from legacy. The legacy positions file is rewritten into the new format.
This operation only occurs if the positions file doesn't exist and the `legacy_positions_file` is valid.
After the configuration is successfully converted, the `legacy_positions_file` is deleted.
If you add any labels before `loki.source.file`, then the positions file conversion won't work.
The legacy positions file didn't have a concept of labels in the positions file, so the conversion assumes no labels.
{{< /admonition >}}

## Blocks

The following blocks are supported inside the definition of `loki.source.file`:

| Hierarchy     | Name              | Description                                                       | Required |
|---------------|-------------------|-------------------------------------------------------------------|----------|
| decompression | [decompression][] | Configure reading logs from compressed files.                     | no       |
| file_watch    | [file_watch][]    | Configure how often files should be polled from disk for changes. | no       |

[decompression]: #decompression-block
[file_watch]: #file_watch-block

### decompression block

The `decompression` block contains configuration for reading logs from
compressed files. The following arguments are supported:

| Name            | Type       | Description                                                     | Default | Required |
| --------------- | ---------- | --------------------------------------------------------------- | ------- | -------- |
| `enabled`       | `bool`     | Whether decompression is enabled.                               |         | yes      |
| `initial_delay` | `duration` | Time to wait before starting to read from new compressed files. | 0       | no       |
| `format`        | `string`   | Compression format.                                             |         | yes      |

If you compress a file under a folder being scraped, `loki.source.file` might try to ingest your file before you finish compressing it.
To avoid it, pick an `initial_delay` that's long enough to avoid it.

Currently supported compression formats are:

- `gz` - for Gzip
- `z` - for zlib
- `bz2` - for bzip2

The component can only support one compression format at a time.
To handle multiple formats, you must create multiple components.

### file_watch block

The `file_watch` block configures how often log files are polled from disk for changes.
The following arguments are supported:

| Name                 | Type       | Description                          | Default | Required |
| -------------------- | ---------- | ------------------------------------ | ------- | -------- |
| `min_poll_frequency` | `duration` | Minimum frequency to poll for files. | 250ms   | no       |
| `max_poll_frequency` | `duration` | Maximum frequency to poll for files. | 250ms   | no       |

If no file changes are detected, the poll frequency doubles until a file change is detected or the poll frequency reaches the `max_poll_frequency`.

If file changes are detected, the poll frequency is reset to `min_poll_frequency`.

## Exported fields

`loki.source.file` doesn't export any fields.

## Component health

`loki.source.file` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.source.file` exposes some target-level debug information per reader:

- The tailed path.
- Whether the reader is running.
- The last recorded read offset in the positions file.

## Debug metrics

- `loki_source_file_read_bytes_total` (gauge): Number of bytes read.
- `loki_source_file_file_bytes_total` (gauge): Number of bytes total.
- `loki_source_file_read_lines_total` (counter): Number of lines read.
- `loki_source_file_encoding_failures_total` (counter): Number of encoding failures.
- `loki_source_file_files_active_total` (gauge): Number of active files.

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

### File globbing

This example collects log entries from the files matching `*.log` pattern using `local.file_match` component.
When files appear or disappear, the list of targets is updated accordingly.

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

This example collects log entries from the compressed files matching `*.gz` pattern using `local.file_match` component and the decompression configuration on the `loki.source.file` component.

```alloy

local.file_match "logs" {
  path_targets = [
    {__path__ = "/tmp/*.gz"},
  ]
}

loki.source.file "tmpfiles" {
  targets    = local.file_match.logs.targets
  forward_to = [loki.write.local.receiver]
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

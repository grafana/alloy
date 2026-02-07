---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.file/
description: Learn about otelcol.exporter.file
labels:
  stage: public-preview
  products:
    - oss
title: otelcol.exporter.file
---

# `otelcol.exporter.file`

`otelcol.exporter.file` accepts metrics, logs, and traces telemetry data from other `otelcol` components and writes it to files on disk.
You can write data in JSON or Protocol Buffers `proto` format.
You can optionally enable file rotation, compression, and separate output files based on a resource attribute.

{{< admonition type="note" >}}
`otelcol.exporter.file` is a wrapper over the upstream OpenTelemetry Collector Contrib [`fileexporter`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`fileexporter`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/fileexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.file` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.file "<LABEL>" {
  path = "<PATH>"
}
```

## Arguments

You can use the following arguments with `otelcol.exporter.file`:

| Name             | Type       | Description                                              | Default  | Required |
| ---------------- | ---------- | -------------------------------------------------------- | -------- | -------- |
| `path`           | `string`   | Path to the file to write telemetry data.                |          | yes      |
| `append`         | `bool`     | Append to the file when `true`; truncate when `false`.   | `false`  | no       |
| `compression`    | `string`   | Compression algorithm. Alloy supports only `"zstd"`.     |          | no       |
| `flush_interval` | `duration` | Time between flushes to disk. Must be greater than zero. | `"1s"`   | no       |
| `format`         | `string`   | Data format. Must be `"json"` or `"proto"`.              | `"json"` | no       |

You can't enable `append` and `compression` together.

{{< admonition type="note" >}}
The upstream `encoding` argument isn't supported.
Use `format` instead.
{{< /admonition >}}

## Blocks

You can use the following blocks with `otelcol.exporter.file`:

| Block                            | Description                                                          | Required |
| -------------------------------- | -------------------------------------------------------------------- | -------- |
| [`debug_metrics`][debug_metrics] | Configures internal metrics for this component.                      | no       |
| [`group_by`][group_by]           | Writes to separate files based on a resource attribute value.        | no       |
| [`rotation`][rotation]           | Configures file rotation. Ignored when `group_by.enabled` is `true`. | no       |

You can't enable `append` and `rotation` together.

[debug_metrics]: #debug_metrics
[group_by]: #group_by
[rotation]: #rotation

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `group_by`

The `group_by` block writes telemetry to separate files based on a resource attribute.

| Name                 | Type     | Description                                            | Default                       | Required |
| -------------------- | -------- | ------------------------------------------------------ | ----------------------------- | -------- |
| `enabled`            | `bool`   | Enable `group_by` behavior.                            | `false`                       | no       |
| `max_open_files`     | `int`    | Maximum number of simultaneously open files.           | `100`                         | no       |
| `resource_attribute` | `string` | Resource attribute whose value replaces `*` in `path`. | `"fileexporter.path_segment"` | no       |

When `group_by.enabled` is `true`:

- `path` must contain exactly one `*` character.
- The exporter replaces the `*` with the `resource_attribute` value.
- `rotation` settings don't apply.

### `rotation`

The `rotation` block configures rolling log file behavior.

| Name            | Type   | Description                                                           | Default | Required |
| --------------- | ------ | --------------------------------------------------------------------- | ------- | -------- |
| `localtime`     | `bool` | Use local time instead of UTC in rotated filenames.                   | `false` | no       |
| `max_backups`   | `int`  | Maximum number of rotated files to retain.                            | `100`   | no       |
| `max_days`      | `int`  | Maximum number of days to retain files. `0` keeps files indefinitely. | `0`     | no       |
| `max_megabytes` | `int`  | Maximum size in megabytes before the file rotates.                    | `100`   | no       |

## Exported fields

`otelcol.exporter.file` doesn't export any fields.

## Component health

`otelcol.exporter.file` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.file` doesn't expose any component-specific debug information.

## Examples

The following examples demonstrate how you can use `otelcol.exporter.file`.

### Basic file export

```alloy
otelcol.exporter.file "default" {
  path = "/tmp/traces.json"
}
```

### File export with rotation

```alloy
otelcol.exporter.file "rotated" {
  path   = "/var/log/telemetry.json"
  format = "json"

  rotation {
    max_megabytes = 50
    max_days      = 7
    max_backups   = 10
    localtime     = true
  }
}
```

### File export with compression

```alloy
otelcol.exporter.file "compressed" {
  path        = "/tmp/traces.jsonl"
  format      = "proto"
  compression = "zstd"
}
```

### Group by resource attribute

```alloy
otelcol.exporter.file "grouped" {
  path = "/tmp/logs/*/service.log"
  
  group_by {
    enabled           = true
    resource_attribute = "service.name"
    max_open_files     = 50
  }
}
```

### Complete pipeline

```alloy
otelcol.receiver.otlp "default" {
  grpc {
    endpoint = "0.0.0.0:4317"
  }

  http {
    endpoint = "0.0.0.0:4318"
  }

  output {
    metrics = [otelcol.exporter.file.default.input]
    logs    = [otelcol.exporter.file.default.input]
    traces  = [otelcol.exporter.file.default.input]
  }
}

otelcol.exporter.file "default" {
  path           = "/tmp/telemetry.json"
  format         = "json"
  flush_interval = "5s"

  rotation {
    max_megabytes = 100
    max_backups   = 5
  }
}
```

## Technical details

The `otelcol.exporter.file` component writes telemetry data to files on disk in either JSON or Protocol Buffers format.
It supports:

- **File rotation**: Automatically rotate files based on size, age, or number of backups
- **Compression**: Compress output files using `zstd` algorithm
- **Group by attributes**: Write to different files based on resource attribute values
- **Flushing**: Configurable flush intervals to control write frequency

### File formats

- **JSON format**: Each telemetry record is written as a separate line in JSON format (JSONL)
- **`proto` format**: Binary Protocol Buffers format with length-prefixed messages

### File naming with rotation

When rotation is enabled, rotated files are renamed with a timestamp:

- Original: `data.json`
- Rotated: `data-2023-09-20T10-30-00.123.json`

### Group by functionality

When `group_by` is enabled, files are created dynamically based on resource attribute values:

- Path template: `/logs/*/app.log`
- Resource attribute `service.name=frontend`
- Resulting file: `/logs/frontend/app.log`

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.file` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
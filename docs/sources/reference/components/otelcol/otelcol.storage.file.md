---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.storage.file/
description: Learn about otelcol.storage.file
title: otelcol.storage.file
labels:
  stage: public-preview
  products:
    - oss
---

# `otelcol.storage.file`

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.storage.file` exposes a `handler` that other `otelcol` components can use to write state to a local directory.
The current implementation of this component uses [`bbolt`][] to store and read data on disk.

{{< admonition type="note" >}}
`otelcol.storage.file` is a wrapper over the upstream OpenTelemetry Collector [`filestorage`][] extension.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`filestorage`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/storage/filestorage
{{< /admonition >}}

You can specify multiple `otelcol.storage.file` components by giving them different labels.

[`bbolt`]: https://github.com/etcd-io/bbolt

## Usage

```alloy
otelcol.storage.file "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `otelcol.storage.file`:

| Name                    | Type       | Description                                                                                 | Default  | Required |
| ----------------------- | ---------- | ------------------------------------------------------------------------------------------- | -------- | -------- |
| `create_directory`      | `bool`     | Will the component be responsible for creating the `directory`.                             | `true`   | no       |
| `directory`             | `string`   | The path to the dedicated data storage directory.                                           |          | no       |
| `directory_permissions` | `string`   | The octal file permissions used when creating the `directory` if `create_directory` is set. | `"0750"` | no       |
| `fsync`                 | `bool`     | Will `fsync` be called after each write operation.                                          | `false`  | no       |
| `timeout`               | `duration` | The timeout for file storage operations.                                                    | `"1s"`   | no       |

The default `directory` used for file storage is a subdirectory of the `data-alloy` directory located in the {{< param "PRODUCT_NAME" >}} working directory.
This will vary depending on the path specified by the [command line flag][run] `--storage-path`.

`create_directory` is `false` by default in the upstream receiver.
`create_directory` is `true` by default in {{< param "PRODUCT_NAME" >}} because `directory` defaults to the `data-alloy` path in {{< param "PRODUCT_NAME" >}} and that directory is owned by the {{< param "PRODUCT_NAME" >}} process.

[run]: ../../../cli/run/

## Blocks

You can use the following blocks with `otelcol.storage.file`:

| Block                            | Description                                                                | Required |
| -------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`compaction`][compaction]       | Configures file storage compaction.                                        | no       |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[compaction]: #compaction
[debug_metrics]: #debug_metrics

### `compaction`

The `compaction` block defines the compaction parameters for the file storage.

| Name                            | Type       | Description                                                                    | Default | Required |
| ------------------------------- | ---------- | ------------------------------------------------------------------------------ | ------- | -------- |
| `check_interval`                | `duration` | The interval to check if online compaction is required.                        | `"5s"`  | no       |
| `cleanup_on_start`              | `bool`     | Cleanup temporary files on component start.                                    | `false` | no       |
| `directory`                     | `string`   | The path to the directory where temporary compaction artifacts will be stored. |         | no       |
| `max_transaction_size`          | `int`      | Maximum number of items present in a single compaction iteration.              | `65536` | no       |
| `on_rebound`                    | `bool`     | Run compaction online when rebound conditions are met.                         | `false` | no       |
| `on_start`                      | `bool`     | Run compaction on component start.                                             | `false` | no       |
| `rebound_needed_threshold_mib`  | `int`      | File storage total allocated size boundary to mark need for online compaction. | `100`   | no       |
| `rebound_trigger_threshold_mib` | `int`      | File storage used allocated size boundary to trigger online compaction.        | `10`    | no       |

The default `directory` used for file storage is a subdirectory of the `data-alloy` directory located in the {{< param "PRODUCT_NAME" >}} working directory.
This will vary depending on the path specified by the [command line flag][run] `--storage-path`.

If `on_rebound` online compaction is enabled, compaction will be triggered when total allocated data is greater than `rebound_needed_threshold_mib` and used allocated data is less than `rebound_trigger_threshold_mib`.
More detailed information about the way the component supports file compaction for allocated disk storage recovery can be found in the upstream component's [documentation][compaction_docs].

[compaction_docs]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/storage/filestorage#compaction

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                       | Description                                                             |
| --------- | -------------------------- | ----------------------------------------------------------------------- |
| `handler` | `capsule(otelcol.Handler)` | A value that other components can use to persist state to file storage. |

## Component health

`otelcol.storage.file` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.storage.file` doesn't expose any component-specific debug information.

## Examples

### `otelcol.receiver.filelog`

This examples uses an `otelcol.storage.file` component to store file offsets for an `otelcol.receiver.filelog` component.
This will only use a small amount of data for each file that has been read so it's unlikely that you will need to be concerned about compaction settings.

The default settings of the component will place the [`bbolt`] file for the receiver in `<STORAGE_PATH>/otelcol.storage.file.default/receiver_filelog_default`

```alloy

otelcol.storage.file "default" {}

otelcol.receiver.filelog "default" {
    include = ["/var/log/*.log"]
    storage = otelcol.storage.file.default.handler
    operators = [{
      type = "regex_parser",
      regex = "^(?P<timestamp>[^ ]+)",
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

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.file_stats/
aliases:
  - ../otelcol.receiver.file_stats/ # /docs/alloy/latest/reference/otelcol.receiver.file_stats/
title: otelcol.receiver.file_stats
description: Learn about otelcol.receiver.file_stats
---

# otelcol.receiver.file_stats

`otelcol.receiver.file_stats` collects metrics from files and folders specified with a glob pattern.

{{< admonition type="note" >}}
`otelcol.receiver.file_stats` is a wrapper over the upstream OpenTelemetry Collector `filestats` receiver from the `otelcol-contrib` distribution.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

Multiple `otelcol.receiver.file_stats` components can be specified by giving them different labels.

{{< admonition type="warning" >}}
`otelcol.receiver.file_stats` only works on macOS, Linux, and Windows.
{{< /admonition >}}

## Usage

```alloy
otelcol.receiver.file_stats "LABEL" {
  include = "GLOB_PATTERN"

  output {
    metrics = [...]
  }
}
```

## Arguments

`otelcol.receiver.file_stats` supports the following arguments:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`include` | `string` | Glob path for paths to collect stats from. | | yes
`collection_interval` | `duration` | How often to collect statistics. | `"1m"` | no
`initial_delay` | `duration` | Initial time to wait before collecting statistics. | `"1s"` | no
`timeout` | `duration` | Timeout for a collection; `0s` means no timeout. | `"0s"` | no

`include` is a glob pattern that specifies which paths (files and folders) to collect stats from.
A `*` character matches entries in a directory, while `**` includes subdirectories.
For example, `/var/log/**/*.log` matches all files and directories ending in `.log` recursively inside `/var/log`.

The `timeout` argument controls the timeout for each collection specified by the `collection_interval`.
The timeout applies to the entire collection process across all paths matched by the `include` argument.

## Blocks

The following blocks are supported inside the definition of `otelcol.receiver.file_stats`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
metrics | [metrics][] | Configures which metrics will be sent to downstream components. | no
metrics > file.atime | [file.atime][] | Configures the `file.atime` metric. | no
metrics > file.count | [file.count][] | Configures the `file.count` metric. | no
metrics > file.ctime | [file.ctime][] | Configures the `file.ctime` metric. | no
metrics > file.mtime | [file.mtime][] | Configures the `file.mtime` metric. | no
metrics > file.size | [file.size][] | Configures the `file.size` metric. | no
resource_attributes | [resource_attributes][] | Configures resource attributes for metrics sent to downstream components. | no
resource_attributes > file.name | [file.name][] | Configures the `file.name` resource attribute. | no
resource_attributes > file.name > metrics_include | [metrics_include][] | Metrics to include the `file.name` resource attribute in. | no
resource_attributes > file.name > metrics_exclude | [metrics_exclude][] | Metrics to exclude the `file.name` resource attribute from. | no
resource_attributes > file.path | [file.path][] | Configures the `file.path` resource attribute. | no
resource_attributes > file.path > metrics_include | [metrics_include][] | Metrics to include the `file.path` resource attribute in. | no
resource_attributes > file.path > metrics_exclude | [metrics_exclude][] | Metrics to exclude the `file.path` resource attribute from. | no
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no
output | [output][] | Configures where to send received telemetry data. | yes

[metrics]: #metrics-block
[file.atime]: #fileatime-block
[file.count]: #filecount-block
[file.ctime]: #filectime-block
[file.mtime]: #filemtime-block
[file.size]: #filesize-block
[resource_attributes]: #resource_attributes-block
[file.name]: #filename-block
[metrics_include]: #metrics_include-block
[metrics_exclude]: #metrics_exclude-block
[file.path]: #filepath-block
[debug_metrics]: #debug_metrics-block
[output]: #output-block

### metrics block

The `metrics` block configures the set of metrics that will be sent to downstream components.
It accepts no arguments, but contains other blocks for individual metrics:

* The [file.atime][] block
* The [file.count][] block
* The [file.ctime][] block
* The [file.mtime][] block
* The [file.size][] block

Refer to the documentation of individual metric blocks for whether that metric is enabled by default.

[file.atime]: #fileatime-block
[file.count]: #filecount-block
[file.ctime]: #filectime-block
[file.mtime]: #filemtime-block
[file.size]: #filesize-block

### file.atime block

The `file.atime` block configures the `file.atime` metric.
`file.atime` tracks the elapsed time since the last access of the file or folder in Unix seconds since the epoch.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`enabled` | `boolean` | Whether to collect the `file.atime` metric. | `false` | no

### file.count block

The `file.count` block configures the `file.count` metric.
`file.count` tracks the number of files and folders in the specified glob pattern.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`enabled` | `boolean` | Whether to collect the `file.count` metric. | `false` | no

### file.ctime block

The `file.ctime` block configures the `file.ctime` metric.
`file.ctime` tracks the elapsed time since the last change of the file or folder in Unix seconds since the epoch.
Changes include permissions, ownership, and timestamps.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`enabled` | `boolean` | Whether to collect the `file.ctime` metric. | `false` | no

### file.mtime block

The `file.mtime` block configures the `file.mtime` metric.
`file.mtime` tracks the elapsed time since the last modification of the file or folder in Unix seconds since the epoch.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`enabled` | `boolean` | Whether to collect the `file.mtime` metric. | `true` | no

### file.size block

The `file.size` block configures the `file.size` metric.
`file.size` tracks the size of the file or folder in bytes.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`enabled` | `boolean` | Whether to collect the `file.size` metric. | `true` | no

### resource_attributes block

The `resource_attributes` block configures resource attributes for metrics sent to downstream components.
It accepts no arguments, but contains other blocks for configuring individual resource attributes:

* The [file.name][] block
* The [file.path][] block

Refer to the documentation of individual resource attribute blocks for whether that resource attribute is enabled by default.

[file.name]: #filename-block
[file.path]: #filepath-block

### file.name block

The `file.name` block configures the `file.name` resource attribute.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`enabled` | `boolean` | Whether to include the `file.name` resource attribute. | `true` | no

When `enabled` is true, the `file.name` attribute is included in all metrics.

The children blocks `metrics_include` and `metrics_exclude` can be used to further filter which metrics are given the `file.name` attribute.
If a given metric matches all the `metrics_include` blocks and none of the `metrics_exclude` blocks, the `file.name` attribute is added.

### metrics_include block

The `metrics_include` block configures a filter for matching metrics.
The `metrics_include` block may be specified multiple times.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`strict` | `string` | The exact name of the metric to include. | | yes*
`regexp` | `string` | A regular expression for the metrics to include. | | yes*

Exactly one of `strict` or `regexp` must be specified.

### metrics_exclude block

The `metrics_exclude` block configures a filter for excluding metrics.
The `metrics_exclude` block may be specified multiple times.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`strict` | `string` | The exact name of the metric to exclude. | | yes*
`regexp` | `string` | A regular expression for the metrics to exclude. | | yes*

Exactly one of `strict` or `regexp` must be specified.

### file.path block

The `file.path` block configures the `file.path` resource attribute.

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`enabled` | `boolean` | Whether to include the `file.path` resource attribute. | `false` | no

When `enabled` is true, the `file.path` attribute is included in all metrics.
The children blocks `metrics_include` and `metrics_exclude` can be used to further filter which metrics are given the `file.path` attribute.
If a given metric matches all the `metrics_include` blocks and none of the `metrics_exclude` blocks, the `file.path` attribute is added.

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.file_stats` does not export any fields.

## Component health

`otelcol.receiver.file_stats` is reported as unhealthy when:

* It is given an invalid configuration.
* It runs on an unsupported operating system.

## Debug information

`otelcol.receiver.file_stats` does not expose any component-specific debug information.

## Example

This example forwards file stats of files and folders with the `.log` extension in `/var/log` through a batch processor before finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.file_stats "default" {
  include = "/var/log/**/*.log"

  output {
    metrics = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = env("OTLP_ENDPOINT")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.file_stats` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.process/
aliases:
  - ../prometheus.exporter.process/ # /docs/alloy/latest/reference/components/prometheus.exporter.process/
description: Learn about prometheus.exporter.process
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.process
---

# `prometheus.exporter.process`

The `prometheus.exporter.process` component embeds the [`process_exporter`](https://github.com/ncabatoff/process-exporter) for collecting process stats from `/proc`.

{{< docs/shared lookup="reference/components/exporter-clustering-warning.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Usage

```alloy
prometheus.exporter.process "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.process`:

| Name                  | Type     | Description                                       | Default   | Required |
| --------------------- | -------- | ------------------------------------------------- | --------- | -------- |
| `gather_smaps`        | `bool`   | Gather metrics from the smaps file for a process. | `true`    | no       |
| `procfs_path`         | `string` | The procfs mount point.                           | `"/proc"` | no       |
| `recheck_on_scrape`   | `bool`   | Recheck process names on each scrape.             | `false`   | no       |
| `remove_empty_groups` | `bool`   | Forget process groups with no processes.          | `false`   | no       |
| `track_children`      | `bool`   | Whether to track a process' children.             | `true`    | no       |
| `track_threads`       | `bool`   | Report metrics for a process' individual threads. | `true`    | no       |

If you set `remove_empty_groups` to the default, `false`, the process "groups" created by the `matcher` blocks continue to report metrics even after the processes in that group have stopped running.
This ensures you can see when a process count drops to zero, but it can cause unbounded growth in reported metrics and memory usage if your `matcher` generates dynamic group names, for example, using specific PIDs.
The reporting continues until {{< param "PRODUCT_NAME" >}} is restarted.

When you set `remove_empty_groups` to `true`, process groups are forgotten and stop reporting metrics as soon as they contain no running processes.
Grafana recommends that you set `remove_empty_groups` to `true` if your name argument utilizes unique identifiers like `.PID` or `.StartTime`.

For example, when you set `remove_empty_groups` to `false` and the `name` argument for a `matcher` block utilizes the `.PID` of a process, the `matcher` creates a new process group for every new process instance.
The old process groups continue to report metrics with values of 0, even though no running processes are associated with them, leading to high cardinality.
Set `remove_empty_groups` to `true` to remove the old groups, and prevent the high cardinality.

## Blocks

You can use the following block with `prometheus.exporter.process`:

| Name        | Description                                                                    | Required |
| ----------- | ------------------------------------------------------------------------------ | -------- |
| [matcher][] | A collection of matching rules to use for deciding which processes to monitor. | no       |

[matcher]: #matcher

### `matcher`

Each `matcher` block configuration can match multiple processes, which are tracked as a single process "group."

| Name      | Type           | Description                                                                                      | Default          | Required |
| --------- | -------------- | ------------------------------------------------------------------------------------------------ | ---------------- | -------- |
| `cmdline` | `list(string)` | A list of regular expressions applied to the `argv` of the process.                              |                  | no       |
| `comm`    | `list(string)` | A list of strings that match the base executable name for a process, truncated to 15 characters. |                  | no       |
| `exe`     | `list(string)` | A list of strings that match `argv[0]` for a process.                                            |                  | no       |
| `name`    | `string`       | The name to use for identifying the process group name in the metric.                            | `"{{.ExeBase}}"` | no       |

The `name` argument can use the following template variables. By default it uses the base path of the executable:

* `{{.Comm}}`: Basename of the original executable from /proc/\<pid\>/stat.
* `{{.ExeBase}}`: Basename of the executable from argv[0].
* `{{.ExeFull}}`: Fully qualified path of the executable.
* `{{.Username}}`: Username of the effective user.
* `{{.Matches}}`: Map containing all regular explression capture groups resulting from matching a process with the cmdline rule group.
* `{{.PID}}`: PID of the process. Note that the PID is copied from the first executable found.
* `{{.StartTime}}`: The start time of the process. This is useful when combined with PID as PIDS get reused over time.
* `{{.Cgroups}}`: The cgroups, if supported, of the process (`/proc/self/cgroup`). This is particularly useful for identifying to which container a process belongs.

{{< admonition type="note" >}}
Using `PID` or `StartTime` is discouraged, as it's almost never what you want, and is likely to result in high cardinality metrics.
{{< /admonition >}}

The value that's used for matching `comm` list elements is derived from reading the second field of `/proc/<pid>/stat`, stripped of parens.

For values in `exe`, if there are no slashes, only the basename of `argv[0]` needs to match.
Otherwise, the name must be an exact match.
For example, `"postgres"` may match any PostgreSQL binary, but `/usr/local/bin/postgres` only matches a PostgreSQL process with that exact path.
If any of the strings match, the process is tracked.

Each regular expression in `cmdline` must match the corresponding `argv` for the process to be tracked.
The first element that is matched is `argv[1]`.
Regular expression captures are added to the `.Matches` map for use in the name.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.process` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.process` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.process` doesn't expose any component-specific debug metrics.

## Example

This example uses a [`prometheus.scrape`][scrape] component to collect metrics from `prometheus.exporter.process`:

```alloy
prometheus.exporter.process "example" {
  track_children = false

  matcher {
    comm = ["alloy"]
  }
}

// Configure a prometheus.scrape component to collect process_exporter metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.process.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.process` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

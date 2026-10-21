---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.process/
aliases:
  - ../prometheus.exporter.process/ # /docs/alloy/latest/reference/components/prometheus.exporter.process/
description: Learn about prometheus.exporter.process
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-10-16
title: prometheus.exporter.process
---

# `prometheus.exporter.process`

The `prometheus.exporter.process` component embeds the [`process_exporter`][process-exporter] to collect process stats from `/proc`.

{{< docs/shared lookup="reference/components/exporter-clustering-warning.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< admonition type="note" >}}
The `prometheus.exporter.process` component only works on Linux systems.
On other operating systems, {{< param "PRODUCT_NAME" >}} writes a warning to the logs and the component doesn't collect process metrics.
{{< /admonition >}}

You can specify multiple `prometheus.exporter.process` components by giving them different labels.

[process-exporter]: https://github.com/ncabatoff/process-exporter

## Usage

```alloy
prometheus.exporter.process "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.process`:

| Name                  | Type     | Description                                                                                         | Default   | Required |
| --------------------- | -------- | --------------------------------------------------------------------------------------------------- | --------- | -------- |
| `gather_smaps`        | `bool`   | Gather proportional memory metrics for a process from the smaps files.                              | `true`    | no       |
| `procfs_path`         | `string` | The procfs mount point.                                                                             | `"/proc"` | no       |
| `recheck_on_scrape`   | `bool`   | Recheck process names on each scrape.                                                               | `false`   | no       |
| `remove_empty_groups` | `bool`   | Remove process groups that contain no processes.                                                    | `false`   | no       |
| `track_children`      | `bool`   | Track children of a tracked process in the same group, unless a child belongs to a different group. | `true`    | no       |
| `track_threads`       | `bool`   | Report metrics for each thread name in addition to each group name.                                 | `true`    | no       |

If you set `remove_empty_groups` to the default, `false`, the process "groups" that the `matcher` blocks create continue to report metrics even after the processes in that group have stopped running.
This ensures you can see when a process count drops to zero, but it can cause unbounded growth in reported metrics and memory usage if your `matcher` generates dynamic group names based on specific PIDs.
The reporting continues until you restart {{< param "PRODUCT_NAME" >}}.

When you set `remove_empty_groups` to `true`, the component drops a process group and stops reporting metrics for it when the group contains no running processes.
Grafana recommends that you set `remove_empty_groups` to `true` if your `name` argument uses unique identifiers like `.PID` or `.StartTime`.

For example, if you set `remove_empty_groups` to `false` and the `name` argument for a `matcher` block uses the `.PID` of a process, the `matcher` creates a separate process group for every process instance.
Groups whose processes have exited continue to report metrics with values of 0, which leads to high cardinality.
Set `remove_empty_groups` to `true` so the component removes those groups and prevents the high cardinality.

The component sets the `instance` label on its exported targets to the `HOSTNAME` environment variable.
If `HOSTNAME` isn't set, the component uses the operating system hostname, or `unknown` if it can't determine one.

## Blocks

You can use the following block with `prometheus.exporter.process`:

{{< docs/alloy-config >}}

| Block                | Description                                 | Required |
| -------------------- | ------------------------------------------- | -------- |
| [`matcher`][matcher] | Configures a group of processes to monitor. | no       |

[matcher]: #matcher

{{< /docs/alloy-config >}}

### `matcher`

Each `matcher` block can match multiple processes, which the component tracks as a single process "group."

You can use the following arguments with the `matcher` block:

| Name      | Type           | Description                                                                                      | Default          | Required |
| --------- | -------------- | ------------------------------------------------------------------------------------------------ | ---------------- | -------- |
| `cmdline` | `list(string)` | A list of regular expressions, each matched against the corresponding element of `argv`.         |                  | no       |
| `comm`    | `list(string)` | A list of strings that match the base executable name for a process, truncated to 15 characters. |                  | no       |
| `exe`     | `list(string)` | A list of strings that match `argv[0]` for a process.                                            |                  | no       |
| `name`    | `string`       | The name to use for the process group in metrics.                                                | `"{{.ExeBase}}"` | no       |

Each `matcher` block must set at least one of `cmdline`, `comm`, or `exe`.
If you set none of them, the component fails with an invalid configuration error.

The component derives the value for matching `comm` list elements from the second field of `/proc/<pid>/stat`, with the parentheses removed.

For values in `exe`, if there are no slashes, only the basename of `argv[0]` needs to match.
Otherwise, the name must be an exact match.
For example, `"postgres"` may match any PostgreSQL binary, but `/usr/local/bin/postgres` only matches a PostgreSQL process with that exact path.
If any of the strings match, the component tracks the process.

The component matches each regular expression in `cmdline` against the corresponding element of `argv`, starting at `argv[1]`.
The first regular expression matches `argv[1]`, the second matches `argv[2]`, and so on.
Every regular expression must match its corresponding element before the component tracks the process.
The component adds named regular expression captures, for example `(?P<version>[0-9]+)`, to the `.Matches` map for use in `name`.

The `name` argument can use the following template variables. By default it uses the base path of the executable:

- `{{.Comm}}`: Basename of the original executable from `/proc/<pid>/stat`.
- `{{.ExeBase}}`: Basename of the executable from `argv[0]`.
- `{{.ExeFull}}`: Fully qualified path of the executable.
- `{{.Username}}`: Username of the effective user.
- `{{.Matches}}`: A map of the named capture groups from the `cmdline` regular expressions.
- `{{.PID}}`: PID of the process. The component copies the PID from the first executable found.
- `{{.StartTime}}`: The start time of the process. This is useful with `PID`, because PIDs get reused over time.
- `{{.Cgroups}}`: The cgroups of the process, from `/proc/self/cgroup`, if supported. This is particularly useful for identifying which container a process belongs to.

{{< admonition type="note" >}}
Avoid `PID` and `StartTime`.
They're almost never what you want, and they're likely to result in high cardinality metrics.
{{< /admonition >}}

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

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

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

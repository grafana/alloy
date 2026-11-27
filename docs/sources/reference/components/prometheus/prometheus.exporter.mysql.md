---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.mysql/
aliases:
  - ../prometheus.exporter.mysql/ # /docs/alloy/latest/reference/components/prometheus.exporter.mysql/
description: Learn about prometheus.exporter.mysql
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-11-27
title: prometheus.exporter.mysql
---

# `prometheus.exporter.mysql`

The `prometheus.exporter.mysql` component embeds the [`mysqld_exporter`][mysqld_exporter] to collect metrics from a MySQL server.

You can specify multiple `prometheus.exporter.mysql` components by giving them different labels.

[mysqld_exporter]: https://github.com/prometheus/mysqld_exporter

## Usage

```alloy
prometheus.exporter.mysql "<LABEL>" {
  data_source_name = "<DATA_SOURCE_NAME>"
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.mysql`:

| Name                       | Type           | Description                                                                                                | Default | Required |
| -------------------------- | -------------- | ---------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `data_source_name`         | `secret`       | [Data Source Name][dsn] for the MySQL server to connect to.                                                |         | no       |
| `disable_collectors`       | `list(string)` | A list of [collectors][] to disable from the default set.                                                  |         | no       |
| `enable_collectors`        | `list(string)` | A list of [collectors][] to enable in addition to the default set.                                         |         | no       |
| `enable_lock_wait_timeout` | `bool`         | Enables the `lock_wait_timeout` MySQL connection parameter.                                                | `true`  | no       |
| `lock_wait_timeout`        | `int`          | Timeout, in seconds, to acquire a metadata lock. Has no effect when `enable_lock_wait_timeout` is `false`. | `2`     | no       |
| `log_slow_filter`          | `bool`         | Adds a `log_slow_filter` session setting to avoid slow query logging of scrapes.                           | `false` | no       |
| `set_collectors`           | `list(string)` | A list of [collectors][] to run. Fully overrides the default set.                                          |         | no       |

If you don't set `data_source_name`, the component reads the data source name from the `MYSQLD_EXPORTER_DATA_SOURCE_NAME` environment variable.
If you set neither, the component fails with an error.

The component applies `set_collectors` first, then `disable_collectors`, then `enable_collectors`.
If you list a collector in both `disable_collectors` and `enable_collectors`, the component enables it.

Set a `lock_wait_timeout` on the connection to avoid potentially long wait times for metadata locks.
Refer to [`lock_wait_timeout`][lock-wait-timeout] in the MySQL documentation for more information.

{{< admonition type="note" >}}
`log_slow_filter` isn't supported by Oracle MySQL.
{{< /admonition >}}

The component sets the `instance` label on its exported targets to the network type, address, and database name from `data_source_name`, in the form `<NETWORK>(<ADDRESS>)/<DATABASE>`, for example, `tcp(db.example.com:3306)/shop`.
When the component reads the data source name from the environment variable, it sets the `instance` label to `tcp(127.0.0.1:3306)/`, regardless of the server address.

[collectors]: #supported-collectors
[dsn]: https://github.com/go-sql-driver/mysql#dsn-data-source-name
[lock-wait-timeout]: https://dev.mysql.com/doc/refman/8.0/en/server-system-variables.html#sysvar_lock_wait_timeout

### Supported collectors

The full list of supported collectors is:

| Name                                               | Description                                                                                  | Enabled by default |
| -------------------------------------------------- | -------------------------------------------------------------------------------------------- | ------------------ |
| `global_status`                                    | Collect metrics from `SHOW GLOBAL STATUS`.                                                   | yes                |
| `global_variables`                                 | Collect metrics from `SHOW GLOBAL VARIABLES`.                                                | yes                |
| `info_schema.innodb_cmp`                           | Collect metrics from `information_schema.innodb_cmp`.                                        | yes                |
| `info_schema.innodb_cmpmem`                        | Collect metrics from `information_schema.innodb_cmpmem`.                                     | yes                |
| `info_schema.query_response_time`                  | Collect query response time distribution if `query_response_time_stats` is ON.               | yes                |
| `slave_status`                                     | Scrape information from `SHOW SLAVE STATUS`.                                                 | yes                |
| `auto_increment.columns`                           | Collect `auto_increment` columns and max values from `information_schema`.                   | no                 |
| `binlog_size`                                      | Collect the current size of all registered `binlog` files.                                   | no                 |
| `engine_innodb_status`                             | Collect metrics from `SHOW ENGINE INNODB STATUS`.                                            | no                 |
| `engine_tokudb_status`                             | Collect metrics from `SHOW ENGINE TOKUDB STATUS`.                                            | no                 |
| `heartbeat`                                        | Collect metrics from heartbeat database and tables.                                          | no                 |
| `info_schema.clientstats`                          | If running with userstat=1, enable to collect client statistics.                             | no                 |
| `info_schema.innodb_metrics`                       | Collect metrics from `information_schema.innodb_metrics`.                                    | no                 |
| `info_schema.innodb_tablespaces`                   | Collect metrics from `information_schema.innodb_sys_tablespaces`.                            | no                 |
| `info_schema.processlist`                          | Collect current thread state counts from the `information_schema.processlist`.               | no                 |
| `info_schema.replica_host`                         | Collect metrics from `information_schema.replica_host_status`.                               | no                 |
| `info_schema.schemastats`                          | If running with userstat=1, enable to collect schema statistics.                             | no                 |
| `info_schema.tables`                               | Collect metrics from `information_schema.tables`.                                            | no                 |
| `info_schema.tablestats`                           | If running with userstat=1, enable to collect table statistics.                              | no                 |
| `info_schema.userstats`                            | If running with userstat=1, enable to collect user statistics.                               | no                 |
| `mysql.user`                                       | Collect data from `mysql.user`.                                                              | no                 |
| `perf_schema.eventsstatements`                     | Collect metrics from `performance_schema.events_statements_summary_by_digest`.               | no                 |
| `perf_schema.eventsstatementssum`                  | Collect metrics of grand sums from `performance_schema.events_statements_summary_by_digest`. | no                 |
| `perf_schema.eventswaits`                          | Collect metrics from `performance_schema.events_waits_summary_global_by_event_name`.         | no                 |
| `perf_schema.file_events`                          | Collect metrics from `performance_schema.file_summary_by_event_name`.                        | no                 |
| `perf_schema.file_instances`                       | Collect metrics from `performance_schema.file_summary_by_instance`.                          | no                 |
| `perf_schema.indexiowaits`                         | Collect metrics from `performance_schema.table_io_waits_summary_by_index_usage`.             | no                 |
| `perf_schema.memory_events`                        | Collect metrics from `performance_schema.memory_summary_global_by_event_name`.               | no                 |
| `perf_schema.replication_applier_status_by_worker` | Collect metrics from `performance_schema.replication_applier_status_by_worker`.              | no                 |
| `perf_schema.replication_group_member_stats`       | Collect metrics from `performance_schema.replication_group_member_stats`.                    | no                 |
| `perf_schema.replication_group_members`            | Collect metrics from `performance_schema.replication_group_members`.                         | no                 |
| `perf_schema.tableiowaits`                         | Collect metrics from `performance_schema.table_io_waits_summary_by_table`.                   | no                 |
| `perf_schema.tablelocks`                           | Collect metrics from `performance_schema.table_lock_waits_summary_by_table`.                 | no                 |
| `slave_hosts`                                      | Scrape information from `SHOW SLAVE HOSTS`.                                                  | no                 |
| `sys.user_summary`                                 | Collect per-user metrics from `sys.x$user_summary`.                                         | no                 |

## Blocks

You can use the following blocks with `prometheus.exporter.mysql`:

{{< docs/alloy-config >}}

| Block                                                          | Description                                              | Required |
| -------------------------------------------------------------- | -------------------------------------------------------- | -------- |
| [`heartbeat`][heartbeat]                                       | Configures the `heartbeat` collector.                    | no       |
| [`info_schema.processlist`][info_schema.processlist]           | Configures the `info_schema.processlist` collector.      | no       |
| [`info_schema.tables`][info_schema.tables]                     | Configures the `info_schema.tables` collector.           | no       |
| [`mysql.user`][mysql.user]                                     | Configures the `mysql.user` collector.                   | no       |
| [`perf_schema.eventsstatements`][perf_schema.eventsstatements] | Configures the `perf_schema.eventsstatements` collector. | no       |
| [`perf_schema.file_instances`][perf_schema.file_instances]     | Configures the `perf_schema.file_instances` collector.   | no       |
| [`perf_schema.memory_events`][perf_schema.memory_events]       | Configures the `perf_schema.memory_events` collector.    | no       |

[info_schema.processlist]: #info_schemaprocesslist
[info_schema.tables]: #info_schematables
[perf_schema.eventsstatements]: #perf_schemaeventsstatements
[perf_schema.file_instances]: #perf_schemafile_instances
[perf_schema.memory_events]: #perf_schemamemory_events
[heartbeat]: #heartbeat
[mysql.user]: #mysqluser

{{< /docs/alloy-config >}}

### `heartbeat`

The `heartbeat` block configures the `heartbeat` collector.
This configuration takes effect only when you enable the `heartbeat` collector.

You can use the following arguments with the `heartbeat` block:

| Name       | Type     | Description                                                                                     | Default       | Required |
| ---------- | -------- | ----------------------------------------------------------------------------------------------- | ------------- | -------- |
| `database` | `string` | Database to collect heartbeat data from.                                                        | `"heartbeat"` | no       |
| `table`    | `string` | Table to collect heartbeat data from.                                                           | `"heartbeat"` | no       |
| `utc`      | `bool`   | Uses UTC for the current server timestamp. Set to `true` when `pt-heartbeat` runs with `--utc`. | `false`       | no       |

### `info_schema.processlist`

The `info_schema.processlist` block configures the `info_schema.processlist` collector.
This configuration takes effect only when you enable the `info_schema.processlist` collector.

You can use the following arguments with the `info_schema.processlist` block:

| Name                | Type   | Description                                                                             | Default | Required |
| ------------------- | ------ | --------------------------------------------------------------------------------------- | ------- | -------- |
| `min_time`          | `int`  | Minimum time, in seconds, a thread must be in each state for the collector to count it. | `0`     | no       |
| `processes_by_host` | `bool` | Enable collecting the number of processes by host.                                      | `true`  | no       |
| `processes_by_user` | `bool` | Enable collecting the number of processes by user.                                      | `true`  | no       |

### `info_schema.tables`

The `info_schema.tables` block configures the `info_schema.tables` collector.
This configuration takes effect only when you enable the `info_schema.tables` collector.

You can use the following arguments with the `info_schema.tables` block:

| Name        | Type     | Description                                                                     | Default | Required |
| ----------- | -------- | ------------------------------------------------------------------------------- | ------- | -------- |
| `databases` | `string` | Comma-separated list of databases to collect table stats for, or `"*"` for all. | `"*"`   | no       |

### `mysql.user`

The `mysql.user` block configures the `mysql.user` collector.
This configuration takes effect only when you enable the `mysql.user` collector.

You can use the following arguments with the `mysql.user` block:

| Name         | Type   | Description                                          | Default | Required |
| ------------ | ------ | ---------------------------------------------------- | ------- | -------- |
| `privileges` | `bool` | Enable collecting user privileges from `mysql.user`. | `false` | no       |

### `perf_schema.eventsstatements`

The `perf_schema.eventsstatements` block configures the `perf_schema.eventsstatements` collector.
This configuration takes effect only when you enable the `perf_schema.eventsstatements` collector.

You can use the following arguments with the `perf_schema.eventsstatements` block:

| Name              | Type           | Description                                                                        | Default | Required |
| ----------------- | -------------- | ---------------------------------------------------------------------------------- | ------- | -------- |
| `exclude_schemas` | `list(string)` | List of schemas to exclude from events statements collection.                      |         | no       |
| `limit`           | `int`          | Limit the number of events statements digests, in descending order by `last_seen`. | `250`   | no       |
| `text_limit`      | `int`          | Maximum length of the normalized statement text.                                   | `120`   | no       |
| `time_limit`      | `int`          | Maximum age, in seconds, of the `last_seen` events statements.                     | `86400` | no       |

### `perf_schema.file_instances`

The `perf_schema.file_instances` block configures the `perf_schema.file_instances` collector.
This configuration takes effect only when you enable the `perf_schema.file_instances` collector.

You can use the following arguments with the `perf_schema.file_instances` block:

| Name            | Type     | Description                                                                         | Default            | Required |
| --------------- | -------- | ----------------------------------------------------------------------------------- | ------------------ | -------- |
| `filter`        | `string` | Regular expression to select rows in `performance_schema.file_summary_by_instance`. | `".*"`             | no       |
| `remove_prefix` | `string` | Prefix to trim away from `file_name`.                                               | `"/var/lib/mysql"` | no       |

Refer to [File summary tables][file-summary-tables] in the MySQL documentation for more information about the table that `filter` and `remove_prefix` apply to.

[file-summary-tables]: https://dev.mysql.com/doc/mysql-perfschema-excerpt/8.0/en/performance-schema-file-summary-tables.html

### `perf_schema.memory_events`

The `perf_schema.memory_events` block configures the `perf_schema.memory_events` collector.
This configuration takes effect only when you enable the `perf_schema.memory_events` collector.

You can use the following arguments with the `perf_schema.memory_events` block:

| Name            | Type     | Description                                                                        | Default     | Required |
| --------------- | -------- | ---------------------------------------------------------------------------------- | ----------- | -------- |
| `remove_prefix` | `string` | Prefix to trim away from `performance_schema.memory_summary_global_by_event_name`. | `"memory/"` | no       |

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.mysql` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.mysql` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.mysql` doesn't expose any component-specific debug metrics.

## Example

The following example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.mysql`:

```alloy
prometheus.exporter.mysql "example" {
  data_source_name  = "root@(server-a:3306)/"
  enable_collectors = ["heartbeat", "mysql.user"]
}

// Configure a prometheus.scrape component to collect mysql metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.mysql.example.targets
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

`prometheus.exporter.mysql` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

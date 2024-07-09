---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.mysql/
aliases:
  - ../prometheus.exporter.mysql/ # /docs/alloy/latest/reference/components/prometheus.exporter.mysql/
description: Learn about prometheus.exporter.mysql
title: prometheus.exporter.mysql
---

# prometheus.exporter.mysql

The `prometheus.exporter.mysql` component embeds
[mysqld_exporter](https://github.com/prometheus/mysqld_exporter) for collecting stats from a MySQL server.

## Usage

```alloy
prometheus.exporter.mysql "LABEL" {
    data_source_name = DATA_SOURCE_NAME
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
All arguments are optional. Omitted fields take their default values.

| Name                 | Type           | Description                                                                                                         | Default | Required |
| -------------------- | -------------- | ------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `data_source_name`   | `secret`       | [Data Source Name](https://github.com/go-sql-driver/mysql#dsn-data-source-name) for the MySQL server to connect to. |         | yes      |
| `enable_collectors`  | `list(string)` | A list of [collectors][] to enable on top of the default set.                                                       |         | no       |
| `disable_collectors` | `list(string)` | A list of [collectors][] to disable from the default set.                                                           |         | no       |
| `set_collectors`     | `list(string)` | A list of [collectors][] to run. Fully overrides the default set.                                                   |         | no       |
| `lock_wait_timeout`  | `int`          | Timeout, in seconds, to acquire a metadata lock.                                                                    | `2`     | no       |
| `log_slow_filter`    | `bool`         | Used to avoid queries from scrapes being logged in the slow query log.                                              | `false` | no       |

Set a `lock_wait_timeout` on the connection to avoid potentially long wait times for metadata locks. View more detailed documentation on `lock_wait_timeout` [in the MySQL documentation](https://dev.mysql.com/doc/refman/8.0/en/server-system-variables.html#sysvar_lock_wait_timeout).

> **NOTE**: `log_slow_filter` is not supported by Oracle MySQL.

[collectors]: #supported-collectors

## Blocks

The following blocks are supported inside the definition of
`prometheus.exporter.mysql` to configure collector-specific options:

| Hierarchy                    | Name                             | Description                                              | Required |
| ---------------------------- | -------------------------------- | -------------------------------------------------------- | -------- |
| info_schema.processlist      | [info_schema.processlist][]      | Configures the `info_schema.processlist` collector.      | no       |
| info_schema.tables           | [info_schema.tables][]           | Configures the `info_schema.tables` collector.           | no       |
| perf_schema.eventsstatements | [perf_schema.eventsstatements][] | Configures the `perf_schema.eventsstatements` collector. | no       |
| perf_schema.file_instances   | [perf_schema.file_instances][]   | Configures the `perf_schema.file_instances` collector.   | no       |
| perf_schema.memory_events    | [perf_schema.memory_events][]    | Configures the `perf_schema.memory_events` collector.    | no       |
| heartbeat                    | [heartbeat][]                    | Configures the `heartbeat` collector.                    | no       |
| mysql.user                   | [mysql.user][]                   | Configures the `mysql.user` collector.                   | no       |

[info_schema.processlist]: #info_schemaprocesslist-block
[info_schema.tables]: #info_schematables-block
[perf_schema.eventsstatements]: #perf_schemaeventsstatements-block
[perf_schema.file_instances]: #perf_schemafile_instances-block
[perf_schema.memory_events]: #perf_schemamemory_events-block
[heartbeat]: #heartbeat-block
[mysql.user]: #mysqluser-block

### info_schema.processlist block

| Name                | Type   | Description                                                | Default | Required |
| ------------------- | ------ | ---------------------------------------------------------- | ------- | -------- |
| `min_time`          | `int`  | Minimum time a thread must be in each state to be counted. | `0`     | no       |
| `processes_by_user` | `bool` | Enable collecting the number of processes by user.         | `true`  | no       |
| `processes_by_host` | `bool` | Enable collecting the number of processes by host.         | `true`  | no       |

### info_schema.tables block

| Name                           | Type     | Description                                                       | Default | Required |
| ------------------------------ | -------- | ----------------------------------------------------------------- | ------- | -------- |
| `info_schema_tables_databases` | `string` | Regular expression to match databases to collect table stats for. | `"*"`   | no       |

### perf_schema.eventsstatements block

| Name         | Type  | Description                                                                        | Default | Required |
| ------------ | ----- | ---------------------------------------------------------------------------------- | ------- | -------- |
| `limit`      | `int` | Limit the number of events statements digests, in descending order by `last_seen`. | `250`   | no       |
| `time_limit` | `int` | Limit how old, in seconds, the `last_seen` events statements can be.               | `86400` | no       |
| `text_limit` | `int` | Maximum length of the normalized statement text.                                   | `120`   | no       |

### perf_schema.file_instances block

| Name            | Type     | Description                                                                         | Default            | Required |
| --------------- | -------- | ----------------------------------------------------------------------------------- | ------------------ | -------- |
| `filter`        | `string` | Regular expression to select rows in `performance_schema.file_summary_by_instance`. | `".*"`             | no       |
| `remove_prefix` | `string` | Prefix to trim away from `file_name`.                                               | `"/var/lib/mysql"` | no       |

View more detailed documentation on the tables used in `perf_schema_file_instances_filter` and `perf_schema_file_instances_remove_prefix` [in the MySQL documentation](https://dev.mysql.com/doc/mysql-perfschema-excerpt/8.0/en/performance-schema-file-summary-tables.html).

### perf_schema.memory_events block

| Name            | Type     | Description                                                                         | Default            | Required |
| --------------- | -------- | ----------------------------------------------------------------------------------- | ------------------ | -------- |
| `remove_prefix` | `string` | Prefix to trim away from `performance_schema.memory_summary_global_by_event_name`.  | `"memory/"`        | no       |

### heartbeat block

| Name       | Type     | Description                                                                           | Default       | Required |
| ---------- | -------- | ------------------------------------------------------------------------------------- | ------------- | -------- |
| `database` | `string` | Database to collect heartbeat data from.                                              | `"heartbeat"` | no       |
| `table`    | `string` | Table to collect heartbeat data from.                                                 | `"heartbeat"` | no       |
| `utc`      | `bool`   | Use UTC for timestamps of the current server (`pt-heartbeat` is called with `--utc`). | `false`       | no       |

### mysql.user block

| Name         | Type   | Description                                          | Default | Required |
| ------------ | ------ | ---------------------------------------------------- | ------- | -------- |
| `privileges` | `bool` | Enable collecting user privileges from `mysql.user`. | `false` | no       |

### Supported Collectors

The full list of supported collectors is:

| Name                                             | Description                                                                                  | Enabled by default |
| ------------------------------------------------ | -------------------------------------------------------------------------------------------- | ------------------ |
| auto_increment.columns                           | Collect `auto_increment` columns and max values from `information_schema`.                   | no                 |
| binlog_size                                      | Collect the current size of all registered `binlog` files.                                   | no                 |
| engine_innodb_status                             | Collect metrics from `SHOW ENGINE INNODB STATUS`.                                            | no                 |
| engine_tokudb_status                             | Collect metrics from `SHOW ENGINE TOKUDB STATUS`.                                            | no                 |
| global_status                                    | Collect metrics from `SHOW GLOBAL STATUS`.                                                   | yes                |
| global_variables                                 | Collect metrics from `SHOW GLOBAL VARIABLES`.                                                | yes                |
| heartbeat                                        | Collect metrics from heartbeat database and tables.                                          | no                 |
| info_schema.clientstats                          | If running with userstat=1, enable to collect client statistics.                             | no                 |
| info_schema.innodb_cmpmem                        | Collect metrics from `information_schema.innodb_cmpmem`.                                     | yes                |
| info_schema.innodb_metrics                       | Collect metrics from `information_schema.innodb_metrics`.                                    | yes                |
| info_schema.innodb_tablespaces                   | Collect metrics from `information_schema.innodb_sys_tablespaces`.                            | no                 |
| info_schema.processlist                          | Collect current thread state counts from the `information_schema.processlist`.               | no                 |
| info_schema.query_response_time                  | Collect query response time distribution if `query_response_time_stats` is ON.               | yes                |
| info_schema.replica_host                         | Collect metrics from `information_schema.replica_host_status`.                               | no                 |
| info_schema.schemastats                          | If running with userstat=1, enable to collect schema statistics.                             | no                 |
| info_schema.tables                               | Collect metrics from `information_schema.tables`.                                            | no                 |
| info_schema.tablestats                           | If running with userstat=1, enable to collect table statistics.                              | no                 |
| info_schema.userstats                            | If running with userstat=1, enable to collect user statistics.                               | no                 |
| mysql.user                                       | Collect data from `mysql.user`.                                                              | no                 |
| perf_schema.eventsstatements                     | Collect metrics from `performance_schema.events_statements_summary_by_digest`.               | no                 |
| perf_schema.eventsstatementssum                  | Collect metrics of grand sums from `performance_schema.events_statements_summary_by_digest`. | no                 |
| perf_schema.eventswaits                          | Collect metrics from `performance_schema.events_waits_summary_global_by_event_name`.         | no                 |
| perf_schema.file_events                          | Collect metrics from `performance_schema.file_summary_by_event_name`.                        | no                 |
| perf_schema.file_instances                       | Collect metrics from `performance_schema.file_summary_by_instance`.                          | no                 |
| perf_schema.indexiowaits                         | Collect metrics from `performance_schema.table_io_waits_summary_by_index_usage`.             | no                 |
| perf_schema.memory_events                        | Collect metrics from `performance_schema.memory_summary_global_by_event_name`.               | no                 |
| perf_schema.replication_applier_status_by_worker | Collect metrics from `performance_schema.replication_applier_status_by_worker`.              | no                 |
| perf_schema.replication_group_member_stats       | Collect metrics from `performance_schema.replication_group_member_stats`.                    | no                 |
| perf_schema.replication_group_members            | Collect metrics from `performance_schema.replication_group_members`.                         | no                 |
| perf_schema.tableiowaits                         | Collect metrics from `performance_schema.table_io_waits_summary_by_table`.                   | no                 |
| perf_schema.tablelocks                           | Collect metrics from `performance_schema.table_lock_waits_summary_by_table`.                 | no                 |
| slave_hosts                                      | Scrape information from `SHOW SLAVE HOSTS`.                                                  | no                 |
| slave_status                                     | Scrape information from `SHOW SLAVE STATUS`.                                                 | yes                |

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.mysql` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.mysql` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.mysql` does not expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.mysql`:

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
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

Replace the following:

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the remote_write API.
- `PASSWORD`: The password to use for authentication to the remote_write API.

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

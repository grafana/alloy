---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.mssql/
aliases:
  - ../prometheus.exporter.mssql/ # /docs/alloy/latest/reference/components/prometheus.exporter.mssql/
description: Learn about prometheus.exporter.mssql
title: prometheus.exporter.mssql
---

# prometheus.exporter.mssql

The `prometheus.exporter.mssql` component embeds [`sql_exporter`](https://github.com/burningalchemist/sql_exporter) for collecting stats from a Microsoft SQL Server and exposing them as Prometheus metrics.

## Usage

```alloy
prometheus.exporter.mssql "LABEL" {
    connection_string = CONNECTION_STRING
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
Omitted fields take their default values.

| Name                   | Type       | Description                                                         | Default | Required |
| ---------------------- | ---------- | ------------------------------------------------------------------- | ------- | -------- |
| `connection_string`    | `secret`   | The connection string used to connect to an Microsoft SQL Server.   |         | yes      |
| `max_idle_connections` | `int`      | Maximum number of idle connections to any one target.               | `3`     | no       |
| `max_open_connections` | `int`      | Maximum number of open connections to any one target.               | `3`     | no       |
| `timeout`              | `duration` | The query timeout in seconds.                                       | `"10s"` | no       |
| `query_config`         | `string`   | MSSQL query to Prometheus metric configuration as an inline string. |         | no       |

The [`sql_exporter` examples](https://github.com/burningalchemist/sql_exporter/blob/master/examples/azure-sql-mi/sql_exporter.yml#L21) show the format of the `connection_string` argument:

```conn
sqlserver://USERNAME_HERE:PASSWORD_HERE@SQLMI_HERE_ENDPOINT.database.windows.net:1433?encrypt=true&hostNameInCertificate=%2A.SQL_MI_DOMAIN_HERE.database.windows.net&trustservercertificate=true
```

If specified, the `query_config` argument must be a YAML document as string defining which MSSQL queries map to custom Prometheus metrics.
`query_config` is typically loaded by using the exports of another component.
For example,

- `local.file.LABEL.content`
- `remote.http.LABEL.content`
- `remote.s3.LABEL.content`

Refer to [sql_exporter](https://github.com/burningalchemist/sql_exporter#collectors) for details on how to create a configuration.

### Authentication

By default, the `USERNAME` and `PASSWORD` used within the `connection_string` argument corresponds to a SQL Server username and password.

If {{< param "PRODUCT_NAME" >}} is running in the same Windows domain as the SQL Server, then you can use the parameter `authenticator=winsspi` within the `connection_string` to authenticate without any additional credentials.

```conn
sqlserver://@<HOST>:<PORT>?authenticator=winsspi
```

If you want to use Windows credentials to authenticate, instead of SQL Server credentials, you can use the parameter `authenticator=ntlm` within the `connection_string`.
The `USERNAME` and `PASSWORD` then corresponds to a Windows username and password.
The Windows domain may need to be prefixed to the username with a trailing `\`.

```conn
sqlserver://<DOMAIN\USERNAME>:<PASSWORD>@<HOST>:<PORT>?authenticator=ntlm
```

## Blocks

The `prometheus.exporter.mssql` component does not support any blocks, and is configured fully through arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.mssql` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.mssql` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.mssql` doesn't expose any component-specific debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.mssql`:

```alloy
prometheus.exporter.mssql "example" {
  connection_string = "sqlserver://user:pass@localhost:1433"
}

// Configure a prometheus.scrape component to collect mssql metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.mssql.example.targets
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
- `USERNAME`: The username to use for authentication to the `remote_write` API.
- `PASSWORD`: The password to use for authentication to the `remote_write` API.

[scrape]: ../prometheus.scrape/

## Custom metrics

You can use the optional `query_config` parameter to retrieve custom Prometheus metrics for a MSSQL instance.

If this is defined, the new configuration will be used to query your MSSQL instance and create whatever Prometheus metrics are defined.
If you want additional metrics on top of the default metrics, the default configuration must be used as a base.

The default configuration used by this integration is as follows:

```
collector_name: mssql_standard

metrics:
  - metric_name: mssql_local_time_seconds
    type: gauge
    help: 'Local time in seconds since epoch (Unix time).'
    values: [unix_time]
    query: |
      SELECT DATEDIFF(second, '19700101', GETUTCDATE()) AS unix_time
  - metric_name: mssql_connections
    type: gauge
    help: 'Number of active connections.'
    key_labels:
      - db
    values: [count]
    query: |
      SELECT DB_NAME(sp.dbid) AS db, COUNT(sp.spid) AS count
      FROM sys.sysprocesses sp
      GROUP BY DB_NAME(sp.dbid)
  #
  # Collected from sys.dm_os_performance_counters
  #
  - metric_name: mssql_deadlocks_total
    type: counter
    help: 'Number of lock requests that resulted in a deadlock.'
    values: [cntr_value]
    query: |
      SELECT cntr_value
      FROM sys.dm_os_performance_counters WITH (NOLOCK)
      WHERE counter_name = 'Number of Deadlocks/sec' AND instance_name = '_Total'
  - metric_name: mssql_user_errors_total
    type: counter
    help: 'Number of user errors.'
    values: [cntr_value]
    query: |
      SELECT cntr_value
      FROM sys.dm_os_performance_counters WITH (NOLOCK)
      WHERE counter_name = 'Errors/sec' AND instance_name = 'User Errors'
  - metric_name: mssql_kill_connection_errors_total
    type: counter
    help: 'Number of severe errors that caused SQL Server to kill the connection.'
    values: [cntr_value]
    query: |
      SELECT cntr_value
      FROM sys.dm_os_performance_counters WITH (NOLOCK)
      WHERE counter_name = 'Errors/sec' AND instance_name = 'Kill Connection Errors'
  - metric_name: mssql_page_life_expectancy_seconds
    type: gauge
    help: 'The minimum number of seconds a page will stay in the buffer pool on this node without references.'
    values: [cntr_value]
    query: |
      SELECT top(1) cntr_value
      FROM sys.dm_os_performance_counters WITH (NOLOCK)
      WHERE counter_name = 'Page life expectancy'
  - metric_name: mssql_batch_requests_total
    type: counter
    help: 'Number of command batches received.'
    values: [cntr_value]
    query: |
      SELECT cntr_value
      FROM sys.dm_os_performance_counters WITH (NOLOCK)
      WHERE counter_name = 'Batch Requests/sec'
  - metric_name: mssql_log_growths_total
    type: counter
    help: 'Number of times the transaction log has been expanded, per database.'
    key_labels:
      - db
    values: [cntr_value]
    query: |
      SELECT rtrim(instance_name) AS db, cntr_value
      FROM sys.dm_os_performance_counters WITH (NOLOCK)
      WHERE counter_name = 'Log Growths' AND instance_name <> '_Total'
  - metric_name: mssql_buffer_cache_hit_ratio
    type: gauge
    help: 'Ratio of requests that hit the buffer cache'
    values: [BufferCacheHitRatio]
    query: |
      SELECT (a.cntr_value * 1.0 / b.cntr_value) * 100.0 as BufferCacheHitRatio
      FROM sys.dm_os_performance_counters  a
      JOIN  (SELECT cntr_value, OBJECT_NAME
          FROM sys.dm_os_performance_counters
          WHERE counter_name = 'Buffer cache hit ratio base'
              AND OBJECT_NAME = 'SQLServer:Buffer Manager') b ON  a.OBJECT_NAME = b.OBJECT_NAME
      WHERE a.counter_name = 'Buffer cache hit ratio'
      AND a.OBJECT_NAME = 'SQLServer:Buffer Manager'

  - metric_name: mssql_checkpoint_pages_sec
    type: gauge
    help: 'Checkpoint Pages Per Second'
    values: [cntr_value]
    query: |
      SELECT cntr_value
      FROM sys.dm_os_performance_counters
      WHERE [counter_name] = 'Checkpoint pages/sec'
  #
  # Collected from sys.dm_io_virtual_file_stats
  #
  - metric_name: mssql_io_stall_seconds_total
    type: counter
    help: 'Stall time in seconds per database and I/O operation.'
    key_labels:
      - db
    value_label: operation
    values:
      - read
      - write
    query_ref: mssql_io_stall

  #
  # Collected from sys.dm_os_process_memory
  #
  - metric_name: mssql_resident_memory_bytes
    type: gauge
    help: 'SQL Server resident memory size (AKA working set).'
    values: [resident_memory_bytes]
    query_ref: mssql_process_memory

  - metric_name: mssql_virtual_memory_bytes
    type: gauge
    help: 'SQL Server committed virtual memory size.'
    values: [virtual_memory_bytes]
    query_ref: mssql_process_memory

  - metric_name: mssql_available_commit_memory_bytes
    type: gauge
    help: 'SQL Server available to be committed memory size.'
    values: [available_commit_limit_bytes]
    query_ref: mssql_process_memory

  - metric_name: mssql_memory_utilization_percentage
    type: gauge
    help: 'The percentage of committed memory that is in the working set.'
    values: [memory_utilization_percentage]
    query_ref: mssql_process_memory

  - metric_name: mssql_page_fault_count_total
    type: counter
    help: 'The number of page faults that were incurred by the SQL Server process.'
    values: [page_fault_count]
    query_ref: mssql_process_memory

  #
  # Collected from sys.dm_os_sys_info
  #
  - metric_name: mssql_server_total_memory_bytes
    type: gauge
    help: 'SQL Server committed memory in the memory manager.'
    values: [committed_memory_bytes]
    query_ref: mssql_os_sys_info

  - metric_name: mssql_server_target_memory_bytes
    type: gauge
    help: 'SQL Server target committed memory set for the memory manager.'
    values: [committed_memory_target_bytes]
    query_ref: mssql_os_sys_info

  #
  # Collected from sys.dm_os_sys_memory
  #
  - metric_name: mssql_os_memory
    type: gauge
    help: 'OS physical memory, used and available.'
    value_label: 'state'
    values: [used, available]
    query: |
      SELECT
        (total_physical_memory_kb - available_physical_memory_kb) * 1024 AS used,
        available_physical_memory_kb * 1024 AS available
      FROM sys.dm_os_sys_memory
  - metric_name: mssql_os_page_file
    type: gauge
    help: 'OS page file, used and available.'
    value_label: 'state'
    values: [used, available]
    query: |
      SELECT
        (total_page_file_kb - available_page_file_kb) * 1024 AS used,
        available_page_file_kb * 1024 AS available
      FROM sys.dm_os_sys_memory
queries:
  # Populates `mssql_io_stall` and `mssql_io_stall_total`
  - query_name: mssql_io_stall
    query: |
      SELECT
        cast(DB_Name(a.database_id) as varchar) AS [db],
        sum(io_stall_read_ms) / 1000.0 AS [read],
        sum(io_stall_write_ms) / 1000.0 AS [write]
      FROM
        sys.dm_io_virtual_file_stats(null, null) a
      INNER JOIN sys.master_files b ON a.database_id = b.database_id AND a.file_id = b.file_id
      GROUP BY a.database_id
  # Populates `mssql_resident_memory_bytes`, `mssql_virtual_memory_bytes`, mssql_available_commit_memory_bytes,
  # and `mssql_memory_utilization_percentage`, and `mssql_page_fault_count_total`
  - query_name: mssql_process_memory
    query: |
      SELECT
        physical_memory_in_use_kb * 1024 AS resident_memory_bytes,
        virtual_address_space_committed_kb * 1024 AS virtual_memory_bytes,
        available_commit_limit_kb * 1024 AS available_commit_limit_bytes,
        memory_utilization_percentage,
        page_fault_count
      FROM sys.dm_os_process_memory
  # Populates `mssql_server_total_memory_bytes` and `mssql_server_target_memory_bytes`.
  - query_name: mssql_os_sys_info
    query: |
      SELECT
        committed_kb * 1024 AS committed_memory_bytes,
        committed_target_kb * 1024 AS committed_memory_target_bytes
      FROM sys.dm_os_sys_info
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.mssql` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

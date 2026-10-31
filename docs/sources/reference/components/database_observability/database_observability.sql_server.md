---
canonical: https://grafana.com/docs/alloy/latest/reference/components/database_observability.sql_server/
description: Learn about database_observability.sql_server
title: database_observability.sql_server
labels:
  stage: experimental
  products:
    - oss
---

# `database_observability.sql_server`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`database_observability.sql_server` connects to a Microsoft SQL Server instance and collects observability data across every accessible user database on that instance.
The component forwards this data as log entries to Loki receivers and exports targets for Prometheus scraping.

## Usage

```alloy
database_observability.sql_server "<LABEL>" {
  data_source_name = <DATA_SOURCE_NAME>
  forward_to       = [<LOKI_RECEIVERS>]
}
```

## Arguments

You can use the following arguments with `database_observability.sql_server`:

| Name                | Type                 | Description                                                              | Default | Required |
|---------------------|----------------------|--------------------------------------------------------------------------|---------|----------|
| `data_source_name`  | `secret`             | [Data Source Name][] for the SQL Server instance to connect to. Required when no `database_instance` blocks are defined. |         | no       |
| `forward_to`        | `list(LogsReceiver)` | Where to forward log entries after processing.                           |         | yes      |
| `targets`           | `list(map(string))`  | List of external targets to scrape.                                      |         | no       |
| `disable_collectors`| `list(string)`       | A list of collectors to disable from the default set.                    |         | no       |
| `enable_collectors` | `list(string)`       | A list of collectors to enable on top of the default set.                |         | no       |
| `exclude_current_user` | `bool`            | Exclude tagged Alloy queries and query samples from sessions matching the Alloy application name or opened with the login that Alloy uses. | `true` | no       |
| `exclude_schemas`   | `list(string)`       | A list of schemas to exclude from monitoring, on top of the always-excluded system schemas `sys` and `information_schema`. | `["alloydbadmin", "alloydbmetadata", "azure_maintenance", "azure_sys", "cloudsqladmin", "rdsadmin"]` | no       |
| `exclude_databases` | `list(string)`       | A list of databases to exclude from monitoring, on top of the always-excluded system databases `master`, `model`, `msdb`, and `tempdb`. | `["alloydbadmin", "alloydbmetadata", "azure_maintenance", "azure_sys", "cloudsqladmin", "rdsadmin"]` | no       |
| `exclude_users`     | `list(string)`       | A list of original SQL Server login names to exclude from query samples. | `["azuresu", "cloudsqladmin", "db-o11y", "rdsadmin"]` | no       |
| `query_timeout`     | `duration`           | Timeout for each SQL statement.                                          | `"10s"` | no       |

When `data_source_name` doesn't specify an application name (`app name`), Alloy sets it to `grafana:alloy:database_observability` on its SQL Server connections.

The `exclude_current_user` argument excludes SQL tagged by Alloy from query metrics, query details, explain plans, and query samples.
Query Store doesn't identify the login or application name that executed a query, so this exclusion can't remove arbitrary historical queries by login or previously captured Alloy queries without the marker.

For live query samples, `exclude_current_user` also excludes sessions whose `sys.dm_exec_sessions.program_name` is `grafana:alloy:database_observability` or that were opened with the SQL Server login configured for Alloy.
For `server_metrics`, `exclude_current_user` excludes sessions whose `sys.dm_exec_sessions.program_name` is `grafana:alloy:database_observability` from `database_observability_sql_server_user_sessions`.
Setting it to `false` disables marker, application name, and automatic current-login exclusion.
The `exclude_users` list still applies to query samples, and Alloy always excludes the request that collects the samples.
Schema discovery, server configuration, and health-check results aren't filtered by this argument.

The `query_timeout` applies separately to each SQL statement.
The timeout includes waiting for an available connection and reading the statement results.
A collection cycle can run multiple statements and can take longer than `query_timeout`.
The `server_metrics` collector also runs multiple statements during a Prometheus scrape, so the total scrape duration can exceed `query_timeout`.

The following collectors are configurable:

| Name              | Description                                                                   | Enabled by default |
|-------------------|-------------------------------------------------------------------------------|--------------------|
| `explain_plans`   | Collect and parse query execution plans already captured by Query Store.      | yes                |
| `query_details`   | Collect query text and parsed table names from Query Store.                   | yes                |
| `query_metrics`   | Collect per-query executions, errors, and duration counters from Query Store. | yes                |
| `query_samples`   | Collect query samples and wait events for tracked queries.                    | yes                |
| `schema_details`  | Collect schemas and tables from `information_schema`.                         | yes                |
| `server_configuration` | Collect connection, resource-limit, and memory configuration metrics.     | yes                |
| `server_metrics`  | Collect SQL Server CPU, memory, and user session metrics.                      | yes                |

## Blocks

You can use the following blocks with `database_observability.sql_server`:

{{< docs/alloy-config >}}

| Block                                | Description                                       | Required |
|--------------------------------------|---------------------------------------------------|----------|
| [`cloud_provider`][cloud_provider]   | Provide Cloud Provider information.               | no       |
| `cloud_provider` > [`aws`][aws]      | Provide AWS database host information.            | no       |
| `cloud_provider` > [`azure`][azure]  | Provide Azure database host information.          | no       |
| `cloud_provider` > [`gcp`][gcp]      | Provide GCP database host information.            | no       |
| [`database_instance`][database_instance] | Define one SQL Server instance to monitor. Repeat the block to monitor several instances. | no |
| `database_instance` > [`cloud_provider`][cloud_provider] | Provide Cloud Provider information for one instance. | no |
| `database_instance` > [`rule`][rule] | Relabel the metrics and logs of one instance. | no |
| [`clustering`][clustering]           | Configure the component for when {{< param "PRODUCT_NAME" >}} is running in clustered mode. | no |
| [`explain_plans`][explain_plans]     | Configure the query execution plan collector.     | no       |
| [`health_check`][health_check]       | Configure the health check collector.             | no       |
| [`query_details`][query_details]     | Configure the Query Store query text collector.   | no       |
| [`query_metrics`][query_metrics]     | Configure the Query Store metrics collector.      | no       |
| [`query_samples`][query_samples]     | Configure the query samples collector.            | no       |
| [`schema_details`][schema_details]   | Configure the schema and table details collector. | no       |
| [`server_configuration`][server_configuration] | Configure the server configuration collector. | no       |

[cloud_provider]: #cloud_provider
[aws]: #aws
[azure]: #azure
[gcp]: #gcp
[database_instance]: #database_instance
[rule]: #rule
[clustering]: #clustering
[explain_plans]: #explain_plans
[health_check]: #health_check
[query_details]: #query_details
[query_metrics]: #query_metrics
[query_samples]: #query_samples
[schema_details]: #schema_details
[server_configuration]: #server_configuration

{{< /docs/alloy-config >}}

### `cloud_provider`

The `cloud_provider` block has no attributes.
It contains zero or one of the [`aws`][aws], [`azure`][azure], or [`gcp`][gcp] blocks.
You use the `cloud_provider` block to provide information related to the cloud provider that hosts the database under observation.
This information is appended as labels to the collected metrics.
The labels make it easier for you to filter and group your metrics.

When you don't configure a `cloud_provider` block, {{< param "PRODUCT_NAME" >}} attempts to detect AWS RDS and Azure SQL hosts from the `data_source_name`.

[aws]: #aws
[azure]: #azure
[gcp]: #gcp

### `aws`

The `aws` block supplies the [ARN](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html) identifier for the database being monitored.

| Name  | Type     | Description                                             | Default | Required |
|-------|----------|---------------------------------------------------------|---------|----------|
| `arn` | `string` | The ARN associated with the database under observation. |         | yes      |

### `azure`

The `azure` block supplies the identifying information for the database being monitored.

| Name              | Type     | Description                                          | Default | Required |
|-------------------|----------|------------------------------------------------------|---------|----------|
| `subscription_id` | `string` | The Subscription ID for your Azure account.          |         | yes      |
| `resource_group`  | `string` | The Resource Group that holds the database resource. |         | yes      |
| `server_name`     | `string` | The database server name, for example `orders-db` for the host `orders-db.database.windows.net`. |         | no       |

### `gcp`

The `gcp` block supplies the identifying information for the GCP Cloud SQL database being monitored.

| Name              | Type     | Description                                                                                                                 | Default | Required |
|-------------------|----------|-----------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `connection_name` | `string` | The Cloud SQL instance connection name in the format `project:region:instance`, for example `my-project:us-central1:my-db`. |         | yes      |

### `database_instance`

The `database_instance` block defines one SQL Server instance to monitor.
Repeat the block to monitor several instances with a single component.
The block label must be unique across `database_instance` blocks and identifies the instance in the component's metrics endpoint path.
Each `database_instance` block must also point to a distinct server: two blocks that resolve to the same host, port or named instance, and database name are rejected.

| Name               | Type     | Description                                                      | Default | Required |
|--------------------|----------|-------------------------------------------------------------------|---------|----------|
| `data_source_name` | `secret` | [Data Source Name][] for the SQL Server instance to connect to. |         | yes      |

Each `database_instance` block can also contain a [`cloud_provider`][cloud_provider] block that applies to that instance only, and [`rule`][rule] blocks that relabel the metrics and logs of that instance only.

When you define `database_instance` blocks, don't set the top-level `data_source_name`, `targets`, and `cloud_provider` arguments.
They're mutually exclusive with `database_instance` blocks.
All other arguments and blocks, such as collector settings, apply to every configured instance.

The metrics for each instance are served on a separate `/db/<LABEL>/metrics` path under the component's HTTP endpoint, and the exported targets point to the corresponding path.
When you don't define `database_instance` blocks, the component serves metrics on its historical `/metrics` path.
The metrics endpoints are served exactly at those paths: requests to any other path under the component's HTTP endpoint return HTTP 404.

For example:

```alloy
database_observability.sql_server "pool" {
  forward_to = [loki.write.logs_service.receiver]

  database_instance "orders" {
    data_source_name = sys.env("ORDERS_DSN")

    cloud_provider {
      aws {
        arn = "orders-rds-db-arn"
      }
    }

    rule {
      target_label = "environment"
      replacement  = "production"
    }
  }

  database_instance "billing" {
    data_source_name = sys.env("BILLING_DSN")

    rule {
      target_label = "environment"
      replacement  = "staging"
    }
  }
}
```

In this example, the metrics and logs of the `orders` instance carry `environment="production"`, and those of the `billing` instance carry `environment="staging"`.

### `rule`

The `rule` block relabels the metrics and the logs of the `database_instance` block that contains it.
Use it to tell the data of each monitored instance apart, for example by environment, team, or tier.

{{< docs/shared lookup="reference/components/rule-block-database-observability.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `clustering`

| Name      | Type   | Description                                               | Default | Required |
|-----------|--------|-----------------------------------------------------------|---------|----------|
| `enabled` | `bool` | Enables distributing instances with other cluster nodes.  | `false` | yes      |

When {{< param "PRODUCT_NAME" >}} is [using clustering][], and `enabled` is set to true, then this `database_observability.sql_server` component instance opts-in to distributing its configured instances between all cluster nodes.

Clustering assumes that all cluster nodes are running with the same configuration file.
All component instances opting in to clustering use the instance key of each configured instance, `<host>:<port>/<database>`, and a consistent hashing algorithm to determine ownership of each instance between the cluster peers.
Each peer then only collects from the subset of instances it's responsible for, and only exports the targets of those instances, so `prometheus.scrape` components on the same node scrape exactly the instances the node owns.
When a node joins or leaves the cluster, every peer recalculates ownership: expect a short gap or a brief duplicate collection for an instance while its ownership moves.
While the cluster isn't yet ready to admit traffic, for example while it's still forming and waiting for the minimum cluster size, the component doesn't collect from any instance.

Clustering is also useful with a single instance: when several cluster nodes run an identical configuration, exactly one node collects from the instance at a time, which gives you a highly available setup without duplicate collection.

If {{< param "PRODUCT_NAME" >}} is _not_ running in clustered mode, then the block is a no-op and `database_observability.sql_server` collects from every configured instance.

[using clustering]: ../../../../get-started/clustering/

### `explain_plans`

| Name               | Type       | Description                                    | Default | Required |
|--------------------|------------|-------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to check for a changed execution plan. | `"1m"`  | no       |

The `explain_plans` collector reads the execution plan [Query Store][query_store] already captured for each query tracked by the `query_metrics` collector.
It doesn't compile or run a fresh plan.
Only queries that `query_metrics` is currently tracking are eligible.
When `query_metrics` is disabled, `explain_plans` produces no output.

The collector checks for a changed plan every `collect_interval`.
It forwards a log entry only when the plan's shape has changed since the last entry.
It also forwards a log entry when 30 minutes have passed since the last entry.
This keeps log volume low and ensures a fresh entry at least every 30 minutes.

### `health_check`

| Name               | Type       | Description                          | Default | Required |
|--------------------|------------|---------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to run health checks. | `"1h"`  | no       |

The `health_check` collector periodically verifies that the login has the required grants (`VIEW DEFINITION`, plus `VIEW DATABASE STATE` or its SQL Server 2022+ equivalent `VIEW DATABASE PERFORMANCE STATE`) and that [Query Store][query_store] is enabled and populated, on every database the login can access on the instance. Each check emits a log entry with `op="health_status"` and a `check` field identifying it: `AlloyVersion`, `RequiredGrantsPresent`, `QueryStoreEnabled`, or `QueryStoreHasRows`.

`RequiredGrantsPresent` and `QueryStoreEnabled` pass only when true on every database the login can access; when they fail, the log entry's value lists the non-compliant databases. `QueryStoreHasRows` passes as soon as any accessible database has Query Store data, since it's expected for a low-traffic database to have none.

### `query_details`

| Name               | Type       | Description                                            | Default | Required |
|--------------------|------------|--------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect query text from Query Store. | `"1m"`  | no       |

The `query_details` collector reads [Query Store][query_store] query text for the database selected in the `data_source_name`, not every database on the instance.

### `query_metrics`

| Name                  | Type       | Description                                                        | Default | Required |
|-----------------------|------------|-------------------------------------------------------------------|---------|----------|
| `collect_interval`    | `duration` | How frequently to collect metrics from Query Store.               | `"1m"`  | no       |
| `statements_limit`    | `int`      | Maximum number of queries to track, ranked by recent duration.    | `50`   | no       |
| `statements_lookback` | `duration` | Only queries executed within this window are eligible for tracking.| `"1h"`  | no       |

The `query_metrics` collector reads [Query Store][query_store] for the database selected in the `data_source_name`, not every database on the instance.
Configure the `data_source_name` to select a user database that has Query Store enabled.
When the connected database is a system database such as `master`, or Query Store is disabled or read-only, the collector skips collection and remains healthy.

The login requires `VIEW DATABASE STATE` on the connected database. On SQL Server 2022 and later, `VIEW DATABASE PERFORMANCE STATE` is also sufficient.

[query_store]: https://learn.microsoft.com/sql/relational-databases/performance/monitoring-performance-by-using-the-query-store

### `query_samples`

| Name                      | Type       | Description                                                   | Default | Required |
|---------------------------|------------|---------------------------------------------------------------|---------|----------|
| `collect_interval`        | `duration` | How frequently to collect query samples.                      | `"10s"` | no       |
| `disable_query_redaction` | `bool`     | Collect unredacted SQL query text (might include parameters). | `false` | no       |
| `sample_min_duration`     | `duration` | Minimum elapsed duration for a completed query sample to be emitted. Set to `"0s"` to disable filtering. | `"0s"`  | no       |
| `wait_event_min_duration` | `duration` | Minimum duration for a wait event occurrence to be emitted. Set to `"0s"` to disable filtering. | `"1ms"` | no       |
| `max_samples_per_query`   | `int`      | Maximum number of completed query samples emitted per `(database, query_hash)` in a single collection cycle. Set to `0` for no per-query limit. | `5`     | no       |
| `max_samples_per_collect` | `int`      | Maximum total number of completed query samples emitted across all tracked queries in a single collection cycle. Set to `0` for no per-collection limit. | `50`    | no       |

The `query_samples` collector only collects requests whose query hash is tracked by the `query_metrics` collector.

When requests finish between polls, the collector filters out any request whose final elapsed time is shorter than `sample_min_duration`.
Remaining completed requests are ranked by elapsed duration and then CPU time, and capped by `max_samples_per_query` and `max_samples_per_collect`.
When a request is dropped by `sample_min_duration`, `max_samples_per_query`, or `max_samples_per_collect`, neither the query sample nor its wait event occurrences are emitted.
For emitted samples, only wait event occurrences with a final duration of at least `wait_event_min_duration` are logged.

The collector polls live requests and can miss queries shorter than `collect_interval`, wait events that start and finish between collections, and a query's first execution before Query Store admits its hash.
For completed requests, the emitted resource counters contain the values from the final observation and can omit work performed after that observation.

The login requires `VIEW SERVER STATE` on SQL Server 2019 and earlier. On SQL Server 2022 and later, `VIEW SERVER PERFORMANCE STATE` is also sufficient. Azure SQL Database can restrict the dynamic management views to the current session. In that case, the collector can't observe other sessions.

### `schema_details`

| Name               | Type       | Description                                          | Default | Required |
|--------------------|------------|------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect information from database. | `"1m"`  | no       |

The collector scans every database that the login can access on the instance and collects schema details from each. Only databases where the login has `CONNECT` access to catalog views are collected.

### `server_configuration`

The `server_configuration` block configures how often `database_observability.sql_server` collects connection, resource-limit, and memory configuration metrics.

| Name               | Type       | Description                                      | Default | Required |
|--------------------|------------|--------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect configuration metrics. | `"10m"` | no       |

The collector queries configuration immediately after it starts and then every `collect_interval`.
Prometheus scrapes read the latest successful result from memory and don't query SQL Server.
If a query fails, the collector retains the last successful result and tries again at the next interval.
The top-level `query_timeout` applies to each configuration query.

For SQL Server, Amazon RDS for SQL Server, Google Cloud SQL for SQL Server, and Azure SQL Managed Instance, the collector reads the settings available in `sys.configurations`.
Managed database services can omit settings or manage their values.
The collector skips unavailable settings without failing the collection.

The collector exports the following metrics:

| Metric | Type | Source setting | Description |
|--------|------|----------------|-------------|
| `database_observability_sql_server_config_backup_compression_default` | gauge | `backup compression default` | Whether backup compression is enabled by default. |
| `database_observability_sql_server_config_cost_threshold_for_parallelism` | gauge | `cost threshold for parallelism` | Threshold at which SQL Server creates and runs parallel plans. |
| `database_observability_sql_server_config_database_max_degree_of_parallelism` | gauge | `sys.database_scoped_configurations.MAXDOP` | Maximum degree of parallelism for the current Azure SQL Database. |
| `database_observability_sql_server_config_index_create_memory_bytes` | gauge | `index create memory (KB)` | Minimum memory for index creation. |
| `database_observability_sql_server_config_max_degree_of_parallelism` | gauge | `max degree of parallelism` | Maximum processors used for a parallel plan. |
| `database_observability_sql_server_config_max_server_memory_bytes` | gauge | `max server memory (MB)` | Maximum memory available to the SQL Server memory manager. |
| `database_observability_sql_server_config_max_worker_threads` | gauge | `max worker threads` | Maximum worker threads. |
| `database_observability_sql_server_config_min_memory_per_query_bytes` | gauge | `min memory per query (KB)` | Minimum memory for query execution. |
| `database_observability_sql_server_config_min_server_memory_bytes` | gauge | `min server memory (MB)` | Minimum memory retained by the SQL Server memory manager. |
| `database_observability_sql_server_config_network_packet_size_bytes` | gauge | `network packet size (B)` | Default network packet size. |
| `database_observability_sql_server_config_optimize_for_ad_hoc_workloads` | gauge | `optimize for ad hoc workloads` | Whether plan cache efficiency for single-use ad hoc batches is enabled. |
| `database_observability_sql_server_config_remote_admin_connections` | gauge | `remote admin connections` | Whether remote dedicated administrator connections are enabled. |
| `database_observability_sql_server_config_remote_login_timeout_seconds` | gauge | `remote login timeout (s)` | Timeout for outgoing remote login attempts. |
| `database_observability_sql_server_config_user_connections` | gauge | `user connections` | Configured user connection limit. |

Except for `database_observability_sql_server_config_database_max_degree_of_parallelism`, each metric has a `state` label.
The `configured` state reports the requested value from `sys.configurations.value`.
The `in_use` state reports the running value from `sys.configurations.value_in_use`.
The values can differ when an administrator hasn't run `RECONFIGURE` or when a setting change requires a restart.
SQL Server can also represent automatic defaults differently for `max server memory (MB)` and `min server memory (MB)`.

## `server_metrics` collector

The `server_metrics` collector queries SQL Server when Prometheus scrapes the component.
It has no configuration block; enable or disable it with the `enable_collectors` and `disable_collectors` arguments.
It supports SQL Server 2019 and later, including SQL Server on Amazon RDS, Google Cloud SQL, Azure SQL Database, and Azure SQL Managed Instance.

The collector exports the following metrics:

| Metric | Type | Source | Description |
|--------|------|--------|-------------|
| `database_observability_sql_server_batch_requests_total` | counter | `sys.dm_os_performance_counters` | Total Transact-SQL command batches received by SQL Server. |
| `database_observability_sql_server_database_cpu_utilization_ratio` | gauge | `sys.dm_db_resource_stats` | Average Azure SQL Database CPU utilization relative to the service-tier limit. |
| `database_observability_sql_server_database_data_io_utilization_ratio` | gauge | `sys.dm_db_resource_stats` | Average Azure SQL Database data I/O utilization relative to the service-tier limit. |
| `database_observability_sql_server_database_dtu_limit` | gauge | `sys.dm_user_db_resource_governance` | DTU limit of an Azure SQL Database that uses the DTU purchasing model. |
| `database_observability_sql_server_database_log_write_utilization_ratio` | gauge | `sys.dm_db_resource_stats` | Average Azure SQL Database log-write utilization relative to the service-tier limit. |
| `database_observability_sql_server_database_memory_grant_limit_ratio` | gauge | `sys.dm_user_db_resource_governance` | Maximum memory grant for a single Azure SQL Database request as a ratio of user workload group memory. |
| `database_observability_sql_server_database_memory_utilization_ratio` | gauge | `sys.dm_db_resource_stats` | Average Azure SQL Database memory utilization relative to the service-tier limit. |
| `database_observability_sql_server_database_resource_stats_timestamp_seconds` | gauge | `sys.dm_db_resource_stats` | Unix timestamp of the latest Azure SQL Database resource statistics interval. |
| `database_observability_sql_server_database_resource_governor_max_degree_of_parallelism` | gauge | `sys.dm_user_db_resource_governance` | Maximum degree of parallelism enforced by the Azure SQL Database user workload group. |
| `database_observability_sql_server_database_session_utilization_ratio` | gauge | `sys.dm_db_resource_stats` | Maximum Azure SQL Database session utilization during the latest reporting interval. |
| `database_observability_sql_server_database_storage_allocated_bytes` | gauge | `sys.database_files` | Space allocated to the connected database files, labeled by `file_type`. |
| `database_observability_sql_server_database_storage_limit_bytes` | gauge | `DATABASEPROPERTYEX` | Maximum configured storage capacity of the connected database. |
| `database_observability_sql_server_database_storage_used_bytes` | gauge | `sys.database_files` | Space used within the connected database files, labeled by `file_type`. |
| `database_observability_sql_server_database_vcore_limit` | gauge | `sys.dm_user_db_resource_governance` | vCore limit of an Azure SQL Database that uses the vCore purchasing model. |
| `database_observability_sql_server_database_worker_limit` | gauge | `sys.dm_user_db_resource_governance` | Worker thread limit of the Azure SQL Database user workload group. |
| `database_observability_sql_server_database_worker_utilization_ratio` | gauge | `sys.dm_db_resource_stats` | Maximum Azure SQL Database worker utilization during the latest reporting interval. |
| `database_observability_sql_server_deadlocks_total` | counter | `sys.dm_os_performance_counters` | Total lock requests that resulted in a deadlock. |
| `database_observability_sql_server_host_commit_available_bytes` | gauge | `sys.dm_os_sys_memory` | Available operating system commit visible to the SQL Server host. |
| `database_observability_sql_server_host_commit_limit_bytes` | gauge | `sys.dm_os_sys_memory` | Operating system commit limit visible to the SQL Server host. |
| `database_observability_sql_server_host_high_memory_signal` | gauge | `sys.dm_os_sys_memory` | Whether the host high-memory resource notification is set. |
| `database_observability_sql_server_host_logical_cpu_count` | gauge | `sys.dm_os_sys_info` | Number of logical CPUs visible to the SQL Server host. |
| `database_observability_sql_server_host_low_memory_signal` | gauge | `sys.dm_os_sys_memory` | Whether the host low-memory resource notification is set. |
| `database_observability_sql_server_host_physical_memory_available_bytes` | gauge | `sys.dm_os_sys_memory` | Available physical memory visible to the SQL Server host. |
| `database_observability_sql_server_host_physical_memory_total_bytes` | gauge | `sys.dm_os_sys_memory` | Total physical memory visible to the SQL Server host. |
| `database_observability_sql_server_info` | gauge | `SERVERPROPERTY`, `sys.databases` | SQL Server engine edition and product version metadata, labeled by `engine_edition`, `edition_name`, `product_version`, `product_level`, `product_build`, and `compatibility_level`. |
| `database_observability_sql_server_instance_cpu_utilization_ratio` | gauge | `sys.dm_db_resource_stats`, `master.sys.server_resource_stats` | Average CPU utilization relative to the Azure SQL service-tier limit. |
| `database_observability_sql_server_instance_io_operations_per_second` | gauge | `master.sys.server_resource_stats` | Average physical I/O operations per second during the latest Azure SQL Managed Instance reporting interval. |
| `database_observability_sql_server_instance_io_read_bytes_per_second` | gauge | `master.sys.server_resource_stats` | Average physical bytes read per second during the latest Azure SQL Managed Instance reporting interval. |
| `database_observability_sql_server_instance_io_written_bytes_per_second` | gauge | `master.sys.server_resource_stats` | Average physical bytes written per second during the latest Azure SQL Managed Instance reporting interval. |
| `database_observability_sql_server_instance_memory_utilization_ratio` | gauge | `sys.dm_db_resource_stats` | Average memory utilization of the database engine instance hosting an Azure SQL Database. |
| `database_observability_sql_server_instance_resource_stats_timestamp_seconds` | gauge | `master.sys.server_resource_stats` | Unix timestamp of the latest Azure SQL Managed Instance resource statistics interval. |
| `database_observability_sql_server_instance_storage_allocated_bytes` | gauge | `sys.dm_io_virtual_file_stats` | Total space allocated to database files on the SQL Server instance. |
| `database_observability_sql_server_instance_storage_available_bytes` | gauge | `sys.dm_os_volume_stats` | Available storage capacity on operating system volumes hosting SQL Server files. |
| `database_observability_sql_server_instance_storage_limit_bytes` | gauge | `master.sys.server_resource_stats`, `sys.dm_os_volume_stats` | Reserved storage capacity of an Azure SQL Managed Instance or total operating system volume capacity. |
| `database_observability_sql_server_instance_storage_used_bytes` | gauge | `master.sys.server_resource_stats`, `sys.dm_os_volume_stats` | Storage used by an Azure SQL Managed Instance or across operating system volumes hosting SQL Server files. |
| `database_observability_sql_server_instance_vcore_limit` | gauge | `master.sys.server_resource_stats` | vCore limit of an Azure SQL Managed Instance. |
| `database_observability_sql_server_io_read_bytes_total` | counter | `sys.dm_io_virtual_file_stats` | Total bytes read from SQL Server database and log files. |
| `database_observability_sql_server_io_read_stall_seconds_total` | counter | `sys.dm_io_virtual_file_stats` | Total time spent waiting for read operations on SQL Server files. |
| `database_observability_sql_server_io_reads_total` | counter | `sys.dm_io_virtual_file_stats` | Total physical read operations issued on SQL Server database and log files. |
| `database_observability_sql_server_io_write_stall_seconds_total` | counter | `sys.dm_io_virtual_file_stats` | Total time spent waiting for write operations on SQL Server files. |
| `database_observability_sql_server_io_writes_total` | counter | `sys.dm_io_virtual_file_stats` | Total physical write operations issued on SQL Server database and log files. |
| `database_observability_sql_server_io_written_bytes_total` | counter | `sys.dm_io_virtual_file_stats` | Total bytes written to SQL Server database and log files. |
| `database_observability_sql_server_lock_waits_total` | counter | `sys.dm_os_performance_counters` | Total lock requests that required the caller to wait. |
| `database_observability_sql_server_memory_grants_pending` | gauge | `sys.dm_os_performance_counters` | Current number of processes waiting for a workspace memory grant. |
| `database_observability_sql_server_memory_manager_committed_bytes` | gauge | `sys.dm_os_sys_info` | Memory committed by the SQL Server memory manager. |
| `database_observability_sql_server_memory_manager_target_bytes` | gauge | `sys.dm_os_sys_info` | Memory the SQL Server memory manager can consume. |
| `database_observability_sql_server_page_life_expectancy_seconds` | gauge | `sys.dm_os_performance_counters` | Duration in seconds that a data page stays in the buffer pool without references. |
| `database_observability_sql_server_process_available_commit_memory_bytes` | gauge | `sys.dm_os_process_memory` | Memory available to be committed by the SQL Server process. |
| `database_observability_sql_server_process_committed_memory_resident_ratio` | gauge | `sys.dm_os_process_memory` | Ratio of committed process memory resident in the working set. |
| `database_observability_sql_server_process_cpu_limit` | gauge | `sys.dm_os_job_object` | Logical CPU capacity available to Azure SQL Managed Instance. |
| `database_observability_sql_server_process_cpu_seconds_total` | counter | `sys.dm_os_sys_info` | CPU time consumed by SQL Server threads, labeled by `mode`. |
| `database_observability_sql_server_process_memory_limit_bytes` | gauge | `sys.dm_os_job_object` | Maximum committed memory available to Azure SQL Managed Instance. |
| `database_observability_sql_server_process_page_faults_total` | counter | `sys.dm_os_process_memory` | Page faults incurred by the SQL Server process. |
| `database_observability_sql_server_process_physical_memory_low` | gauge | `sys.dm_os_process_memory` | Whether the process is responding to a low physical-memory notification. |
| `database_observability_sql_server_process_resident_memory_bytes` | gauge | `sys.dm_os_process_memory` | SQL Server process working set. |
| `database_observability_sql_server_process_virtual_memory_bytes` | gauge | `sys.dm_os_process_memory` | Virtual address space committed by the SQL Server process. |
| `database_observability_sql_server_process_virtual_memory_low` | gauge | `sys.dm_os_process_memory` | Whether the process has detected a low virtual-memory condition. |
| `database_observability_sql_server_sos_memory_target_bytes` | gauge | `sys.dm_os_job_object` | Memory available to the Azure SQL Managed Instance SQL operating system. |
| `database_observability_sql_server_sql_compilations_total` | counter | `sys.dm_os_performance_counters` | Total SQL compilations performed by SQL Server. |
| `database_observability_sql_server_sql_recompilations_total` | counter | `sys.dm_os_performance_counters` | Total SQL statement recompilations performed by SQL Server. |
| `database_observability_sql_server_tempdb_space_unallocated_bytes` | gauge | `tempdb.sys.dm_db_file_space_usage` | Unallocated space in `tempdb` data files. |
| `database_observability_sql_server_tempdb_space_used_bytes` | gauge | `tempdb.sys.dm_db_file_space_usage` | Space used in `tempdb` data files, labeled by `category`. |
| `database_observability_sql_server_uptime_seconds` | gauge | `sys.dm_os_sys_info` | Time since the SQL Server process started. |
| `database_observability_sql_server_user_session_limit` | gauge | `sys.configurations` | Maximum user sessions allowed by the active configuration and service limit. |
| `database_observability_sql_server_user_sessions` | gauge | `sys.dm_exec_sessions` | Authenticated user sessions, labeled by `status`. |

The `process_cpu_seconds_total` counter has the `mode` label values `user` and `kernel`.
The `user_sessions` gauge has the `status` label values `running`, `sleeping`, `dormant`, and `preconnect`.
Process-lifetime counters reset when SQL Server restarts or fails over.

The `user_session_limit` metric resolves the default `user connections` setting of `0` to `32767`.
For Azure SQL Managed Instance, it reports the lower of the configured limit and the service limit of `30000`.
Other resources, such as workers and memory, can become saturated before the session limit.
For Azure SQL Database, `user_sessions` is a current count from the connected database.
The separate `database_session_utilization_ratio` metric is the maximum session utilization during the latest 15-second resource statistics interval.
CPU, memory, data I/O, and log-write ratios are averages over that interval.
Worker and session ratios are maxima.
For databases in an elastic pool, these ratios are relative to the per-database limits configured by the pool.

The Azure SQL Database resource metrics require `VIEW DATABASE STATE`.
For Basic, S0, and S1 service objectives and databases in elastic pools, the monitoring login must be a server administrator, a Microsoft Entra administrator, or a member of the `##MS_ServerStateReader##` server role.

The host CPU and memory metrics are available for self-hosted SQL Server, Amazon RDS, and Google Cloud SQL.
They describe the operating system visible to SQL Server and can include managed-service overhead.
Use provider metrics as the authoritative source for cloud service entitlement.

The host metrics aren't emitted for Azure SQL Managed Instance because its host values don't represent the resources allocated to the managed instance.
The collector instead uses `sys.dm_os_job_object` to export the governed CPU and memory limits.
It uses `master.sys.server_resource_stats` for direct CPU, vCore, storage, and I/O metrics.
The I/O metrics are average rates calculated from the totals and actual duration of the latest source interval.
This view is refreshed independently of the Prometheus scrape interval.

The login requires `VIEW SERVER STATE` on SQL Server 2019 and earlier.
On SQL Server 2022 and later, `VIEW SERVER PERFORMANCE STATE` is also sufficient.
Azure SQL Managed Instance requires `VIEW SERVER PERFORMANCE STATE` for process, job-object, and complete session metrics.
Without that permission, `user_sessions` can include only the monitoring login's own session.

## Example

```alloy
database_observability.sql_server "orders_db" {
  data_source_name = "sqlserver://user:pass@server:1433"
  forward_to       = [loki.write.logs_service.receiver]
}

loki.write "logs_service" {
  endpoint {
    url = sys.env("<GRAFANA_CLOUD_HOSTED_LOGS_URL>")
    basic_auth {
      username = sys.env("<GRAFANA_CLOUD_HOSTED_LOGS_ID>")
      password = sys.env("<GRAFANA_CLOUD_RW_API_KEY>")
    }
  }
}
```

Replace the following:

* _`<GRAFANA_CLOUD_HOSTED_LOGS_URL>`_: The URL for your Grafana Cloud hosted logs.
* _`<GRAFANA_CLOUD_HOSTED_LOGS_ID>`_: The user ID for your Grafana Cloud hosted logs.
* _`<GRAFANA_CLOUD_RW_API_KEY>`_: Your Grafana Cloud API key.

[Data Source Name]: https://github.com/microsoft/go-mssqldb#connection-parameters-and-dsn

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`database_observability.sql_server` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`database_observability.sql_server` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

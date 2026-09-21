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
| `exclude_current_user` | `bool`            | Exclude query samples from sessions opened with the login that Alloy uses. | `true` | no       |
| `exclude_schemas`   | `list(string)`       | A list of schemas to exclude from monitoring, on top of the always-excluded system schemas `sys` and `information_schema`. | `["alloydbadmin", "alloydbmetadata", "azure_maintenance", "azure_sys", "cloudsqladmin", "rdsadmin"]` | no       |
| `exclude_databases` | `list(string)`       | A list of databases to exclude from monitoring, on top of the always-excluded system databases `master`, `model`, `msdb`, and `tempdb`. | `["alloydbadmin", "alloydbmetadata", "azure_maintenance", "azure_sys", "cloudsqladmin", "rdsadmin"]` | no       |
| `exclude_users`     | `list(string)`       | A list of original SQL Server login names to exclude from query samples. | `["azuresu", "cloudsqladmin", "db-o11y", "rdsadmin"]` | no       |
| `query_timeout`     | `duration`           | Timeout for each SQL statement.                                          | `"10s"` | no       |

The `query_timeout` applies separately to each SQL statement.
The timeout includes waiting for an available connection and reading the statement results.
A collection cycle can run multiple statements and can take longer than `query_timeout`.

The following collectors are configurable:

| Name              | Description                                                                   | Enabled by default |
|-------------------|-------------------------------------------------------------------------------|--------------------|
| `explain_plans`   | Collect and parse query execution plans already captured by Query Store.      | yes                |
| `query_details`   | Collect query text and parsed table names from Query Store.                   | yes                |
| `query_metrics`   | Collect per-query executions, errors, and duration counters from Query Store. | yes                |
| `query_samples`   | Collect query samples and wait events for tracked queries.                    | yes                |
| `schema_details`  | Collect schemas and tables from `information_schema`.                         | yes                |

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
| [`explain_plans`][explain_plans]     | Configure the query execution plan collector.     | no       |
| [`query_details`][query_details]     | Configure the Query Store query text collector.   | no       |
| [`query_metrics`][query_metrics]     | Configure the Query Store metrics collector.      | no       |
| [`query_samples`][query_samples]     | Configure the query samples collector.            | no       |
| [`schema_details`][schema_details]   | Configure the schema and table details collector. | no       |

[cloud_provider]: #cloud_provider
[aws]: #aws
[azure]: #azure
[gcp]: #gcp
[database_instance]: #database_instance
[explain_plans]: #explain_plans
[query_details]: #query_details
[query_metrics]: #query_metrics
[query_samples]: #query_samples
[schema_details]: #schema_details

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
Each `database_instance` block must also point to a distinct server: two blocks that resolve to the same host, port, and database name are rejected.

| Name               | Type     | Description                                                      | Default | Required |
|--------------------|----------|-------------------------------------------------------------------|---------|----------|
| `data_source_name` | `secret` | [Data Source Name][] for the SQL Server instance to connect to. |         | yes      |

Each `database_instance` block can also contain a [`cloud_provider`][cloud_provider] block that applies to that instance only.

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
  }

  database_instance "billing" {
    data_source_name = sys.env("BILLING_DSN")
  }
}
```

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

The `query_samples` collector only collects requests whose query hash is tracked by the `query_metrics` collector.

The collector polls live requests and can miss queries shorter than `collect_interval`, wait events that start and finish between collections, and a query's first execution before Query Store admits its hash.
For completed requests, the emitted resource counters contain the values from the final observation and can omit work performed after that observation.

The login requires `VIEW SERVER STATE` on SQL Server 2019 and earlier. On SQL Server 2022 and later, `VIEW SERVER PERFORMANCE STATE` is also sufficient. Azure SQL Database can restrict the dynamic management views to the current session. In that case, the collector can't observe other sessions.

### `schema_details`

| Name               | Type       | Description                                          | Default | Required |
|--------------------|------------|------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect information from database. | `"1m"`  | no       |

The collector scans every database that the login can access on the instance and collects schema details from each. Only databases where the login has `CONNECT` access to catalog views are collected.

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

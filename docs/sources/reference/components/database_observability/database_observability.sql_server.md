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
| `data_source_name`  | `secret`             | [Data Source Name][] for the SQL Server instance to connect to.          |         | yes      |
| `forward_to`        | `list(LogsReceiver)` | Where to forward log entries after processing.                           |         | yes      |
| `targets`           | `list(map(string))`  | List of external targets to scrape.                                      |         | no       |
| `disable_collectors`| `list(string)`       | A list of collectors to disable from the default set.                    |         | no       |
| `enable_collectors` | `list(string)`       | A list of collectors to enable on top of the default set.                |         | no       |
| `exclude_schemas`   | `list(string)`       | A list of schemas to exclude from monitoring, on top of the always-excluded system schemas `sys` and `information_schema`. | `["alloydbadmin", "alloydbmetadata", "azure_maintenance", "azure_sys", "cloudsqladmin", "rdsadmin"]` | no       |
| `exclude_databases` | `list(string)`       | A list of databases to exclude from monitoring, on top of the always-excluded system databases `master`, `model`, `msdb`, and `tempdb`. | `["alloydbadmin", "alloydbmetadata", "azure_maintenance", "azure_sys", "cloudsqladmin", "rdsadmin"]` | no       |

The following collectors are configurable:

| Name              | Description                                                  | Enabled by default |
|-------------------|--------------------------------------------------------------|--------------------|
| `schema_details`  | Collect schemas and tables from `information_schema`. | yes                |

## Blocks

You can use the following blocks with `database_observability.sql_server`:

{{< docs/alloy-config >}}

| Block                                | Description                                       | Required |
|--------------------------------------|---------------------------------------------------|----------|
| [`cloud_provider`][cloud_provider]   | Provide Cloud Provider information.               | no       |
| `cloud_provider` > [`aws`][aws]      | Provide AWS database host information.            | no       |
| `cloud_provider` > [`azure`][azure]  | Provide Azure database host information.          | no       |
| `cloud_provider` > [`gcp`][gcp]      | Provide GCP database host information.            | no       |
| [`schema_details`][schema_details]   | Configure the schema and table details collector. | no       |

[cloud_provider]: #cloud_provider
[aws]: #aws
[azure]: #azure
[gcp]: #gcp
[schema_details]: #schema_details

{{< /docs/alloy-config >}}

### `cloud_provider`

The `cloud_provider` block has no attributes.
It contains zero or one of the [`aws`][aws], [`azure`][azure], or [`gcp`][gcp] blocks.
You use the `cloud_provider` block to provide information related to the cloud provider that hosts the database under observation.
This information is appended as labels to the collected metrics.
The labels make it easier for you to filter and group your metrics.

When you don't configure a `cloud_provider` block, {{< param "PRODUCT_NAME" >}} attempts to detect AWS RDS and Azure SQL hosts from the `data_source_name`.

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

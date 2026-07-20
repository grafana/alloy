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

| Block                              | Description                                       | Required |
|------------------------------------|---------------------------------------------------|----------|
| [`schema_details`][schema_details] | Configure the schema and table details collector. | no       |

[schema_details]: #schema_details

{{< /docs/alloy-config >}}

### `schema_details`

| Name               | Type       | Description                                          | Default | Required |
|--------------------|------------|------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect information from database. | `"1m"`  | no       |

The collector scans every database that the login can access on the instance and collects schema details from each. Only databases where the login has `CONNECT` access to catalog views are collected.

{{< admonition type="note" >}}
On Azure SQL Database, set the `data_source_name` to connect to the `master` database, for example with `database=master`, to collect more than one database.
The connecting login needs a user in each database you want to monitor, with `VIEW DEFINITION` granted so the component can read catalog metadata.
If the connection targets a single user database, the component collects only that database.
{{< /admonition >}}

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

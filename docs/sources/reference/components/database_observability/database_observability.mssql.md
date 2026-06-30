---
canonical: https://grafana.com/docs/alloy/latest/reference/components/database_observability.mssql/
description: Learn about database_observability.mssql
title: database_observability.mssql
labels:
  stage: experimental
  products:
    - oss
---

# `database_observability.mssql`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`database_observability.mssql` connects to a Microsoft SQL Server database and collects observability data.
The component forwards this data as log entries to Loki receivers and exports targets for Prometheus scraping.

## Usage

```alloy
database_observability.mssql "<LABEL>" {
  data_source_name = <DATA_SOURCE_NAME>
  forward_to       = [<LOKI_RECEIVERS>]
}
```

## Arguments

You can use the following arguments with `database_observability.mssql`:

| Name                | Type                 | Description                                                              | Default | Required |
|---------------------|----------------------|--------------------------------------------------------------------------|---------|----------|
| `data_source_name`  | `secret`             | [Data Source Name][] for the SQL Server instance to connect to.          |         | yes      |
| `forward_to`        | `list(LogsReceiver)` | Where to forward log entries after processing.                           |         | yes      |
| `targets`           | `list(map(string))`  | List of external targets to scrape.                                      |         | no       |
| `disable_collectors`| `list(string)`       | A list of collectors to disable from the default set.                    |         | no       |
| `enable_collectors` | `list(string)`       | A list of collectors to enable on top of the default set.                |         | no       |
| `exclude_schemas`   | `list(string)`       | A list of schemas to exclude from monitoring, on top of the always-excluded system schemas `sys` and `information_schema`. | `["alloydbadmin", "alloydbmetadata", "azure_maintenance", "azure_sys", "cloudsqladmin", "rdsadmin"]` | no       |

The following collectors are configurable:

| Name              | Description                                                  | Enabled by default |
|-------------------|--------------------------------------------------------------|--------------------|
| `schema_details`  | Collect schemas and tables from `information_schema`. | yes                |

## Blocks

You can use the following blocks with `database_observability.mssql`:

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

## Example

```alloy
database_observability.mssql "orders_db" {
  data_source_name = "sqlserver://user:pass@mssql:1433"
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

`database_observability.mssql` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`database_observability.mssql` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

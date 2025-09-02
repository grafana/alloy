---
canonical: https://grafana.com/docs/alloy/latest/reference/components/database_observability.mysql/
description: Learn about database_observability.mysql
title: database_observability.mysql
labels:
  stage: experimental
  products:
    - oss
---

# `database_observability.mysql`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Usage

```alloy
database_observability.mysql "<LABEL>" {
  data_source_name = <DATA_SOURCE_NAME>
  forward_to       = [<LOKI_RECEIVERS>]
}
```

## Arguments

You can use the following arguments with `database_observability.mysql`:

| Name                                       | Type                 | Description                                                                 | Default | Required |
|--------------------------------------------|----------------------|-----------------------------------------------------------------------------|---------|----------|
| `data_source_name`                         | `secret`             | [Data Source Name][] for the MySQL server to connect to.                    |         | yes      |
| `forward_to`                               | `list(LogsReceiver)` | Where to forward log entries after processing.                              |         | yes      |
| `targets`                                  | `list(map(string))`  | List of targets to scrape.                                                  |         | yes      |
| `disable_collectors`                       | `list(string)`       | A list of collectors to disable from the default set.                       |         | no       |
| `enable_collectors`                        | `list(string)`       | A list of collectors to enable on top of the default set.                   |         | no       |
| `allow_update_performance_schema_settings` | `boolean`            | Whether to allow updates to `performance_schema` settings in any collector. | `false` | no       |

The following collectors are configurable:

| Name              | Description                                              | Enabled by default |
|-------------------|----------------------------------------------------------|--------------------|
| `query_details`   | Collect queries information.                             | yes                |
| `schema_details`  | Collect schemas and tables from `information_schema`.    | yes                |
| `query_samples`   | Collect query samples.                                   | yes                |
| `setup_consumers` | Collect enabled `performance_schema.setup_consumers`.    | yes                |
| `locks`           | Collect queries that are waiting/blocking other queries. | no                 |
| `explain_plans`   | Collect explain plans information.                       | no                 |

## Blocks

You can use the following blocks with `database_observability.mysql`:

| Block                                | Description                                       | Required |
|--------------------------------------|---------------------------------------------------|----------|
| [`cloud_provider`][cloud_provider]   | Provide Cloud Provider information.               | no       |
| `cloud_provider` > [`aws`][aws]      | Provide AWS database host information.            | no       |
| [`setup_consumers`][setup_consumers] | Configure the `setup_consumers` collector.        | no       |
| [`query_details`][query_details]     | Configure the queries collector.                  | no       |
| [`schema_details`][schema_details]   | Configure the schema and table details collector. | no       |
| [`explain_plans`][explain_plans]     | Configure the explain plans collector.            | no       |
| [`locks`][locks]                     | Configure the locks collector.                    | no       |
| [`query_samples`][query_samples]     | Configure the query samples collector.            | no       |

The > symbol indicates deeper levels of nesting.
For example, `cloud_provider` > `aws` refers to a `aws` block defined inside an `cloud_provider` block.

[cloud_provider]: #cloud_provider
[aws]: #aws
[setup_consumers]: #setup_consumers
[query_details]: #query_details
[schema_details]: #schema_details
[explain_plans]: #explain_plans
[locks]: #locks
[query_samples]: #query_samples

### `cloud_provider`

The `cloud_provider` block has no attributes.
It contains zero or more [`aws`][aws] blocks.
You use the `cloud_provider` block to provide information related to the cloud provider that hosts the database under observation.
This information is appended as labels to the collected metrics.
The labels make it easier for you to filter and group your metrics.

### `aws`

The `aws` block supplies the [ARN](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html) identifier for the database being monitored.

| Name  | Type     | Description                                             | Default | Required |
|-------|----------|---------------------------------------------------------|---------|----------|
| `arn` | `string` | The ARN associated with the database under observation. |         | yes      |

### `setup_consumers`

| Name               | Type       | Description                                                                                   | Default | Required |
|--------------------|------------|-----------------------------------------------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect `performance_schema.setup_consumers` information from the database. | `"1h"`  | no       |

### `query_details`

| Name               | Type       | Description                                          | Default | Required |
|--------------------|------------|------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect information from database. | `"1m"`  | no       |

### `schema_details`

| Name               | Type       | Description                                                           | Default | Required |
|--------------------|------------|-----------------------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect information from database.                  | `"1m"`  | no       |
| `cache_enabled`    | `boolean`  | Whether to enable caching of table definitions.                       | `true`  | no       |
| `cache_size`       | `integer`  | Cache size.                                                           | `256`   | no       |
| `cache_ttl`        | `duration` | Cache TTL.                                                            | `"10m"` | no       |

### `explain_plans`

| Name                           | Type           | Description                                                                     | Default | Required |
| ------------------------------ | -------------- | ------------------------------------------------------------------------------- | ------- | -------- |
| `collect_interval`             | `duration`     | How frequently to collect information from database.                            | `"1m"`  | no       |
| `explain_plan_exclude_schemas` | `list(string)` | List of schemas to exclude from explain plan collection.                        |         | no       |
| `initial_lookback`             | `duration`     | How far back to look for explain plan queries on the first collection interval. | `"24h"` | no       |
| `per_collect_ratio`            | `float`        | Ratio of explain plan queries to collect per collect interval.                  | `1.0`   | no       |

### `locks`

| Name               | Type       | Description                                                                            | Default | Required |
| ------------------ | ---------- | -------------------------------------------------------------------------------------- | ------- | -------- |
| `collect_interval` | `duration` | How frequently to collect information from database.                                   | `"1m"`  | no       |
| `threshold`        | `duration` | Threshold for locks to be considered slow. Locks that exceed this duration are logged. | `"1s"`  | no       |

### `query_samples`

| Name                             | Type       | Description                                                                    | Default | Required |
|----------------------------------|------------|--------------------------------------------------------------------------------|---------|----------|
| `collect_interval`               | `duration` | How frequently to collect information from database.                           | `"1m"`  | no       |
| `disable_query_redaction`        | `bool`     | Collect unredacted SQL query text including parameters.                        | `false` | no       |
| `auto_enable_setup_consumers`    | `boolean`  | Whether to enable some specific `performance_schema.setup_consumers` settings. | `false` | no       |
| `setup_consumers_check_interval` | `duration` | How frequently to check if `setup_consumers` are correctly enabled.            | `"1h"`  | no       |

## Example

```alloy
database_observability.mysql "orders_db" {
  data_source_name = "user:pass@tcp(mysql:3306)/"
  forward_to       = [loki.write.logs_service.receiver]

  enable_collectors = ["query_samples"]

  cloud_provider {
    aws {
      arn = "your-rds-db-arn"
    }
  }
}

prometheus.scrape "orders_db" {
  targets = database_observability.mysql.orders_db.targets
  honor_labels = true // required to keep job and instance labels
  forward_to = [prometheus.remote_write.metrics_service.receiver]
}

prometheus.remote_write "metrics_service" {
  endpoint {
    url = sys.env("<GRAFANA_CLOUD_HOSTED_METRICS_URL>")
    basic_auth {
      username = sys.env("<GRAFANA_CLOUD_HOSTED_METRICS_ID>")
      password = sys.env("<GRAFANA_CLOUD_RW_API_KEY>")
    }
  }
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

* _`<GRAFANA_CLOUD_HOSTED_METRICS_URL>`_: The URL for your Grafana Cloud hosted metrics.
* _`<GRAFANA_CLOUD_HOSTED_METRICS_ID>`_: The user ID for your Grafana Cloud hosted metrics.
* _`<GRAFANA_CLOUD_RW_API_KEY>`_: Your Grafana Cloud API key.
* _`<GRAFANA_CLOUD_HOSTED_LOGS_URL>`_: The URL for your Grafana Cloud hosted logs.
* _`<GRAFANA_CLOUD_HOSTED_LOGS_ID>`_: The user ID for your Grafana Cloud hosted logs.

[Data Source Name]: https://github.com/go-sql-driver/mysql#dsn-data-source-name

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`database_observability.mysql` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`database_observability.mysql` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

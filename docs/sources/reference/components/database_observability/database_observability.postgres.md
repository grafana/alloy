---
canonical: https://grafana.com/docs/alloy/latest/reference/components/database_observability.postgres/
description: Learn about database_observability.postgres
title: database_observability.postgres
labels:
  stage: public-preview
  products:
    - oss
---

# `database_observability.postgres`

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Usage

```alloy
database_observability.postgres "<LABEL>" {
  data_source_name = <DATA_SOURCE_NAME>
  forward_to       = [<LOKI_RECEIVERS>]
  targets          = "<TARGET_LIST>"
}
```

## Arguments

You can use the following arguments with `database_observability.postgres`:

| Name                 | Type                 | Description                                                 | Default | Required |
|----------------------|----------------------|-------------------------------------------------------------|---------|----------|
| `data_source_name`   | `secret`             | [Data Source Name][] for the Postgres server to connect to. |         | yes      |
| `forward_to`         | `list(LogsReceiver)` | Where to forward log entries after processing.              |         | yes      |
| `targets`            | `list(map(string))`  | List of targets to scrape.                                  |         | yes      |
| `disable_collectors` | `list(string)`       | A list of collectors to disable from the default set.       |         | no       |
| `enable_collectors`  | `list(string)`       | A list of collectors to enable on top of the default set.   |         | no       |
| `exclude_databases`  | `list(string)`       | A list of databases to exclude from monitoring.             |         | no       |

The following collectors are configurable:

| Name             | Description                                                           | Enabled by default |
|------------------|-----------------------------------------------------------------------|--------------------|
| `query_details`  | Collect queries information.                                          | yes                |
| `query_samples`  | Collect query samples and wait events information.                    | yes                |
| `schema_details` | Collect schemas, tables, and columns from PostgreSQL system catalogs. | yes                |
| `explain_plans`  | Collect query explain plans.                                          | yes                |

## Blocks

You can use the following blocks with `database_observability.postgres`:

| Block                              | Description                                       | Required |
|------------------------------------|---------------------------------------------------|----------|
| [`cloud_provider`][cloud_provider] | Provide Cloud Provider information.               | no       |
| `cloud_provider` > [`aws`][aws]    | Provide AWS database host information.            | no       |
| `cloud_provider` > [`azure`][azure]  | Provide Azure database host information.          | no       |
| [`query_details`][query_details]   | Configure the queries collector.                  | no       |
| [`query_samples`][query_samples]   | Configure the query samples collector.            | no       |
| [`schema_details`][schema_details] | Configure the schema and table details collector. | no       |
| [`explain_plans`][explain_plans]   | Configure the explain plans collector.            | no       |
| [`health_check`][health_check]     | Configure the health check collector.             | no       |

The > symbol indicates deeper levels of nesting.
For example, `cloud_provider` > `aws` refers to a `aws` block defined inside an `cloud_provider` block.

[cloud_provider]: #cloud_provider
[aws]: #aws
[azure]: #azure
[query_details]: #query_details
[query_samples]: #query_samples
[schema_details]: #schema_details
[explain_plans]: #explain_plans
[health_check]: #health_check

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

### `azure`

The `azure` block supplies the identifying information for the database being monitored.

| Name              | Type     | Description                                          | Default | Required |
|-------------------|----------|------------------------------------------------------|---------|----------|
| `subscription_id` | `string` | The Subscription ID for your Azure account.          |         | yes      |
| `resource_group`  | `string` | The Resource Group that holds the database resource. |         | yes      |
| `server_name`     | `string` | The database server name.                            |         | no       |

### `query_details`

| Name               | Type       | Description                                          | Default | Required |
|--------------------|------------|------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect information from database. | `"1m"`  | no       |

### `query_samples`

| Name                      | Type       | Description                                                   | Default | Required |
|---------------------------|------------|---------------------------------------------------------------|---------|----------|
| `collect_interval`        | `duration` | How frequently to collect information from database.          | `"15s"` | no       |
| `disable_query_redaction` | `bool`     | Collect unredacted SQL query text (might include parameters). | `false` | no       |
| `exclude_current_user`    | `bool`     | Do not collect query samples for current database user.       | `true`  | no       |

### `schema_details`

| Name               | Type       | Description                                                           | Default | Required |
|--------------------|------------|-----------------------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect information from database.                  | `"1m"`  | no       |
| `cache_enabled`    | `boolean`  | Whether to enable caching of table definitions.                       | `true`  | no       |
| `cache_size`       | `integer`  | Cache size.                                                           | `256`   | no       |
| `cache_ttl`        | `duration` | Cache TTL.                                                            | `"10m"` | no       |


### `explain_plans`

| Name                | Type           | Description                                          | Default | Required |
|---------------------|----------------|------------------------------------------------------|---------|----------|
| `collect_interval`  | `duration`     | How frequently to collect information from database. | `"1m"`  | no       |
| `per_collect_ratio` | `float64`      | The ratio of queries to collect explain plans for.   | `1.0`   | no       |

### `health_check`

| Name               | Type       | Description                                          | Default | Required |
|--------------------|------------|------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect information from database. | `"1h"`  | no       |

## Example

```alloy
database_observability.postgres "orders_db" {
  data_source_name = "postgres://user:pass@localhost:5432/dbname"
  forward_to       = [loki.relabel.orders_db.receiver]
  targets          = prometheus.exporter.postgres.orders_db.targets

  enable_collectors = ["query_samples", "explain_plans"]

  cloud_provider {
    aws {
      arn = "your-rds-db-arn"
    }
  }
}

prometheus.exporter.postgres "orders_db" {
  data_source_name   = "postgres://user:pass@localhost:5432/dbname"
  enabled_collectors = ["stat_statements"]
}

loki.relabel "orders_db" {
  forward_to = [loki.write.logs_service.receiver]
  rule {
    target_label = "job"
    replacement  = "integrations/db-o11y"
  }
  rule {
    target_label = "instance"
    replacement  = "orders_db"
  }
}

discovery.relabel "orders_db" {
  targets = database_observability.postgres.orders_db.targets

  rule {
    target_label = "job"
    replacement  = "integrations/db-o11y"
  }
  rule {
    target_label = "instance"
    replacement  = "orders_db"
  }
}

prometheus.scrape "orders_db" {
  targets    = discovery.relabel.orders_db.targets
  job_name   = "integrations/db-o11y"
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

[Data Source Name]: https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`database_observability.postgres` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`database_observability.postgres` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

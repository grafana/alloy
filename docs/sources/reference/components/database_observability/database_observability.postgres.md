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

## Exports

The following fields are exported and can be referenced by other components:

| Name                  | Type            | Description                                                                      |
|-----------------------|-----------------|----------------------------------------------------------------------------------|
| `targets`             | `list(map(string))` | Targets that can be used to collect metrics from the component.               |
| `error_logs_receiver` | `LogsReceiver`  | Receiver for PostgreSQL error logs that processes and exports metrics.           |

The following collectors are configurable:

| Name             | Description                                                           | Enabled by default |
|------------------|-----------------------------------------------------------------------|--------------------|
| `query_details`  | Collect queries information.                                          | yes                |
| `query_samples`  | Collect query samples and wait events information.                    | yes                |
| `schema_details` | Collect schemas, tables, and columns from PostgreSQL system catalogs. | yes                |
| `explain_plans`  | Collect query explain plans.                                          | yes                |
| `error_logs`     | Process PostgreSQL error logs and export metrics (always enabled).    | yes                |

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

## Error Logs Collector

The `error_logs` collector processes PostgreSQL error logs received through the `error_logs_receiver` entry point and exports Prometheus metrics.
Unlike other collectors, it **runs independently of the database connection** and starts immediately when the component is created.

### Key Features

- **Always-on processing**: Processes logs even when the database is unavailable
- **Entry point receiver**: Provides an `error_logs_receiver` that must be fed by log sources (e.g., `loki.source.file`, `loki.source.cloudwatch`)
- **RDS log format support**: Parses structured PostgreSQL logs using AWS RDS format
- **SQLSTATE extraction**: Automatically extracts and classifies errors by SQLSTATE codes
- **Prometheus metrics**: Exports detailed error metrics with labels for severity, SQLSTATE, database, user, and instance
- **Format validation**: Validates log format and provides warnings for misconfigured log output

### Exported Receiver

The component exports an `error_logs_receiver` entry point that must be fed by log source components.
The receiver does not collect logs itself - it processes logs forwarded to it:

- `loki.source.file` - reads PostgreSQL log files and forwards to the receiver
- `loki.source.cloudwatch` - reads CloudWatch Logs (RDS) and forwards to the receiver
- `otelcol.receiver.otlp` + `otelcol.exporter.loki` - receives OTLP logs and forwards to the receiver exporting to loki format

### Metrics

The error_logs collector exports the following Prometheus metrics:

| Metric Name                                  | Type    | Description                                          | Labels                                                             |
|----------------------------------------------|---------|------------------------------------------------------|--------------------------------------------------------------------|
| `postgres_errors_total`                      | counter | Total PostgreSQL errors by severity and SQLSTATE     | `severity`, `sqlstate`, `sqlstate_class`, `database`, `user`, `instance`, `server_id` |
| `postgres_error_log_parse_failures_total`    | counter | Number of log lines that failed to parse             | -                                                                  |

### Required PostgreSQL Configuration

For the error_logs collector to work correctly, PostgreSQL must be configured with the RDS log format:

```sql
-- Set log format (requires superuser or rds_superuser)
ALTER SYSTEM SET log_line_prefix = '%m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a';

-- Reload configuration
SELECT pg_reload_conf();
```

```sql
SHOW log_line_prefix;
```

### Supported Log Format

The collector expects PostgreSQL logs in the RDS format with these fields:

```
<timestamp>:<remote_host:port>:<user>@<database>:[<pid>]:<line>:<SQLSTATE>:<session_start>:<vtxid>:<txid>:<session_id>:<query><app><severity>: <message>
```

Example log line:
```
2026-02-02 21:35:40.130 UTC:10.24.155.141(34110):app_user@books_store:[32032]:2:40001:2026-02-02 21:33:19 UTC:25/112:0:693c34cb.2398::psqlERROR:  canceling statement due to user request
```

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

// Read PostgreSQL log files and forward to error_logs collector
loki.source.file "postgres_logs" {
  targets = [{
    __path__ = "/var/log/postgresql/postgresql-*.log",
    job      = "postgres-errors",
  }]
  
  forward_to = [database_observability.postgres.orders_db.error_logs_receiver]
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
- Components that consume [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

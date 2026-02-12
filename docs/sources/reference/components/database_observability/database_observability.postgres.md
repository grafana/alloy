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

| Name             | Type                | Description                                                                      |
|------------------|---------------------|----------------------------------------------------------------------------------|
| `targets`        | `list(map(string))` | Targets that can be used to collect metrics from the component.                  |
| `logs_receiver`  | `LogsReceiver`      | Receiver for PostgreSQL logs that processes and exports error metrics.           |

The following collectors are configurable:

| Name             | Description                                                           | Enabled by default |
|------------------|-----------------------------------------------------------------------|--------------------|
| `query_details`  | Collect queries information.                                          | yes                |
| `query_samples`  | Collect query samples and wait events information.                    | yes                |
| `schema_details` | Collect schemas, tables, and columns from PostgreSQL system catalogs. | yes                |
| `explain_plans`  | Collect query explain plans.                                          | yes                |
| `logs`           | Process PostgreSQL logs and export error metrics.                     | yes                |

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

## Logs Collector

The `logs` collector processes PostgreSQL logs received through the `logs_receiver` entry point and exports Prometheus metrics for query and server errors.

### Exported Receiver

The component exports a `logs_receiver` entry point that must be fed by log source components.

- `otelcol.receiver.awscloudwatch` + `otelcol.exporter.loki` - reads CloudWatch Logs (RDS) and forwards to the receiver
- `loki.source.file` - reads PostgreSQL log files and forwards to the receiver
- `otelcol.receiver.otlp` + `otelcol.exporter.loki` - receives OTLP logs and forwards to the receiver

### Metrics

The logs collector exports the following Prometheus metrics:

| Metric Name                               | Type    | Description                                       | Labels                                                                                |
| ----------------------------------------- | ------- | ------------------------------------------------- | ------------------------------------------------------------------------------------- |
| `postgres_errors_total`                   | counter | Total PostgreSQL errors by severity and SQLSTATE. | `severity`, `sqlstate`, `sqlstate_class`, `database`, `user`, `instance`, `server_id` |
| `postgres_error_log_parse_failures_total` | counter | Number of log lines that failed to parse.         | -                                                                                     |

### Required PostgreSQL Configuration

For the logs collector to work correctly, PostgreSQL must be configured with the following RDS log format:

```sql
-- Set log format (requires superuser or rds_superuser)
ALTER SYSTEM SET log_line_prefix = '%m:%r:%u@%d:[%p]:%l:%e:%s:%v:%x:%c:%q%a';

-- Reload configuration
SELECT pg_reload_conf();
```

Use the following SELECT statement to show and verify the current string format applied to the beginning of each log line.

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

### Watermark and Historical Log Processing

The `logs` collector uses a watermark file to track the last processed log timestamp. This prevents re-counting historical logs on component restart and maintains proper Prometheus counter semantics.

**Default behavior:**
- Watermark file: `<data_path>/dbo11y_pg_logs_watermark.txt`
- On first run: Starts processing logs from the current time (skips historical logs)
- On restart: Resumes from the last processed timestamp
- Sync frequency: Every 10 seconds (atomic writes for crash safety)

**Benefits:**
- `postgres_errors_total` maintains monotonically increasing values
- No duplicate counting on Alloy restarts
- Proper `rate()` and `increase()` calculations in Prometheus
- Resumes from last position after downtime (no data loss)

**Important:** When using log sources with `start_from` set to historical timestamps (e.g., `otelcol.receiver.awscloudwatch` with `start_from = "2026-01-01T00:00:00Z"`), the watermark ensures that historical logs are only counted once, even if Alloy restarts multiple times.

## Examples

### With loki.source.file

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

// Read PostgreSQL log files and forward to logs collector
loki.source.file "postgres_logs" {
  targets = [{
    __path__ = "/var/log/postgresql/postgresql-*.log",
    job      = "postgres-logs",
  }]
  
  forward_to = [database_observability.postgres.orders_db.logs_receiver]
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

### With otelcol.receiver.awscloudwatch

```alloy
// Storage for CloudWatch state persistence
otelcol.storage.file "cloudwatch" {
  directory = "/var/lib/alloy/storage"
}

// Fetch logs from CloudWatch
otelcol.receiver.awscloudwatch "rds_logs" {
  region  = "us-east-1"
  storage = otelcol.storage.file.cloudwatch.handler
  
  logs {
    poll_interval = "1m"
    // Omit start_from for tail-only (recommended)
    // start_from = "2026-02-01T00:00:00Z"  // For historical backfill
    
    groups {
      named {
        group_name = "/aws/rds/instance/production-db/postgresql"
      }
    }
  }
  
  output {
    logs = [otelcol.exporter.loki.rds_logs.input]
  }
}

// Convert OTLP to Loki format
otelcol.exporter.loki "rds_logs" {
  forward_to = [database_observability.postgres.rds_db.logs_receiver]
}

// Database observability component
database_observability.postgres "rds_db" {
  data_source_name = "postgres://user:pass@rds-endpoint.region.rds.amazonaws.com:5432/postgres"
  targets          = prometheus.exporter.postgres.rds_db.targets
  forward_to       = [loki.relabel.rds_db.receiver]
  
  cloud_provider {
    aws {
      arn = "arn:aws:rds:us-east-1:123456789012:db:production-db"
    }
  }
}

prometheus.exporter.postgres "rds_db" {
  data_source_name   = "postgres://user:pass@rds-endpoint.region.rds.amazonaws.com:5432/postgres"
  enabled_collectors = ["stat_statements"]
}

loki.relabel "rds_db" {
  forward_to = [loki.write.logs_service.receiver]
  rule {
    target_label = "job"
    replacement  = "integrations/db-o11y"
  }
  rule {
    target_label = "instance"
    replacement  = "rds_db"
  }
}

discovery.relabel "rds_db" {
  targets = database_observability.postgres.rds_db.targets
  
  rule {
    target_label = "job"
    replacement  = "integrations/db-o11y"
  }
  rule {
    target_label = "instance"
    replacement  = "rds_db"
  }
}

prometheus.scrape "rds_db" {
  targets    = discovery.relabel.rds_db.targets
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

**AWS Credentials:** The `otelcol.receiver.awscloudwatch` component requires AWS credentials. Configure via:
- Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`
- Docker: Mount `~/.aws` credentials or pass environment variables
- Kubernetes: Use [IAM Roles for Service Accounts (IRSA)](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) or Kubernetes secrets

**Required IAM permissions:**
```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "logs:FilterLogEvents",
      "logs:GetLogEvents",
      "logs:DescribeLogGroups",
      "logs:DescribeLogStreams"
    ],
    "Resource": "arn:aws:logs:*:*:log-group:/aws/rds/instance/*"
  }]
}
```

**Persistent storage:** Both `otelcol.storage.file` directory and Alloy's data path (`--storage.path`) must be persisted across restarts to maintain CloudWatch state and watermarks.

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

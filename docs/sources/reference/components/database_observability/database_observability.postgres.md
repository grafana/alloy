---
canonical: https://grafana.com/docs/alloy/latest/reference/components/database_observability.postgres/
description: Learn about database_observability.postgres
title: database_observability.postgres
labels:
  stage: general-availability
  products:
    - oss
---

# `database_observability.postgres`

`database_observability.postgres` connects to a PostgreSQL database and collects observability data from system catalogs and the `pg_stat_statements` extension.
The component collects query details, schema information, explain plans, query samples, and processes PostgreSQL logs.
It forwards this data as log entries to Loki receivers and exports targets for Prometheus scraping.

## Usage

```alloy
database_observability.postgres "<LABEL>" {
  data_source_name = <DATA_SOURCE_NAME>
  forward_to       = [<LOKI_RECEIVERS>]
}
```

## Arguments

You can use the following arguments with `database_observability.postgres`:

| Name                 | Type                 | Description                                                 | Default | Required |
|----------------------|----------------------|-------------------------------------------------------------|---------|----------|
| `data_source_name`   | `secret`             | [Data Source Name][] for the Postgres server to connect to. |         | yes      |
| `forward_to`         | `list(LogsReceiver)` | Where to forward log entries after processing.              |         | yes      |
| `targets`            | `list(map(string))`  | List of external targets to scrape for Prometheus metrics.  |         | no       |
| `disable_collectors` | `list(string)`       | A list of collectors to disable from the default set.       |         | no       |
| `enable_collectors`  | `list(string)`       | A list of collectors to enable on top of the default set.   |         | no       |
| `exclude_databases`  | `list(string)`       | A list of databases to exclude from monitoring.             | `["alloydbadmin", "alloydbmetadata", "azure_maintenance", "azure_sys", "cloudsqladmin", "rdsadmin"]` | no       |
| `exclude_users`      | `list(string)`       | A list of users to exclude from monitoring.                 | `["azuresu", "cloudsqladmin", "db-o11y", "rdsadmin"]` | no       |

[Data Source Name]: https://pkg.go.dev/github.com/lib/pq#hdr-URL_connection_strings-NewConfig

Refer to the [PostgreSQL documentation](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING) for more information about the format of the connection strings in `data_source_name`.

## Exports

The following fields are exported and can be referenced by other components:

| Name            | Type                | Description                                                            |
| --------------- | ------------------- | ---------------------------------------------------------------------- |
| `logs_receiver` | `LogsReceiver`      | Receiver for PostgreSQL logs that processes and exports error metrics. |
| `targets`       | `list(map(string))` | Targets that can be used to collect metrics from the component.        |

The following collectors are configurable:

| Name             | Description                                                           | Enabled by default |
|------------------|-----------------------------------------------------------------------|--------------------|
| `explain_plans`  | Collect query explain plans.                                          | yes                |
| `logs`           | Process PostgreSQL logs and export error metrics.                     | yes                |
| `query_details`  | Collect queries information.                                          | yes                |
| `query_samples`  | Collect query samples and wait events information.                    | yes                |
| `schema_details` | Collect schemas, tables, and columns from PostgreSQL system catalogs. | yes                |

## Blocks

You can use the following blocks with `database_observability.postgres`:

{{< docs/alloy-config >}}

| Block                              | Description                                       | Required |
|------------------------------------|---------------------------------------------------|----------|
| [`cloud_provider`][cloud_provider]   | Provide Cloud Provider information.               | no       |
| `cloud_provider` > [`aws`][aws]      | Provide AWS database host information.            | no       |
| `cloud_provider` > [`azure`][azure]  | Provide Azure database host information.          | no       |
| `cloud_provider` > [`gcp`][gcp]      | Provide GCP database host information.            | no       |
| [`query_details`][query_details]   | Configure the queries collector.                  | no       |
| [`query_samples`][query_samples]   | Configure the query samples collector.            | no       |
| [`schema_details`][schema_details] | Configure the schema and table details collector. | no       |
| [`explain_plans`][explain_plans]   | Configure the explain plans collector.            | no       |
| [`health_check`][health_check]               | Configure the health check collector.   | no       |
| [`prometheus_exporter`][prometheus_exporter] | Configure the embedded postgres_exporter. | no       |

[cloud_provider]: #cloud_provider
[aws]: #aws
[azure]: #azure
[gcp]: #gcp
[query_details]: #query_details
[query_samples]: #query_samples
[schema_details]: #schema_details
[explain_plans]: #explain_plans
[health_check]: #health_check
[prometheus_exporter]: #prometheus_exporter

{{< /docs/alloy-config >}}

### `cloud_provider`

The `cloud_provider` block has no attributes.
It contains zero or more [`aws`][aws], [`azure`][azure], or [`gcp`][gcp] blocks.
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

### `gcp`

The `gcp` block supplies the identifying information for the GCP Cloud SQL database being monitored.

| Name              | Type     | Description                                                                                                                 | Default | Required |
|-------------------|----------|-----------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `connection_name` | `string` | The Cloud SQL instance connection name in the format `project:region:instance`, for example `my-project:us-central1:my-db`. |         | yes      |

### `query_details`

| Name               | Type       | Description                                          | Default | Required |
|--------------------|------------|------------------------------------------------------|---------|----------|
| `collect_interval` | `duration` | How frequently to collect information from database. | `"1m"`  | no       |
| `statements_limit` | `integer`  | Max number of recent queries to collect details for. | `100`   | no       |

### `query_samples`

| Name                      | Type       | Description                                                   | Default | Required |
|---------------------------|------------|---------------------------------------------------------------|---------|----------|
| `collect_interval`        | `duration` | How frequently to collect information from database.          | `"15s"` | no       |
| `disable_query_redaction` | `bool`     | Collect unredacted SQL query text (might include parameters). | `false` | no       |
| `exclude_current_user`    | `bool`     | Do not collect query samples for current database user.       | `true`  | no       |
| `enable_pre_classified_wait_events`   | `boolean`  | When `true`, emits telemetry data with pre-classified wait event information. | `false` | no       |

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

### `prometheus_exporter`

The `prometheus_exporter` block configures the embedded postgres_exporter scrapers.
The `data_source_name` is inherited from the parent block.

Refer to [`prometheus.exporter.postgres`](../../prometheus/prometheus.exporter.postgres/) docs for the full list of supported arguments and sub-blocks.

## `logs` collector

The `logs` collector processes PostgreSQL logs received through the `logs_receiver` entry point and exports Prometheus metrics for query and server errors.

The `logs_receiver` entry point must be fed by `loki` log source components, for example:

- `loki.source.file`: to read and process PostgreSQL log files from a self-hosted database instance
- `otelcol.receiver.awscloudwatch` and `otelcol.exporter.loki`: to read and process CloudWatch Logs for an AWS RDS instance

{{< admonition type="note" >}}
Refer to the [documentation](https://grafana.com/docs/grafana-cloud/monitor-applications/database-observability/get-started/postgres/) for detailed log configuration options.
{{< /admonition >}}

### Emitted Loki entries

Alongside `database_observability_pg_errors_total`, the `logs` collector
forwards a Loki entry on its `forward_to` target for every PostgreSQL
`ERROR`/`FATAL`/`PANIC` for which the matching `STATEMENT:` continuation was
successfully observed. The entry uses a single stable label and encodes
every other field as logfmt key/value pairs in the line body — no Loki
structured metadata is used, so the entries are portable across Loki
versions and downstream tooling.

| Field                          | Source                  | Notes                                                                |
|--------------------------------|-------------------------|----------------------------------------------------------------------|
| Label `op`                     | constant                | `error`                                                              |
| Body field `level`             | constant                | `info` (Alloy convention from `BuildLokiEntry`)                      |
| Body field `severity`          | log keyword             | `ERROR`, `FATAL`, or `PANIC`                                         |
| Body field `sqlstate`          | `%e`                    | full 5-character SQLSTATE                                            |
| Body field `sqlstate_class`    | first 2 chars of `%e`   | `40`, `42`, `53`, `23`, ...                                          |
| Body field `datname`           | `%d`                    | database name                                                        |
| Body field `query_fingerprint` | computed                | `libpg_query` fingerprint of the SQL text (16-char hex)              |
| Body field `pid`               | `%p`                    | backend PID                                                          |
| Body field `backend_start`     | `%s`                    | session start timestamp (raw text)                                   |
| Body field `application_name`  | `%a`                    | typically `[unknown]` unless set client-side                         |
| Body field `xid`               | `%x`                    | omitted when `0` (read-only / not yet assigned)                      |
| Body field `client_addr`       | host portion of `%r`    | `[local]` for unix-domain                                            |
| Body field `client_port`       | port portion of `%r`    | empty for unix-domain                                                |
| Body field `session_id`        | `%c`                    | unique per backend connection                                        |
| Body field `user`              | `%u`                    | also present on `pg_errors_total` as a label                         |
| Body field `error_message`     | text after `<sev>:`     | human-readable error message                                         |

Compute error rate per logical query in LogQL:

```logql
sum by (datname, query_fingerprint)
  (count_over_time({op="error"} | logfmt [5m]))
```

Correlate to `pg_stat_activity` samples emitted by the `query_samples`
collector by joining on `query_fingerprint` AND `pid` AND a small time
window around the entry's timestamp. If the failed query was inside a write
transaction, `xid` is also a deterministic key against
`pg_stat_activity.backend_xid`.

### `op="query_association"` entries from `query_details`

The `query_details` collector emits one Loki entry per `pg_stat_statements`
row on each scrape. The line carries the row's `queryid`, the computed
`query_fingerprint`, and the partially-normalized SQL text from the view.
Use it as a queryid → fingerprint lookup table to translate aggregate
metrics in `pg_stat_statements_*` into per-fingerprint rollups.

| Field                          | Source                          | Notes                                                                |
|--------------------------------|---------------------------------|----------------------------------------------------------------------|
| Label `op`                     | constant                        | `query_association`                                                  |
| Body field `level`             | constant                        | `info`                                                               |
| Body field `queryid`           | `pg_stat_statements.queryid`    | PostgreSQL's native, version-dependent identifier                    |
| Body field `query_fingerprint` | computed                        | `libpg_query` fingerprint of the SQL text (16-char hex)              |
| Body field `querytext`         | `pg_stat_statements.query`      | partially-normalized SQL with comments stripped                      |
| Body field `datname`           | from `pg_database`              | database name                                                        |

Look up the queryids belonging to a fingerprint via LogQL, then aggregate
`pg_stat_statements_*` counters by those queryids in PromQL:

```logql
{op="query_association"} | logfmt | query_fingerprint="<fp>"
```

Because `pg_query.Fingerprint` canonicalizes constants at the AST level,
the fingerprint emitted here matches the fingerprint computed from the raw
`pg_stat_activity.query` text (`op="query_sample"`) and from server-log
`STATEMENT:` continuations (`op="error"`). The same value is the join key
across all three surfaces.

## Example

```alloy
database_observability.postgres "orders_db" {
  data_source_name = "postgres://user:pass@localhost:5432/dbname"
  forward_to       = [loki.relabel.orders_db.receiver]
  targets          = prometheus.exporter.postgres.orders_db.targets

  enable_collectors = ["query_samples", "explain_plans"]
}

prometheus.exporter.postgres "orders_db" {
  data_source_name   = "postgres://user:pass@localhost:5432/dbname"
  enabled_collectors = ["stat_statements"]
}

// OPTIONAL: read PostgreSQL log files and forward to logs collector
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

# Setting Up Database Observability with Grafana Cloud

> NOTE: while the components `database_observability.mysql` and `database_observability.postgres` are marked as experimental, it is recommended to refer to the ["next"](https://grafana.com/docs/alloy/next/reference/components/database_observability/) version of docs for a complete reference documentation.

## MySQL

### Setting up the MySQL database

1. Your MySQL DB should be above version 8.

2. Create a dedicated DB user and grant permissions.

```sql
CREATE USER 'db-o11y'@'%' IDENTIFIED by '<password>';
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'db-o11y'@'%';
GRANT SELECT ON performance_schema.* TO 'db-o11y'@'%';
```

3. Grant the `db-o11y` user additional privileges to access the objects (schemas, tables, views) for which you want to collect detailed information.

For example, to limit permissions only to a schema named `payments`:

```sql
GRANT SELECT, SHOW VIEW ON payments.* TO 'db-o11y'@'%';
```

Alternatively, grant access to all available schemas:

```sql
GRANT SELECT, SHOW VIEW ON *.* TO 'db-o11y'@'%';
```

4. Verify that the user has been properly created.

```sql
SHOW GRANTS FOR 'db-o11y'@'%';

+-------------------------------------------------------------------+
| Grants for db-o11y@%                                              |
+-------------------------------------------------------------------+
| GRANT PROCESS, REPLICATION CLIENT ON *.* TO `db-o11y`@`%`         |
| GRANT SELECT, SHOW VIEW ON *.* TO `db-o11y`@`%`                   |
+-------------------------------------------------------------------+
```

5. Enable Performance Schema. To enable it explicitly, start the server with the `performance_schema` variable set to an appropriate value. Verify that Performance Schema has been enabled:

```sql
SHOW VARIABLES LIKE 'performance_schema';

+--------------------+-------+
| Variable_name      | Value |
+--------------------+-------+
| performance_schema | ON    |
+--------------------+-------+
```

6. Increase `max_digest_length` and `performance_schema_max_digest_length` to `4096`. Verify that the changes have been applied:

```sql
SHOW VARIABLES LIKE 'max_digest_length';

+-------------------+-------+
| Variable_name     | Value |
+-------------------+-------+
| max_digest_length | 4096  |
+-------------------+-------+
```

and

```sql
SHOW VARIABLES LIKE 'performance_schema_max_digest_length';

+--------------------------------------+-------+
| Variable_name                        | Value |
+--------------------------------------+-------+
| performance_schema_max_digest_length | 4096  |
+--------------------------------------+-------+
```

7. [OPTIONAL] Increase `performance_schema_max_sql_text_length` to `4096` if you want to collect the actual, unredacted sql text from queries samples (this requires setting `disable_query_redaction` to `true`, see later). Verify that the changes have been applied:

```sql
SHOW VARIABLES LIKE 'performance_schema_max_sql_text_length';

+----------------------------------------+-------+
| Variable_name                          | Value |
+----------------------------------------+-------+
| performance_schema_max_sql_text_length | 4096  |
+----------------------------------------+-------+
```

8. [OPTIONAL] Enable the `events_statements_cpu` consumer if you want to capture CPU activity and time on query samples. Verify the current setting with a sql query:

```sql
SELECT * FROM performance_schema.setup_consumers WHERE NAME = 'events_statements_cpu';
```

Use this statement to enable the consumer if it's disabled:

```sql
UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME = 'events_statements_cpu';
```

Note that the `events_statements_cpu` consumer will be disabled again when the database is restarted. If you prefer Alloy to verify and enable the consumer on your behalf then extend the grants of the `db-o11y` user:

```sql
GRANT UPDATE ON performance_schema.setup_consumers TO 'db-o11y'@'%';
```

and additionally enable these options:

```
database_observability.mysql "mysql_<your_DB_name>" {
  enable_collectors = ["query_samples"]

  // Global option to allow writing to performance_schema tables
  allow_update_performance_schema_settings = true

  // Option to allow the `query_samples` collector to
  // enable the 'events_statements_cpu' consumer
  query_samples {
    auto_enable_setup_consumers = true
  }
}
```

9. [OPTIONAL] Enable the `events_waits_current` and `events_waits_history` consumers if you want to collect wait events for each query sample. Verify the current settings with a sql query:

```sql
SELECT * FROM performance_schema.setup_consumers WHERE NAME IN ('events_waits_current', 'events_waits_history');
```

Use this statement to enable the consumers if they are disabled:

```sql
UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME IN ('events_waits_current', 'events_waits_history');
```

As noted in the step above, these consumers will be disabled again when the database is restarted. If you prefer Alloy to verify and enable the consumers on your behalf then follow the instructions from the step above.

### Running and configuring Alloy

1. You need to run the latest Alloy version from the `main` branch. The latest tags are available here on [Docker Hub](https://hub.docker.com/r/grafana/alloy-dev/tags) (for example, `grafana/alloy-dev:v1.10.0-devel-630bcbb` or more recent) . Additionally, the `--stability.level=experimental` CLI flag is necessary for running the `database_observability` component.

2. Add the following configuration block to Alloy.
- Replace `<your_DB_name>`
- Create a [`local.file`](https://grafana.com/docs/alloy/latest/reference/components/local/local.file/) with your DB secrets. The content of the file should be the [Data Source Name](https://github.com/go-sql-driver/mysql#dsn-data-source-name) string, for example `"user:password@(hostname:port)/"`.

3. Copy this block for each DB you'd like to monitor.

```
local.file "mysql_secret_<your_DB_name>" {
  filename  = "/var/lib/alloy/mysql_secret_<your_DB_name>"
  is_secret = true
}

prometheus.exporter.mysql "integrations_mysqld_exporter_<your_DB_name>" {
  data_source_name  = local.file.mysql_secret_<your_DB_name>.content
  enable_collectors = ["perf_schema.eventsstatements", "perf_schema.eventswaits"]
}

database_observability.mysql "mysql_<your_DB_name>" {
  data_source_name  = local.file.mysql_secret_<your_DB_name>.content
  forward_to        = [loki.relabel.database_observability_mysql_<your_DB_name>.receiver]
  targets           = prometheus.exporter.mysql.integrations_mysqld_exporter_<your_DB_name>.targets

  // OPTIONAL: enable collecting samples of queries with their execution metrics. The sql text will be redacted to hide sensitive params.
  enable_collectors = ["query_samples"]

  // OPTIONAL: if `query_samples` collector is enabled, you can use
  // the following setting to disable sql text redaction (by default
  // query samples are redacted).
  query_samples {
    disable_query_redaction = true
  }

  // OPTIONAL: provide additional information specific to the Cloud Provider
  // that hosts the database to enable certain infrastructure observability features.
  cloud_provider {
    aws {
      arn = "your-rds-db-arn"
    }
  }
}

loki.relabel "database_observability_mysql_<your_DB_name>" {
  forward_to = [loki.write.logs_service.receiver]

  // OPTIONAL: add any additional relabeling rules; must be consistent with rules in "discovery.relabel"
  rule {
    target_label = "instance"
    replacement  = "<instance_label>"
  }
  rule {
    target_label = "<custom_label_1>"
    replacement  = "<custom_value_1>"
  }
}

discovery.relabel "database_observability_mysql_<your_DB_name>" {
  targets = database_observability.mysql.mysql_<your_DB_name>.targets

  rule {
    target_label = "job"
    replacement  = "integrations/db-o11y"
  }

  // OPTIONAL: add any additional relabeling rules; must be consistent with rules in "loki.relabel"
  rule {
    target_label = "instance"
    replacement  = "<instance_label>"
  }
  rule {
    target_label = "<custom_label_1>"
    replacement  = "<custom_value_1>"
  }
}

prometheus.scrape "database_observability_mysql_<your_DB_name>" {
  targets    = discovery.relabel.database_observability_mysql_<your_DB_name>.output
  job_name   = "integrations/db-o11y"
  forward_to = [prometheus.remote_write.metrics_service.receiver]
}
```

4. Add Prometheus remote_write and Loki write config, if not present already. From the Grafana Cloud home, click on your stack and view URLs under Prometheus and Loki details where API tokens may also be generated.

```
prometheus.remote_write "metrics_service" {
  endpoint {
    url = sys.env("GCLOUD_HOSTED_METRICS_URL")

    basic_auth {
      password = sys.env("GCLOUD_RW_API_KEY")
      username = sys.env("GCLOUD_HOSTED_METRICS_ID")
    }
  }
}

loki.write "logs_service" {
  endpoint {
    url = sys.env("GCLOUD_HOSTED_LOGS_URL")

    basic_auth {
      password = sys.env("GCLOUD_RW_API_KEY")
      username = sys.env("GCLOUD_HOSTED_LOGS_ID")
    }
  }
}
```

### Configuring Alloy with the k8s-monitoring helm chart

When using the k8s-monitoring helm chart you might need to extend your `values.yaml` with:

```yaml
alloy:
  image:
    repository: "grafana/alloy-dev"
    tag: <alloy-version> // e.g. "v1.10.0-devel-630bcbb"

  alloy:
    stabilityLevel: experimental

extraConfig: |
  // Add the config blocks for Database Observability
  prometheus.exporter.mysql "integrations_mysqld_exporter_<your_DB_name>" {
    ...
  }
  ...
  database_observability.mysql "mysql_<your_DB_name>" {
    ...
  }
```

### Example Alloy configuration

This is a complete example of Alloy Database Observability configuration using two different databases:

```
prometheus.remote_write "metrics_service" {
  endpoint {
    url = sys.env("GCLOUD_HOSTED_METRICS_URL")

    basic_auth {
      password = sys.env("GCLOUD_RW_API_KEY")
      username = sys.env("GCLOUD_HOSTED_METRICS_ID")
    }
  }
}

loki.write "logs_service" {
  endpoint {
    url = sys.env("GCLOUD_HOSTED_LOGS_URL")

    basic_auth {
      password = sys.env("GCLOUD_RW_API_KEY")
      username = sys.env("GCLOUD_HOSTED_LOGS_ID")
    }
  }
}

local.file "mysql_secret_example_db_1" {
  filename  = "/var/lib/alloy/mysql_secret_example_db_1"
  is_secret = true
}

prometheus.exporter.mysql "integrations_mysqld_exporter_example_db_1" {
  data_source_name  = local.file.mysql_secret_example_db_1.content
  enable_collectors = ["perf_schema.eventsstatements", "perf_schema.eventswaits"]
}

database_observability.mysql "mysql_example_db_1" {
  data_source_name  = local.file.mysql_secret_example_db_1.content
  forward_to        = [loki.relabel.database_observability_mysql_example_db_1.receiver]
  targets           = prometheus.exporter.mysql.integrations_mysqld_exporter_example_db_1.targets
  enable_collectors = ["query_samples"]
}

loki.relabel "database_observability_mysql_example_db_1" {
  forward_to = [loki.write.logs_service.receiver]
}

discovery.relabel "database_observability_mysql_example_db_1" {
  targets = database_observability.mysql.mysql_example_db_1.targets

  rule {
    target_label = "job"
    replacement  = "integrations/db-o11y"
  }
}

prometheus.scrape "database_observability_mysql_example_db_1" {
  targets    = discovery.relabel.database_observability_mysql_example_db_1.output
  job_name   = "integrations/db-o11y"
  forward_to = [prometheus.remote_write.metrics_service.receiver]
}

local.file "mysql_secret_example_db_2" {
  filename  = "/var/lib/alloy/mysql_secret_example_db_2"
  is_secret = true
}

prometheus.exporter.mysql "integrations_mysqld_exporter_example_db_2" {
  data_source_name  = local.file.mysql_secret_example_db_2.content
  enable_collectors = ["perf_schema.eventsstatements", "perf_schema.eventswaits"]
}

database_observability.mysql "mysql_example_db_2" {
  data_source_name  = local.file.mysql_secret_example_db_2.content
  forward_to        = [loki.relabel.database_observability_mysql_example_db_2.receiver]
  targets           = prometheus.exporter.mysql.integrations_mysqld_exporter_example_db_2.targets
  enable_collectors = ["query_samples"]
}

loki.relabel "database_observability_mysql_example_db_2" {
  forward_to = [loki.write.logs_service.receiver]
}

discovery.relabel "database_observability_mysql_example_db_2" {
  targets = database_observability.mysql.mysql_example_db_2.targets

  rule {
    target_label = "job"
    replacement  = "integrations/db-o11y"
  }
}

prometheus.scrape "database_observability_mysql_example_db_2" {
  targets    = discovery.relabel.database_observability_mysql_example_db_2.targets
  job_name   = "integrations/db-o11y"
  forward_to = [prometheus.remote_write.metrics_service.receiver]
}
```

## PostgreSQL

### Setting up the Postgres database

1. Your Postgres DB should be at least version 16.

2. Add the `pg_stat_statements` module to `shared_preload_libraries` in `postgresql.conf`. This requires a restart of the Postgres DB.

3. Create the `pg_stat_statements` extension in every database you want to monitor.

```sql
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

4. Verify that the extension is enabled.

```sql
SELECT * FROM pg_extension WHERE extname = 'pg_stat_statements';
```

5. Increase `track_activity_query_size` to `4096`. Verify that the change has been applied:

```sql
show track_activity_query_size;

 track_activity_query_size
---------------------------
 4kB
```

6. Create a dedicated DB user and grant permissions to monitor the DB.

```sql
CREATE USER "db-o11y" WITH PASSWORD '<password>';
GRANT pg_monitor TO "db-o11y";
GRANT pg_read_all_stats TO "db-o11y";
```

7. Verify that the user has been properly created and has the correct privileges for the `pg_stat_statements` extension.

```sql
-- run with the `db-o11y` user
SELECT * FROM pg_stat_statements LIMIT 1;
```

8. Grant the `db-o11y` user additional privileges to access the objects (databases, schemas, tables, views) for which you want to collect detailed information.

For example, connect to a `payments` database and grant access to specific schemas:

```sql
-- switch to the 'payments' database
\c payments

-- grant USAGE and SELECT permissions in the 'public' schema
GRANT USAGE ON SCHEMA public TO "db-o11y";
GRANT SELECT ON ALL TABLES IN SCHEMA public TO "db-o11y";

-- grant USAGE and SELECT permissions in the 'tests' schema
GRANT USAGE ON SCHEMA tests TO "db-o11y";
GRANT SELECT ON ALL TABLES IN SCHEMA tests TO "db-o11y";
```

Alternatively, use the predefined role `pg_read_all_data` to grant `USAGE` and `SELECT` permissions to all objects at once:

```sql
GRANT pg_read_all_data TO "db-o11y";
```

### Running and configuring Alloy

1. You need to run the latest Alloy version from the `main` branch. The latest tags are available here on [Docker Hub](https://hub.docker.com/r/grafana/alloy-dev/tags) (for example, `grafana/alloy-dev:v1.10.0-devel-630bcbb` or more recent) . Additionally, the `--stability.level=experimental` CLI flag is necessary for running the `database_observability` component.

2. Add the following configuration block to Alloy.
- Replace `<your_DB_name>`
- Create a [`local.file`](https://grafana.com/docs/alloy/latest/reference/components/local/local.file/) with your DB secrets. The content of the file should be the Data Source Name string, for example `"postgresql://user:password@(hostname:port)/postgres?sslmode=require"`.

3. Copy this block for each DB you'd like to monitor.

```
local.file "postgres_secret_<your_DB_name>" {
  filename  = "/var/lib/alloy/postgres_secret_<your_DB_name>"
  is_secret = true
}

prometheus.exporter.postgres "integrations_postgres_exporter_<your_DB_name>" {
  data_source_name  = local.file.postgres_secret_<your_DB_name>.content
  enabled_collectors = ["stat_statements"]

  autodiscovery {
    enabled = true

    // If running on AWS RDS, exclude the `rdsadmin` database
    database_denylist = ["rdsadmin"]
  }
}

database_observability.postgres "postgres_<your_DB_name>" {
  data_source_name  = local.file.postgres_secret_<your_DB_name>.content
  forward_to        = [loki.relabel.database_observability_postgres_<your_DB_name>.receiver]
  targets           = prometheus.exporter.postgres.integrations_postgres_exporter_<your_DB_name>.targets

  // OPTIONAL: enable collecting samples of queries with their execution metrics. The sql text will be redacted to hide sensitive params.
  enable_collectors = ["query_samples", "query_details"]

  // OPTIONAL: if `query_samples` collector is enabled, you can use
  // the following setting to disable sql text redaction (by default
  // query samples are redacted).
  query_samples {
    disable_query_redaction = true
  }

  // OPTIONAL: provide additional information specific to the Cloud Provider
  // that hosts the database to enable certain infrastructure observability features.
  cloud_provider {
    aws {
      arn = "your-rds-db-arn"
    }
  }
}

loki.relabel "database_observability_postgres_<your_DB_name>" {
  forward_to = [loki.write.logs_service.receiver]

  // OPTIONAL: add any additional relabeling rules; must be consistent with rules in "discovery.relabel"
  rule {
    target_label = "instance"
    replacement  = "<instance_label>"
  }
  rule {
    target_label = "<custom_label_1>"
    replacement  = "<custom_value_1>"
  }
}

discovery.relabel "database_observability_postgres_<your_DB_name>" {
  targets = concat(prometheus.exporter.postgres.integrations_postgres_exporter_<your_DB_name>.targets, database_observability.postgres.postgres_<your_DB_name>.targets)

  rule {
    target_label = "job"
    replacement  = "integrations/db-o11y"
  }

  // OPTIONAL: add any additional relabeling rules; must be consistent with rules in "loki.relabel"
  rule {
    target_label = "instance"
    replacement  = "<instance_label>"
  }
  rule {
    target_label = "<custom_label_1>"
    replacement  = "<custom_value_1>"
  }
}

prometheus.scrape "database_observability_postgres_<your_DB_name>" {
  targets    = discovery.relabel.database_observability_postgres_<your_DB_name>.output
  job_name   = "integrations/db-o11y"
  forward_to = [prometheus.remote_write.metrics_service.receiver]
}
```

4. Add Prometheus remote_write and Loki write config, if not present already. From the Grafana Cloud home, click on your stack and view URLs under Prometheus and Loki details where API tokens may also be generated.

```
prometheus.remote_write "metrics_service" {
  endpoint {
    url = sys.env("GCLOUD_HOSTED_METRICS_URL")

    basic_auth {
      password = sys.env("GCLOUD_RW_API_KEY")
      username = sys.env("GCLOUD_HOSTED_METRICS_ID")
    }
  }
}

loki.write "logs_service" {
  endpoint {
    url = sys.env("GCLOUD_HOSTED_LOGS_URL")

    basic_auth {
      password = sys.env("GCLOUD_RW_API_KEY")
      username = sys.env("GCLOUD_HOSTED_LOGS_ID")
    }
  }
}
```

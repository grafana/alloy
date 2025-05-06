# Setting Up Database Observability with Grafana Cloud

## Setting up the MySQL database

1. Your MySQL DB should be above version 8.

2. Create a dedicated DB user and grant permissions.

```sql
CREATE USER 'db-o11y'@'%' IDENTIFIED by '<password>';
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'db-o11y'@'%';
GRANT SELECT, SHOW VIEW ON *.* TO 'db-o11y'@'%'; /* see note */
```

Please note: Regarding `GRANT SELECT, SHOW VIEW ON *.* TO 'db-o11y'@'%'`, it is possible to restrict permissions, if necessary. Instead, grant the `db-o11y` user privileges access only to the objects (schemas) for which you want information. For example, to restrict permissions only to a schema named `payments`:

```sql
CREATE USER 'db-o11y'@'%' IDENTIFIED by '<password>';
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'db-o11y'@'%';
GRANT SELECT ON performance_schema.* TO 'db-o11y'@'%';   /* required */
GRANT SELECT, SHOW VIEW ON payments.* TO 'db-o11y'@'%';  /* limit grant to the `payments` schema */
```

3. Verify that the user has been properly created.

```sql
SHOW GRANTS FOR 'db-o11y'@'%';

+-------------------------------------------------------------------+
| Grants for db-o11y@%                                              |
+-------------------------------------------------------------------+
| GRANT PROCESS, REPLICATION CLIENT ON *.* TO `db-o11y`@`%`         |
| GRANT SELECT, SHOW VIEW ON *.* TO `db-o11y`@`%`                   |
+-------------------------------------------------------------------+
```

4. Enable Performance Schema. To enable it explicitly, start the server with the `performance_schema` variable set to an appropriate value. Verify that Performance Schema has been enabled:

```sql
SHOW VARIABLES LIKE 'performance_schema';

+--------------------+-------+
| Variable_name      | Value |
+--------------------+-------+
| performance_schema | ON    |
+--------------------+-------+
```

5. Increase `max_digest_length` and `performance_schema_max_digest_length` to `4096`. Verify that the changes have been applied:

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

## Running and configuring Alloy

1. You need to run the latest Alloy version from the `main` branch. The latest tags are available here on [Docker Hub](https://hub.docker.com/r/grafana/alloy-dev/tags) (for example, `grafana/alloy-dev:v1.9.0-devel-5128872` or more recent) . Additionally, the `--stability.level=experimental` CLI flag is necessary for running the `database_observability` component.

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
  enable_collectors = ["perf_schema.eventsstatements"]
  perf_schema.eventsstatements {
    text_limit = 2048
  }
}

database_observability.mysql "mysql_<your_DB_name>" {
  data_source_name = local.file.mysql_secret_<your_DB_name>.content
  forward_to       = [loki.relabel.database_observability_mysql_<your_DB_name>.receiver]
  collect_interval = "1m"
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
  targets = concat(prometheus.exporter.mysql.integrations_mysqld_exporter_<your_DB_name>.targets, database_observability.mysql.mysql_<your_DB_name>.targets)

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

## Configuring Alloy with the k8s-monitoring helm chart

When using the k8s-monitoring helm chart you might need to extend your `values.yaml` with:

```yaml
alloy:
  image:
    repository: "grafana/alloy-dev"
    tag: <alloy-version> // e.g. "v1.9.0-devel-5128872"

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

## Example Alloy configuration

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
  enable_collectors = ["perf_schema.eventsstatements"]
  perf_schema.eventsstatements {
    text_limit = 2048
  }
}

database_observability.mysql "mysql_example_db_1" {
  data_source_name = local.file.mysql_secret_example_db_1.content
  forward_to       = [loki.relabel.database_observability_mysql_example_db_1.receiver]
  collect_interval = "1m"
}

loki.relabel "database_observability_mysql_example_db_1" {
  forward_to = [loki.write.logs_service.receiver]
}

discovery.relabel "database_observability_mysql_example_db_1" {
  targets = concat(prometheus.exporter.mysql.integrations_mysqld_exporter_example_db_1.targets, database_observability.mysql.mysql_example_db_1.targets)

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
  enable_collectors = ["perf_schema.eventsstatements"]
  perf_schema.eventsstatements {
    text_limit = 2048
  }
}

database_observability.mysql "mysql_example_db_2" {
  data_source_name = local.file.mysql_secret_example_db_2.content
  forward_to       = [loki.relabel.database_observability_mysql_example_db_2.receiver]
  collect_interval = "1m"
}

loki.relabel "database_observability_mysql_example_db_2" {
  forward_to = [loki.write.logs_service.receiver]
}

discovery.relabel "database_observability_mysql_example_db_2" {
  targets = concat(prometheus.exporter.mysql.integrations_mysqld_exporter_example_db_2.targets, database_observability.mysql.mysql_example_db_2.targets)

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

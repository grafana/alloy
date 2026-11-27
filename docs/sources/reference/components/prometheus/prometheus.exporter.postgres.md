---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.postgres/
aliases:
  - ../prometheus.exporter.postgres/ # /docs/alloy/latest/reference/components/prometheus.exporter.postgres/
description: Learn about prometheus.exporter.postgres
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-10-14
title: prometheus.exporter.postgres
---

# `prometheus.exporter.postgres`

The `prometheus.exporter.postgres` component embeds the [`postgres_exporter`][postgres-exporter] to collect metrics from a PostgreSQL database.

You can specify multiple `prometheus.exporter.postgres` components by giving them different labels.

[postgres-exporter]: https://github.com/prometheus-community/postgres_exporter

## Usage

```alloy
prometheus.exporter.postgres "<LABEL>" {
  data_source_names = ["<DATA_SOURCE_NAME>"]
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.postgres`:

| Name                         | Type           | Description                                                                   | Default | Required |
| ---------------------------- | -------------- | ----------------------------------------------------------------------------- | ------- | -------- |
| `data_source_names`          | `list(secret)` | Specifies the PostgreSQL servers to connect to.                               |         | yes      |
| `custom_queries_config_path` | `string`       | Path to YAML file containing custom queries to expose as metrics.             | `""`    | no       |
| `disable_default_metrics`    | `bool`         | When `true`, only exposes metrics supplied from `custom_queries_config_path`. | `false` | no       |
| `disable_settings_metrics`   | `bool`         | Disables collection of metrics from `pg_settings`.                            | `false` | no       |
| `enabled_collectors`         | `list(string)` | List of collectors to enable. Refer to the information below for more detail. | `[]`    | no       |

Refer to the [PostgreSQL documentation][libpq-connstring] for more information about the format of the connection strings in `data_source_names`.

Refer to the examples for the `custom_queries_config_path` file in the [`postgres_exporter` repository][postgres-exporter-queries].

[libpq-connstring]: https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
[postgres-exporter-queries]: https://github.com/prometheus-community/postgres_exporter/blob/master/queries.yaml

{{< admonition type="note" >}}
Avoid configuring the exporter with environment variables.
They affect _all_ `prometheus.exporter.postgres` components, not just one.
Refer to the [`postgres_exporter` repository][postgres-exporter-env] for a full list of environment variables.

[postgres-exporter-env]: https://github.com/prometheus-community/postgres_exporter#environment-variables
{{< /admonition >}}

The `prometheus.exporter.postgres` component configures the embedded [`postgres_exporter`][postgres-exporter] to run with a collection timeout of 10 seconds.

By default, the component enables the same set of metrics as the upstream [`postgres_exporter`][postgres-exporter].
Set `custom_queries_config_path` to expose additional metrics from a configuration file.

Set `disable_default_metrics` to `true` to expose only the metrics from `custom_queries_config_path`.
That makes `custom_queries_config_path` required, and you can't set `enabled_collectors`.

Set `enabled_collectors` to control a subset of the metrics collectors.
The following collectors are available for selection:

{{< column-list >}}

* `buffercache_summary`
* `database`
* `database_wraparound`
* `locks`
* `long_running_transactions`
* `postmaster`
* `process_idle`
* `replication`
* `replication_slot`
* `roles`
* `stat_activity_autovacuum`
* `stat_bgwriter`
* `stat_checkpointer` - Only supported in PostgreSQL 17 and later
* `stat_database`
* `stat_progress_vacuum`
* `stat_statements`
* `stat_user_tables`
* `stat_wal_receiver`
* `statio_user_indexes`
* `statio_user_tables`
* `wal`
* `xlog_location`

{{< /column-list >}}

The component enables the following collectors by default:

{{< column-list >}}

* `database`
* `locks`
* `replication`
* `replication_slot`
* `roles`
* `stat_bgwriter`
* `stat_database`
* `stat_progress_vacuum`
* `stat_user_tables`
* `statio_user_tables`
* `wal`

{{< /column-list >}}

{{< admonition type="note" >}}
Due to a limitation of the upstream exporter, `enabled_collectors` applies only to the first data source when you configure multiple `data_source_names`.
{{< /admonition >}}

The component sets the `instance` label on its exported targets to a simplified form of the data source name, `postgresql://<HOST>:<PORT>/<DATABASE>`, with any credentials removed.
The component omits the port when the data source name specifies a host without a port.
When the data source name omits the host, the component uses `localhost:5432`.
When you configure more than one data source name, the component sets the `instance` label to the component ID.

## Blocks

You can use the following blocks with `prometheus.exporter.postgres`:

{{< docs/alloy-config >}}

| Block                                | Description                                   | Required |
| ------------------------------------ | --------------------------------------------- | -------- |
| [`autodiscovery`][autodiscovery]     | Configures discovery of additional databases. | no       |
| [`stat_statements`][stat_statements] | Configures the `stat_statements` collector.   | no       |

[autodiscovery]: #autodiscovery
[stat_statements]: #stat_statements

{{< /docs/alloy-config >}}

### `autodiscovery`

The `autodiscovery` block configures discovery of databases, outside of any specified in `data_source_names`.

You can use the following arguments with the `autodiscovery` block:

| Name                 | Type           | Description                                        | Default | Required |
| -------------------- | -------------- | -------------------------------------------------- | ------- | -------- |
| `database_allowlist` | `list(string)` | Scrape only these databases.                       |         | no       |
| `database_denylist`  | `list(string)` | Scrape all databases except these.                 |         | no       |
| `enabled`            | `bool`         | Whether to automatically discover other databases. | `false` | no       |

If you set `enabled` to `true` without an allowlist or a denylist, the exporter scrapes all databases.

If you don't enable `autodiscovery`, neither `database_allowlist` nor `database_denylist` has any effect.

### `stat_statements`

The `stat_statements` block configures the selection of both the query ID and the full SQL statement.
This configuration takes effect only when you enable the `stat_statements` collector.

You can use the following arguments with the `stat_statements` block:

| Name                | Type           | Description                                         | Default | Required |
| ------------------- | -------------- | --------------------------------------------------- | ------- | -------- |
| `exclude_databases` | `list(string)` | Comma-separated list of database names to exclude.  | `[]`    | no       |
| `exclude_users`     | `list(string)` | Comma-separated list of user names to exclude.      | `[]`    | no       |
| `include_query`     | `bool`         | Enable the selection of query ID and SQL statement. | `false` | no       |
| `limit`             | `number`       | Maximum number of statements to fetch.              | `100`   | no       |
| `query_length`      | `number`       | Maximum length of the statement query text.         | `120`   | no       |

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.postgres` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.postgres` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.postgres` doesn't expose any component-specific debug metrics.

## Examples

### Collect metrics from a PostgreSQL server

The following example uses a `prometheus.exporter.postgres` component to collect metrics from a PostgreSQL server running locally with all default settings:

```alloy
// Because no autodiscovery is defined, this will only scrape the <DATABASE_NAME> database, as defined
// in the DSN below.
prometheus.exporter.postgres "example" {
  data_source_names = ["postgresql://<DB_USERNAME>:<DB_PASSWORD>@localhost:5432/<DATABASE_NAME>?sslmode=disable"]
}

prometheus.scrape "default" {
  targets    = prometheus.exporter.postgres.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

* _`<DB_USERNAME>`_: The username for the PostgreSQL server.
* _`<DB_PASSWORD>`_: The password for the PostgreSQL server.
* _`<DATABASE_NAME>`_: The name of the database to scrape.
* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Collect custom metrics from an allowlisted set of databases

The following example uses a `prometheus.exporter.postgres` component to collect custom metrics from a set of specific databases, replacing default metrics with custom metrics derived from queries in `/etc/alloy/custom-postgres-metrics.yaml`:

```alloy
prometheus.exporter.postgres "example" {
  data_source_names = ["postgresql://<DB_USERNAME>:<DB_PASSWORD>@localhost:5432/<DATABASE_NAME>?sslmode=disable"]

  // This block configures autodiscovery to check for databases outside of the <DATABASE_NAME> db
  // specified in the DSN above. The database_allowlist field limits autodiscovery to the
  // 'frontend_app' and 'backend_app' databases. The component also scrapes <DATABASE_NAME>.
  autodiscovery {
    enabled            = true
    database_allowlist = ["frontend_app", "backend_app"]
  }

  disable_default_metrics    = true
  custom_queries_config_path = "/etc/alloy/custom-postgres-metrics.yaml"
}

prometheus.scrape "default" {
  targets    = prometheus.exporter.postgres.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

* _`<DB_USERNAME>`_: The username for the PostgreSQL server.
* _`<DB_PASSWORD>`_: The password for the PostgreSQL server.
* _`<DATABASE_NAME>`_: The name of the database to scrape.
* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Collect metrics from all databases except for a denylisted database

This example uses a `prometheus.exporter.postgres` component to collect custom metrics from all databases except for the `secrets` database.

```alloy
prometheus.exporter.postgres "example" {
  data_source_names = ["postgresql://<DB_USERNAME>:<DB_PASSWORD>@localhost:5432/<DATABASE_NAME>?sslmode=disable"]

  // The database_denylist field will filter out those databases from the list of databases to scrape,
  // meaning that all databases *except* these will be scraped.
  //
  // In this example it will scrape all databases except for the one named 'secrets'.
  autodiscovery {
    enabled           = true
    database_denylist = ["secrets"]
  }
}

prometheus.scrape "default" {
  targets    = prometheus.exporter.postgres.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

* _`<DB_USERNAME>`_: The username for the PostgreSQL server.
* _`<DB_PASSWORD>`_: The password for the PostgreSQL server.
* _`<DATABASE_NAME>`_: The name of the database to scrape.
* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Escape special characters in a connection string

If your password contains special characters such as `@`, `:`, or `/`, wrap it with `encoding.url_encode`.

```alloy
prometheus.exporter.postgres "example" {
  data_source_names = [
    "postgresql://<DB_USERNAME>:" + encoding.url_encode("p@ss/w:ord!") + "@localhost:5432/<DATABASE_NAME>?sslmode=disable",
  ]
}
```
This keeps the connection string valid.

Replace the following:

* _`<DB_USERNAME>`_: The username for the PostgreSQL server.
* _`<DATABASE_NAME>`_: The name of the database to scrape.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.postgres` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

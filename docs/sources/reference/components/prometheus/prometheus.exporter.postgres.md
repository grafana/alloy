---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.postgres/
aliases:
  - ../prometheus.exporter.postgres/ # /docs/alloy/latest/reference/components/prometheus.exporter.postgres/
description: Learn about prometheus.exporter.postgres
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.postgres
---

# `prometheus.exporter.postgres`

The `prometheus.exporter.postgres` component embeds the [`postgres_exporter`](https://github.com/prometheus-community/postgres_exporter) for collecting metrics from a PostgreSQL database.

You can specify multiple `prometheus.exporter.postgres` components by giving them different labels.

## Usage

```alloy
prometheus.exporter.postgres "<LABEL>" {
    data_source_names = "<DATA_SOURCE_NAMES_LIST>"
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

Refer to the [PostgreSQL documentation](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING) for more information about the format of the connection strings in `data_source_names`.

Refer to the examples for the `custom_queries_config_path` file in the [`postgres_exporter` repository](https://github.com/prometheus-community/postgres_exporter/blob/master/queries.yaml).

{{< admonition type="note" >}}
There are a number of environment variables that aren't recommended for use, as they will affect _all_ `prometheus.exporter.postgres` components.
Refer to the [`postgres_exporter` repository](https://github.com/prometheus-community/postgres_exporter#environment-variables) for a full list of environment variables.
{{< /admonition >}}

By default, the same set of metrics is enabled as in the upstream [`postgres_exporter`](https://github.com/prometheus-community/postgres_exporter/).
If `custom_queries_config_path` is set, additional metrics defined in the given configuration file will be exposed.
If `disable_default_metrics` is set to `true`, only the metrics defined in the `custom_queries_config_path` file will be exposed.

A subset of metrics collectors can be controlled by setting the `enabled_collectors` argument.
The following collectors are available for selection:

{{< column-list >}}

* `buffercache_summary`
* `database_wraparound`
* `database`
* `locks`
* `long_running_transactions`
* `postmaster`
* `process_idle`
* `replication_slot`
* `replication`
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

By default, the following collectors are enabled:

{{< column-list >}}

* `database`
* `locks`
* `replication_slot`
* `replication`
* `roles`
* `stat_bgwriter`
* `stat_database`
* `stat_progress_vacuum`
* `stat_user_tables`
* `statio_user_tables`
* `wal`

{{< /column-list >}}

{{< admonition type="note" >}}
Due to a limitation of the upstream exporter, when multiple `data_source_names` are used, the collectors that are controlled via the `enabled_collectors` argument is only applied to the first data source in the list.
{{< /admonition >}}

## Blocks

You can use the following block with `prometheus.exporter.postgres`:

| Name                             | Description                  | Required |
| -------------------------------- | ---------------------------- | -------- |
| [`autodiscovery`][autodiscovery] | Database discovery settings. | no       |

[autodiscovery]: #autodiscovery

### `autodiscovery`

The `autodiscovery` block configures discovery of databases, outside of any specified in `data_source_names`.

The following arguments are supported:

| Name                 | Type           | Description                                                                    | Default | Required |
|----------------------|----------------|--------------------------------------------------------------------------------|---------|----------|
| `database_allowlist` | `list(string)` | List of databases to filter for, meaning only these databases will be scraped. |         | no       |
| `database_denylist`  | `list(string)` | List of databases to filter out, meaning all other databases will be scraped.  |         | no       |
| `enabled`            | `bool`         | Whether to automatically discover other databases.                             | `false` | no       |

If `enabled` is set to `true` and no allowlist or denylist is specified, the exporter scrapes from all databases.

If `autodiscovery` is disabled, neither `database_allowlist` nor `database_denylist` has any effect.

### `stat_statements`

The `stat_statements` block configures the selection of both the query ID and the full SQL statement.
This configuration takes effect only when the `stat_statements` collector is enabled.

The following arguments are supported:
| Name                | Type           | Description                                         | Default | Required |
| ------------------- | -------------- | --------------------------------------------------- | ------- | -------- |
| `include_query`     | `bool`         | Enable the selection of query ID and SQL statement. | `false` | no       |
| `query_length`      | `number`       | Maximum length of the statement query text.         | `120`   | no       |
| `limit`             | `number`       | Maximum number of statements to fetch.              | `100`   | no       |
| `exclude_databases` | `list(string)` | Comma-separated list of database names to exclude.  | `[]`    | no       |
| `exclude_users`     | `list(string)` | Comma-separated list of user names to exclude.      | `[]`    | no       |

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.postgres` is only reported as unhealthy if given an invalid configuration.

## Debug information

`prometheus.exporter.postgres` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.postgres` doesn't expose any component-specific debug metrics.

## Examples

### Collect metrics from a PostgreSQL server

The following example uses a `prometheus.exporter.postgres` component to collect metrics from a PostgreSQL server running locally with all default settings:

```alloy
// Because no autodiscovery is defined, this will only scrape the 'database_name' database, as defined
// in the DSN below.
prometheus.exporter.postgres "example" {
  data_source_names = ["postgresql://username:password@localhost:5432/database_name?sslmode=disable"]
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

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Collect custom metrics from an allowlisted set of databases

The following example uses a `prometheus.exporter.postgres` component to collect custom metrics from a set of specific databases, replacing default metrics with custom metrics derived from queries in `/etc/alloy/custom-postgres-metrics.yaml`:

```alloy
prometheus.exporter.postgres "example" {
  data_source_names = ["postgresql://username:password@localhost:5432/database_name?sslmode=disable"]

  // This block configures autodiscovery to check for databases outside of the 'database_name' db
  // specified in the DSN above. The database_allowlist field means that only the 'frontend_app' and 'backend_app'
  // databases will be scraped.
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

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

### Collect metrics from all databases except for a denylisted database

This example uses a `prometheus.exporter.postgres` component to collect custom metrics from all databases except for the `secrets` database.

```alloy
prometheus.exporter.postgres "example" {
  data_source_names = ["postgresql://username:password@localhost:5432/database_name?sslmode=disable"]

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

### Escape special characters in postgres url

If your PostgreSQL connection string includes special characters for e.g. password (`@`, `:`, `/`, etc.), you should wrap the password using `encoding.url_encode`.

```alloy
prometheus.exporter.postgres "example" {
  data_source_names = [
    "postgresql://username:" + encoding.url_encode("p@ss/w:ord!") + "@localhost:5432/dbname?sslmode=disable"
  ]
}
```
This ensures the DSN remains valid and correctly parsed.

Replace the following:

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.postgres` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

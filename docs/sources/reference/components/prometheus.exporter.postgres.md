---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/prometheus.exporter.postgres/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/prometheus.exporter.postgres/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/prometheus.exporter.postgres/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.exporter.postgres/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/prometheus.exporter.postgres/
description: Learn about prometheus.exporter.postgres
labels:
  stage: beta
title: prometheus.exporter.postgres
---

# prometheus.exporter.postgres

The `prometheus.exporter.postgres` component embeds
[postgres_exporter](https://github.com/prometheus-community/postgres_exporter) for collecting metrics from a postgres database.

Multiple `prometheus.exporter.postgres` components can be specified by giving them different
labels.

## Usage

```river
prometheus.exporter.postgres "LABEL" {
    data_source_names = DATA_SOURCE_NAMES_LIST
}
```

## Arguments

The following arguments are supported:

| Name                         | Type           | Description                                                                   | Default | Required |
| ---------------------------- | -------------- | ----------------------------------------------------------------------------- | ------- | -------- |
| `data_source_names`          | `list(secret)` | Specifies the Postgres server(s) to connect to.                               |         | yes      |
| `disable_settings_metrics`   | `bool`         | Disables collection of metrics from pg_settings.                              | `false` | no       |
| `disable_default_metrics`    | `bool`         | When `true`, only exposes metrics supplied from `custom_queries_config_path`. | `false` | no       |
| `custom_queries_config_path` | `string`       | Path to YAML file containing custom queries to expose as metrics.             | ""      | no       |

The format for connection strings in `data_source_names` can be found in the [official postgresql documentation](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING).

See examples for the `custom_queries_config_path` file in the [postgres_exporter repository](https://github.com/prometheus-community/postgres_exporter/blob/master/queries.yaml).

**NOTE**: There are a number of environment variables that are not recommended for use, as they will affect _all_ `prometheus.exporter.postgres` components. A full list can be found in the [postgres_exporter repository](https://github.com/prometheus-community/postgres_exporter#environment-variables).

## Blocks

The following blocks are supported:

| Hierarchy     | Block             | Description                  | Required |
| ------------- | ----------------- | ---------------------------- | -------- |
| autodiscovery | [autodiscovery][] | Database discovery settings. | no       |

[autodiscovery]: #autodiscovery-block

### autodiscovery block

The `autodiscovery` block configures discovery of databases, outside of any specified in `data_source_names`.

The following arguments are supported:

| Name                 | Type           | Description                                                                    | Default | Required |
| -------------------- | -------------- | ------------------------------------------------------------------------------ | ------- | -------- |
| `enabled`            | `bool`         | Whether to autodiscover other databases                                        | `false` | no       |
| `database_allowlist` | `list(string)` | List of databases to filter for, meaning only these databases will be scraped. |         | no       |
| `database_denylist`  | `list(string)` | List of databases to filter out, meaning all other databases will be scraped.  |         | no       |

If `enabled` is set to `true` and no allowlist or denylist is specified, the exporter will scrape from all databases.

If `autodiscovery` is disabled, neither `database_allowlist` nor `database_denylist` will have any effect.

## Exported fields

{{< docs/shared lookup="flow/reference/components/exporter-component-exports.md" source="agent" version="<AGENT_VERSION>" >}}

## Component health

`prometheus.exporter.postgres` is only reported as unhealthy if given
an invalid configuration.

## Debug information

`prometheus.exporter.postgres` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.postgres` does not expose any component-specific
debug metrics.

## Examples

### Collect metrics from a PostgreSQL server

This example uses a `prometheus.exporter.postgres` component to collect metrics from a Postgres
server running locally with all default settings:

```river
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
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

Replace the following:

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the remote_write API.
- `PASSWORD`: The password to use for authentication to the remote_write API.

### Collect custom metrics from an allowlisted set of databases

This example uses a `prometheus.exporter.postgres` component to collect custom metrics from a set of
specific databases, replacing default metrics with custom metrics derived from queries in `/etc/agent/custom-postgres-metrics.yaml`:

```river
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
  custom_queries_config_path = "/etc/agent/custom-postgres-metrics.yaml"
}

prometheus.scrape "default" {
  targets    = prometheus.exporter.postgres.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

Replace the following:

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the remote_write API.
- `PASSWORD`: The password to use for authentication to the remote_write API.

### Collect metrics from all databases except for a denylisted database

This example uses a `prometheus.exporter.postgres` component to collect custom metrics from all databases except
for the `secrets` database.

```river
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
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

Replace the following:

- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the remote_write API.
- `PASSWORD`: The password to use for authentication to the remote_write API.

[scrape]: {{< relref "./prometheus.scrape.md" >}}

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.postgres` has exports that can be consumed by the following components:

- Components that consume [Targets](../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

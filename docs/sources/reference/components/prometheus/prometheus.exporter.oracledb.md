---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.oracledb/
aliases:
  - ../prometheus.exporter.oracledb/ # /docs/alloy/latest/reference/components/prometheus.exporter.oracledb/
description: Learn about prometheus.exporter.oracledb
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.oracledb
---

# `prometheus.exporter.oracledb`

The `prometheus.exporter.oracledb` component embeds
[`oracledb_exporter`](https://github.com/oracle/oracle-db-appdev-monitoring) for collecting statistics from one or more OracleDB servers.

{{< admonition type="warning" >}}
Don't run more than one `prometheus.exporter.oracledb` component in the same {{< param "PRODUCT_NAME" >}} process.
Configure one component with multiple `database` blocks when you scrape more than one database.
{{< /admonition >}}

Ensure you have the following:

- Oracle Instant Client Basic installed on the system running {{< param "PRODUCT_NAME" >}}
- Appropriate environment variables configured for Oracle Client libraries

### Oracle instant client basic

When you run the standalone binary, you must install the [Oracle Instant Client Basic](http://www.oracle.com/technetwork/database/features/instant-client/index-097480.html) for your operating system.
The exporter only requires the basic version.

{{< admonition type="note" >}}
You must also provide Oracle Instant Client Basic when you run {{< param "PRODUCT_NAME" >}} in Docker or Kubernetes.
The `prometheus.exporter.oracledb` component relies on Oracle Instant Client libraries that are available in the container image or host environment.

For macOS on Apple silicon, set `DYLD_LIBRARY_PATH` to the directory where you installed the Oracle Instant Client libraries.
For example `export DYLD_LIBRARY_PATH=/lib/oracle/instantclient_23_3`.
{{< /admonition >}}

### Environment variables

Set the following environment variables for Oracle Client library access:

- **Linux**: Set `LD_LIBRARY_PATH` to the Oracle Instant Client library directory
- **macOS (ARM)**: Set `DYLD_LIBRARY_PATH` to the Oracle Instant Client library directory
- **`ORACLE_BASE`** (optional): Base directory for Oracle installations
- **`ORACLE_HOME`** (optional): Location of the Oracle Instant Client installation
- **`TNS_ADMIN`** (optional): Location of your Oracle wallet directory when using wallet authentication

### Database user permissions

The database user specified in the connection string must have permissions to query Oracle system views.
The user requires the `SELECT_CATALOG_ROLE` role, or `SELECT` privilege on specific system views.

Refer to the [Oracle AI Database Metrics Exporter Installation guide][oracledb_exporter_install] for the complete list of required permissions.

[oracledb_exporter_install]: https://oracle.github.io/oracle-db-appdev-monitoring/docs/getting-started/basics

## Usage

```alloy
prometheus.exporter.oracledb "<LABEL>" {
    connection_string = "<CONNECTION_STRING>"
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.oracledb`:

| Name                | Type           | Description                                                    | Default | Required |
| ------------------- | -------------- | -------------------------------------------------------------- | ------- | -------- |
| `connection_string` | `secret`       | (Deprecated) The connection string used to connect to an Oracle Database. Required when no `database` blocks are set. |         | __See below__ |
| `custom_metrics`    | `list(string)` | The paths to the custom metrics files. (TOML format)           |         | no       |
| `default_metrics`   | `string`       | The path to the default metrics file. (TOML format)            |         | no       |
| `max_idle_conns`    | `int`          | Number of maximum idle connections in the connection pool.     | `0`     | no       |
| `max_open_conns`    | `int`          | Number of maximum open connections in the connection pool.     | `10`    | no       |
| `password`          | `secret`       | (Deprecated) The password to use for authentication to the Oracle Database. |         | no       |
| `query_timeout`     | `int`          | The query timeout in seconds.                                  | `5`     | no       |
| `username`          | `string`       | (Deprecated) The username to use for authentication to the Oracle Database. |         | no       |

{{< admonition type="note" >}}
The top-level `connection_string`, `username` and `password` arguments are deprecated. Use the `database` block instead.

If you keep the deprecated top-level configuration and omit `database` blocks, you must set `connection_string`.
If the URL doesn't embed a username and password, set the deprecated top-level `username` and `password` arguments for that target only.
{{< /admonition >}}

Refer to the [`oracledb_exporter` repository](https://github.com/oracle/oracle-db-appdev-monitoring) for examples of TOML metrics files.

For backward compatibility, you can still provide the `username` and `password` arguments in the `connection_string` argument:

```conn
oracle://user:pass@host:port/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
```

If the `connection_string` argument doesn't contain the `username` and `password`, you must provide the `username` and `password` arguments.
In this case, the URL must have the format:

```conn
host:port/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
```

### Multiple databases

When you scrape several databases from one component, define one or more `database` blocks instead of a top-level `connection_string`.
You must not set both `connection_string` and `database` blocks.

The exporter uses the `name` argument as the database name in metrics, for example in the `database` label.
Each block supports the same connection string rules as the top-level `connection_string`, with optional per-block `username` and `password`.

## Blocks

### `database`

The `database` block configures a single Oracle target when using multi-database mode.

| Name                  | Type                | Description                                                                 | Default | Required |
| --------------------- | ------------------- | --------------------------------------------------------------------------- | ------- | -------- |
| `name`                | `string`            | Unique name for this database target.                                       |         | yes      |
| `connection_string`   | `secret`            | Connection string for this database (same formats as top-level).            |         | yes      |
| `labels`              | `map(string)`       | Optional extra labels applied to metrics from this database.                |         | no       |
| `password`            | `secret`            | Password when not embedded in `connection_string`.                          |         | no       |
| `username`            | `string`            | Username when not embedded in `connection_string`.                          |         | no       |

The `name` argument identifies this database in the exporter configuration and must be unique within the component.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.oracledb` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.oracledb` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.oracledb` doesn't expose any component-specific debug metrics.

## Example

The following example collects metrics from two Oracle databases:

```alloy
prometheus.exporter.oracledb "example" {
  database {
    name = "primary"
    connection_string = "db-primary.example.com:1521/ORCL"
    username            = "<DB_USERNAME>"
    password            = "<DB_PASSWORD>"
  }
  database {
    name = "standby"
    connection_string = "db-standby.example.com:1521/ORCL"
    username            = "<DB_USERNAME>"
    password            = "<DB_PASSWORD>"
  }
}

// Configure a prometheus.scrape component to collect oracledb metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.oracledb.example.targets
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

- _`<DB_USERNAME>`_: The username for the Oracle database.
- _`<DB_PASSWORD>`_: The password for the Oracle database.
- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.oracledb` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

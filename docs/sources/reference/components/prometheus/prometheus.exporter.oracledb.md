---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.oracledb/
aliases:
  - ../prometheus.exporter.oracledb/ # /docs/alloy/latest/reference/components/prometheus.exporter.oracledb/
description: Learn about prometheus.exporter.oracledb
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-10-14
title: prometheus.exporter.oracledb
---

# `prometheus.exporter.oracledb`

The `prometheus.exporter.oracledb` component embeds the [`oracledb_exporter`][oracledb-exporter] to collect statistics from one or more Oracle databases.

You can specify multiple `prometheus.exporter.oracledb` components by giving them different labels.

[oracledb-exporter]: https://github.com/oracle/oracle-db-appdev-monitoring

## Prerequisites

`prometheus.exporter.oracledb` needs the Oracle Client libraries on the system that runs {{< param "PRODUCT_NAME" >}}, and a database user that can read Oracle system views.
You set up both outside {{< param "PRODUCT_NAME" >}} before you configure the component.

### Oracle Instant Client Basic

When you run the standalone binary, you must install the [Oracle Instant Client Basic][instant-client] for your operating system.
The exporter only requires the basic version.

{{< admonition type="note" >}}
{{< param "PRODUCT_NAME" >}} container images don't include the Oracle Instant Client libraries.
When you run {{< param "PRODUCT_NAME" >}} in Docker or Kubernetes, you must add the libraries to the image or make them available to the container.

For macOS on Apple silicon, set `DYLD_LIBRARY_PATH` to the directory where you installed the Oracle Instant Client libraries.
For example `export DYLD_LIBRARY_PATH=/lib/oracle/instantclient_23_3`.
{{< /admonition >}}

[instant-client]: https://www.oracle.com/technetwork/database/features/instant-client/index-097480.html

### Environment variables

Set the following environment variables so {{< param "PRODUCT_NAME" >}} can find the Oracle Client libraries:

- `LD_LIBRARY_PATH`: The Oracle Instant Client library directory on Linux.
- `DYLD_LIBRARY_PATH`: The Oracle Instant Client library directory on macOS.
- `ORACLE_BASE`: The base directory for Oracle installations. Optional.
- `ORACLE_HOME`: The location of the Oracle Instant Client installation. Optional.
- `TNS_ADMIN`: The location of your Oracle wallet directory. Set this when you use wallet authentication. Optional.

### Database user permissions

The database user in the connection string must have permissions to query Oracle system views.
The user requires the `SELECT_CATALOG_ROLE` role, or `SELECT` privilege on specific system views.

Refer to the [Oracle AI Database Metrics Exporter Installation guide][oracledb_exporter_install] for the complete list of required permissions.

[oracledb_exporter_install]: https://oracle.github.io/oracle-db-appdev-monitoring/docs/getting-started/basics

## Usage

```alloy
prometheus.exporter.oracledb "<LABEL>" {
  database {
    name              = "<DATABASE_NAME>"
    connection_string = "<CONNECTION_STRING>"
  }
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.oracledb`:

| Name                | Type           | Description                                                | Default | Required |
| ------------------- | -------------- | ---------------------------------------------------------- | ------- | -------- |
| `connection_string` | `secret`       | Deprecated. Use the `database` block instead.              |         | no       |
| `custom_metrics`    | `list(string)` | The paths to the custom metrics files in TOML format.      |         | no       |
| `default_metrics`   | `string`       | The path to the default metrics file in TOML format.       |         | no       |
| `max_idle_conns`    | `int`          | Maximum number of idle connections in the connection pool. | `0`     | no       |
| `max_open_conns`    | `int`          | Maximum number of open connections in the connection pool. | `10`    | no       |
| `password`          | `secret`       | Deprecated. Use the `database` block instead.              |         | no       |
| `query_timeout`     | `int`          | The query timeout in seconds.                              | `5`     | no       |
| `username`          | `string`       | Deprecated. Use the `database` block instead.              |         | no       |

<!-- vale Grafana.GooglePassive = NO -->
<!-- vale Grafana.GoogleWill = NO -->
<!-- vale Grafana.Timeless = NO -->

{{< admonition type="caution" >}}
Starting with {{< param "PRODUCT_NAME" >}} v1.16, the top-level `connection_string`, `username`, and `password` arguments are deprecated.
They will be removed in a future release.
Use the `database` block instead.

If you keep the deprecated top-level configuration and omit `database` blocks, you must set `connection_string`.
Set the top-level `username` and `password` when `connection_string` omits credentials.
These arguments follow the same connection string rules as the [`database`][database-block] block.

[database-block]: #database
{{< /admonition >}}

<!-- vale Grafana.GooglePassive = YES -->
<!-- vale Grafana.GoogleWill = YES -->
<!-- vale Grafana.Timeless = YES -->

The exporter collects a built-in set of metrics.
Set `default_metrics` to replace the built-in set with your own file.
Set `custom_metrics` to add metrics to the set.
A custom metric replaces a built-in metric when both use the same context and field names.

Refer to the [`oracledb_exporter` repository][oracledb-exporter] for examples of TOML metrics files.

The component sets the `instance` label on its exported targets to the host and port portion of the connection string.
Refer to [Multiple databases][multiple-databases] for the `instance` label value when you configure more than one `database` block.

[multiple-databases]: #multiple-databases

## Blocks

You can use the following blocks with `prometheus.exporter.oracledb`:

{{< docs/alloy-config >}}

| Block                  | Description                              | Required |
| ---------------------- | ---------------------------------------- | -------- |
| [`database`][database] | Configures an Oracle database to scrape. | no       |

[database]: #database

{{< /docs/alloy-config >}}

Configure one or more `database` blocks, or the deprecated `connection_string` argument, but not both.

### `database`

The `database` block configures an Oracle database for the component to scrape.

You can use the following arguments with the `database` block:

| Name                | Type          | Description                                            | Default | Required |
| ------------------- | ------------- | ------------------------------------------------------ | ------- | -------- |
| `connection_string` | `secret`      | Connection string for the Oracle database.             |         | yes      |
| `name`              | `string`      | Unique name for the Oracle database.                   |         | yes      |
| `labels`            | `map(string)` | Extra labels to add to the metrics from this database. |         | no       |
| `password`          | `secret`      | Password when `connection_string` omits credentials.   |         | no       |
| `username`          | `string`      | Username when `connection_string` omits credentials.   |         | no       |

Each `database` block must have a `name` that's unique within the component.
The exporter uses the `name` argument as the value of the `database` label on the metrics it collects from this database.

The `connection_string` argument accepts an `oracle://` URL with the username and password embedded:

```text
oracle://<USERNAME>:<PASSWORD>@<HOST>:<PORT>/<SERVICE_NAME>[?<OPTION_1>=<VALUE_1>[&<OPTION_N>=<VALUE_N>]]
```

It also accepts an address without a scheme, where you set the `username` and `password` arguments separately:

```text
<HOST>:<PORT>/<SERVICE_NAME>[?<OPTION_1>=<VALUE_1>[&<OPTION_N>=<VALUE_N>]]
```

Set `username` and `password` only with the second format.
The component returns an error if you set either argument together with an `oracle://` connection string.

## Multiple databases

Configure a `database` block for each Oracle database you want to scrape from one component.
The component connects to every database you configure and exports a single target for all of them.

When you configure more than one `database` block, no single host identifies the target, so the component sets the `instance` label to the component ID, for example, `prometheus.exporter.oracledb.example`.
Use the `database` label, which the exporter sets to each block's `name`, to tell the metrics from each database apart.

Refer to the [Example][example] for a configuration that scrapes two databases.

[example]: #example

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
    name              = "primary"
    connection_string = "db-primary.example.com:1521/ORCL"
    username          = "<DB_USERNAME>"
    password          = "<DB_PASSWORD>"
  }

  database {
    name              = "standby"
    connection_string = "db-standby.example.com:1521/ORCL"
    username          = "<DB_USERNAME>"
    password          = "<DB_PASSWORD>"
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

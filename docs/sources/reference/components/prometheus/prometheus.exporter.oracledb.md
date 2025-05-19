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
[`oracledb_exporter`](https://github.com/oracle/oracle-db-appdev-monitoring) for collecting statistics from a OracleDB server.

## Usage

```alloy
prometheus.exporter.oracledb "<LABEL>" {
    connection_string = "<CONNECTION_STRING>"
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.oracledb`:

| Name                | Type       | Description                                                    | Default | Required |
| ------------------- | ---------- | -------------------------------------------------------------- | ------- | -------- |
| `connection_string` | `secret`   | The connection string used to connect to an Oracle Database.   |         | yes      |
| `username`          | `string`   | The username to use for authentication to the Oracle Database. |         | no       |
| `password`          | `secret`   | The password to use for authentication to the Oracle Database. |         | no       |
| `max_idle_conns`    | `int`      | Number of maximum idle connections in the connection pool.     | `0`     | no       |
| `max_open_conns`    | `int`      | Number of maximum open connections in the connection pool.     | `10`    | no       |
| `query_timeout`     | `int`      | The query timeout in seconds.                                  | `5`     | no       |
| `default_metrics`   | `string`   | The path to the default metrics file. (TOML format)            |         | no       |
| `custom_metrics`    | `[]string` | The paths to the custom metrics files. (TOML format)           |         | no       |

Examples of TOML metrics files can be found in the [oracledb_exporter repository](https://github.com/oracle/oracle-db-appdev-monitoring).

For backward compatibility, the `username` and `password` arguments can still be provided in the `connection_string` argument:

```conn
oracle://user:pass@host:port/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
```

If the `connection_string` argument doesn't contain the `username` and `password`, the `username` and `password` arguments must be provided.
In this case, the URL must have the format:

```conn
host:port/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
```

## Blocks

The `prometheus.exporter.oracledb` component doesn't support any blocks. You can configure this component with arguments.

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

The following example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.oracledb`:

```alloy
prometheus.exporter.oracledb "example" {
  connection_string = "localhost:1521/orcl.localnet"
  username = "system"
  password = "YourPassword123"
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

* _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
* _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
* _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.oracledb` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

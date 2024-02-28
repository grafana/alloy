---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/prometheus.exporter.oracledb/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/prometheus.exporter.oracledb/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/prometheus.exporter.oracledb/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.exporter.oracledb/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/prometheus.exporter.oracledb/
description: Learn about prometheus.exporter.oracledb
title: prometheus.exporter.oracledb
---

# prometheus.exporter.oracledb

The `prometheus.exporter.oracledb` component embeds
[oracledb_exporter](https://github.com/iamseth/oracledb_exporter) for collecting statistics from a OracleDB server.

## Usage

```river
prometheus.exporter.oracledb "LABEL" {
    connection_string = CONNECTION_STRING
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
Omitted fields take their default values.

| Name                | Type     | Description                                                  | Default | Required |
| ------------------- | -------- | ------------------------------------------------------------ | ------- | -------- |
| `connection_string` | `secret` | The connection string used to connect to an Oracle Database. |         | yes      |
| `max_idle_conns`    | `int`    | Number of maximum idle connections in the connection pool.   | `0`     | no       |
| `max_open_conns`    | `int`    | Number of maximum open connections in the connection pool.   | `10`    | no       |
| `query_timeout`     | `int`    | The query timeout in seconds.                                | `5`     | no       |

[The oracledb_exporter running documentation](https://github.com/iamseth/oracledb_exporter/tree/master#running) shows the format and provides examples of the `connection_string` argument:

```conn
oracle://user:pass@server/service_name[?OPTION1=VALUE1[&OPTIONn=VALUEn]...]
```

## Blocks

The `prometheus.exporter.oracledb` component does not support any blocks, and is configured
fully through arguments.

## Exported fields

{{< docs/shared lookup="flow/reference/components/exporter-component-exports.md" source="agent" version="<AGENT_VERSION>" >}}

## Component health

`prometheus.exporter.oracledb` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.oracledb` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.oracledb` does not expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.oracledb`:

```river
prometheus.exporter.oracledb "example" {
  connection_string = "oracle://user:password@localhost:1521/orcl.localnet"
}

// Configure a prometheus.scrape component to collect oracledb metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.oracledb.example.targets
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

`prometheus.exporter.oracledb` has exports that can be consumed by the following components:

- Components that consume [Targets](../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

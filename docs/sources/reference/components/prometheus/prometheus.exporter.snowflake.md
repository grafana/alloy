---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.snowflake/
aliases:
  - ../prometheus.exporter.snowflake/ # /docs/alloy/latest/reference/components/prometheus.exporter.snowflake/
description: Learn about prometheus.exporter.snowflake
title: prometheus.exporter.snowflake
---

# prometheus.exporter.snowflake

The `prometheus.exporter.snowflake` component embeds
[snowflake_exporter](https://github.com/grafana/snowflake-prometheus-exporter) for collecting warehouse, database, table, and replication statistics from a Snowflake account via HTTP for Prometheus consumption.

## Usage

```alloy
prometheus.exporter.snowflake "LABEL" {
    account_name = ACCOUNT_NAME
    username =     USERNAME
    password =     PASSWORD
    warehouse =    WAREHOUSE
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
Omitted fields take their default values.

| Name           | Type     | Description                                           | Default          | Required |
| -------------- | -------- | ----------------------------------------------------- | ---------------- | -------- |
| `account_name` | `string` | The account to collect metrics for.                   |                  | yes      |
| `username`     | `string` | The username for the user used when querying metrics. |                  | yes      |
| `password`     | `secret` | The password for the user used when querying metrics. |                  | yes      |
| `role`         | `string` | The role to use when querying metrics.                | `"ACCOUNTADMIN"` | no       |
| `warehouse`    | `string` | The warehouse to use when querying metrics.           |                  | yes      |

## Blocks

The `prometheus.exporter.snowflake` component does not support any blocks, and is configured
fully through arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.snowflake` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.snowflake` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.snowflake` does not expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.snowflake`:

```alloy
prometheus.exporter.snowflake "example" {
  account_name = "XXXXXXX-YYYYYYY"
  username     = "grafana"
  password     = "snowflake"
  warehouse    = "examples"
}

// Configure a prometheus.scrape component to collect snowflake metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.snowflake.example.targets
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

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.snowflake` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

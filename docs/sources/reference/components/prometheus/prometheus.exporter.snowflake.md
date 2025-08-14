---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.snowflake/
aliases:
  - ../prometheus.exporter.snowflake/ # /docs/alloy/latest/reference/components/prometheus.exporter.snowflake/
description: Learn about prometheus.exporter.snowflake
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.snowflake
---

# `prometheus.exporter.snowflake`

The `prometheus.exporter.snowflake` component embeds the [`snowflake_exporter`](https://github.com/grafana/snowflake-prometheus-exporter) for collecting warehouse, database, table, and replication statistics from a Snowflake account via HTTP for Prometheus consumption.

## Usage

You can use the `prometheus.exporter.snowflake` component with password or RSA authentication.

### Password Authentication

```alloy
prometheus.exporter.snowflake "LABEL" {
    account_name = "<SNOWFLAKE_ACCOUNT_NAME>"
    username =     "<USERNAME>"
    password =     "<PASSWORD>"
    warehouse =    "<VIRTUAL_WAREHOUSE>"
}
```

### RSA Authentication

```alloy
prometheus.exporter.snowflake "LABEL" {
    account_name =         "<SNOWFLAKE_ACCOUNT_NAME>"
    username =             "<USERNAME>"
    private_key_path =     "<RSA_PRIVATE_KEY_PATH>"
    private_key_password = "<RSA_PRIVATE_KEY_PASSWORD>"
    warehouse =            "<VIRTUAL_WAREHOUSE>"
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.snowflake`:

| Name                       | Type     | Description                                                                                       | Default          | Required |
|----------------------------|----------|---------------------------------------------------------------------------------------------------|------------------|----------|
| `account_name`             | `string` | The account to collect metrics from.                                                              |                  | yes      |
| `username`                 | `string` | The username for the user used when querying metrics.                                             |                  | yes      |
| `warehouse`                | `string` | The warehouse to use when querying metrics.                                                       |                  | yes      |
| `enable_tracing`           | `bool`   | Whether to have the snowflake database driver provide trace logging.                              | `false`          | no       |
| `exclude_deleted_tables`   | `bool`   | Whether to exclude deleted tables when querying table storage metrics.                            | `false`          | no       |
| `password`                 | `secret` | The password for the user used when querying metrics (required for password authentication).      |                  | no       |
| `private_key_password`     | `secret` | The password for the user's RSA private key (required for encrypted RSA key-pair authentication). |                  | no       |
| `private_key_path`         | `secret` | The path to the user's RSA private key file (required for RSA key-pair authentication).           |                  | no       |
| `role`                     | `string` | The role to use when querying metrics.                                                            | `"ACCOUNTADMIN"` | no       |

One of `password` or `private_key_path` must be specified to authenticate.
Users with an encrypted private key will also need to provide a `private_key_password`.

## Blocks

The `prometheus.exporter.snowflake` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.snowflake` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.snowflake` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.snowflake` doesn't expose any component-specific debug metrics.

## Example

The following example uses a [`prometheus.scrape`][scrape] component to collect metrics from `prometheus.exporter.snowflake`:

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
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"

    basic_auth {
      username = "<USERNAME>"
      password = "<PASSWORD>"
    }
  }
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

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

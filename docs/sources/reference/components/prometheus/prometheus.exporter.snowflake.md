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

### Password Authentication

```alloy
prometheus.exporter.snowflake "LABEL" {
    account_name = <ACCOUNT_NAME>
    username =     <USERNAME>
    password =     <PASSWORD>
    warehouse =    <WAREHOUSE>
}
```

Replace the following:

- _`<ACCOUNT_NAME>`_: The Snowflake account name you are collecting metrics from.
- _`<USERNAME>`_: The username used to query metrics.
- _`<PASSWORD>`_: The password for the user used to query metrics.
- _`<WAREHOUSE>`_: The virtual warehouse to use when querying metrics.

### RSA Authentication

```alloy
prometheus.exporter.snowflake "LABEL" {
    account_name =         <ACCOUNT_NAME>
    username =             <USERNAME>
    private_key_path =     <PRIVATE_KEY_PATH>
    private_key_password = <PRIVATE_KEY_PASSWORD>
    warehouse =            <WAREHOUSE>
}
```

Replace the following:

- _`<ACCOUNT_NAME>`_: The Snowflake account name you are collecting metrics from.
- _`<USERNAME>`_: The username used to query metrics.
- _`<PRIVATE_KEY_PATH>`_: The path to the user's RSA private key file.
- _`<PRIVATE_KEY_PASSWORD>`_: The password for the user's RSA private key.
- _`<WAREHOUSE>`_: The virtual warehouse to use when querying metrics.

## Arguments

The following arguments can be used to configure the exporter's behavior.
Omitted fields take their default values.
One of `password` or `private_key_path` must be specified to authenticate.
Users with an encrypted private key will also need to provide a `private_key_password`.

| Name                   | Type     | Description                                                                                       | Default          | Required |
| ---------------------- | -------- | ------------------------------------------------------------------------------------------------- | ---------------- | -------- |
| `account_name`         | `string` | The account to collect metrics from.                                                              |                  | yes      |
| `username`             | `string` | The username for the user used when querying metrics.                                             |                  | yes      |
| `password`             | `secret` | The password for the user used when querying metrics (required for password authentication).      |                  | no       |
| `private_key_path`     | `secret` | The path to the user's RSA private key file (required for RSA key-pair authentication).           |                  | no       |
| `private_key_password` | `secret` | The password for the user's RSA private key (required for encrypted RSA key-pair authentication). |                  | no       |
| `role`                 | `string` | The role to use when querying metrics.                                                            | `"ACCOUNTADMIN"` | no       |
| `warehouse`            | `string` | The warehouse to use when querying metrics.                                                       |                  | yes      |

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
    url = <PROMETHEUS_REMOTE_WRITE_URL>

    basic_auth {
      username = <USERNAME>
      password = <PASSWORD>
    }
  }
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus remote_write-compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the remote_write API.
- _`<PASSWORD>`_: The password to use for authentication to the remote_write API.

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

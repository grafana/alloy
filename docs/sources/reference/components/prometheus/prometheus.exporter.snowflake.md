---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.snowflake/
aliases:
  - ../prometheus.exporter.snowflake/ # /docs/alloy/latest/reference/components/prometheus.exporter.snowflake/
description: Learn about prometheus.exporter.snowflake
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-10-21
title: prometheus.exporter.snowflake
---

# `prometheus.exporter.snowflake`

The `prometheus.exporter.snowflake` component embeds the [`snowflake_exporter`][snowflake-exporter] to collect storage, credit usage, login, warehouse load, automatic clustering, and replication metrics from a Snowflake account.

You can specify multiple `prometheus.exporter.snowflake` components by giving them different labels.

[snowflake-exporter]: https://github.com/grafana/snowflake-prometheus-exporter

## Usage

```alloy
prometheus.exporter.snowflake "<LABEL>" {
  account_name = "<SNOWFLAKE_ACCOUNT_NAME>"
  username     = "<SNOWFLAKE_USERNAME>"
  warehouse    = "<VIRTUAL_WAREHOUSE>"
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.snowflake`:

| Name                     | Type     | Description                                                                                | Default          | Required |
| ------------------------ | -------- | ------------------------------------------------------------------------------------------ | ---------------- | -------- |
| `account_name`           | `string` | The account to collect metrics from.                                                       |                  | yes      |
| `username`               | `string` | The username to use when querying metrics.                                                 |                  | yes      |
| `warehouse`              | `string` | The warehouse to use when querying metrics.                                                |                  | yes      |
| `enable_tracing`         | `bool`   | Enables trace logging in the Snowflake database driver.                                    | `false`          | no       |
| `exclude_deleted_tables` | `bool`   | Excludes deleted tables from table storage metrics and disables the deleted table count.   | `false`          | no       |
| `password`               | `secret` | The password for `username`. Required for password authentication.                         |                  | no       |
| `private_key_password`   | `secret` | The password that decrypts the key in `private_key_path`. Required for encrypted keys.     |                  | no       |
| `private_key_path`       | `string` | The path to the RSA private key file for `username`. Required for key-pair authentication. |                  | no       |
| `role`                   | `string` | The role to use when querying metrics.                                                     | `"ACCOUNTADMIN"` | no       |

Set `password` or `private_key_path` to authenticate.
If you set neither, the component fails with an invalid configuration error.
If you set both, the component uses password authentication and ignores `private_key_path` and `private_key_password`.

The `private_key_path` file must contain an RSA private key in PKCS #8 PEM format.
For an encrypted key, set `private_key_password`.

The role in `role` must be able to read the views in the `SNOWFLAKE.ACCOUNT_USAGE` schema.

The component sets the `instance` label on its exported targets to the value of `account_name`.

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
  account_name = "<SNOWFLAKE_ACCOUNT_NAME>"
  username     = "<SNOWFLAKE_USERNAME>"
  password     = "<SNOWFLAKE_PASSWORD>"
  warehouse    = "<VIRTUAL_WAREHOUSE>"
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

- _`<SNOWFLAKE_ACCOUNT_NAME>`_: The Snowflake account to collect metrics from.
- _`<SNOWFLAKE_USERNAME>`_: The username to use when querying metrics.
- _`<SNOWFLAKE_PASSWORD>`_: The password for _`<SNOWFLAKE_USERNAME>`_.
- _`<VIRTUAL_WAREHOUSE>`_: The Snowflake warehouse to use when querying metrics.
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

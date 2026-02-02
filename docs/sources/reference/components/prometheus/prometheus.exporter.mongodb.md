---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.mongodb/
aliases:
  - ../prometheus.exporter.mongodb/ # /docs/alloy/latest/reference/components/prometheus.exporter.mongodb/
description: Learn about prometheus.exporter.mongodb
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.mongodb
---

# `prometheus.exporter.mongodb`

The `prometheus.exporter.mongodb` component embeds the Percona [`mongodb_exporter`](https://github.com/percona/mongodb_exporter).

{{< admonition type="note" >}}
This exporter doesn't collect metrics from multiple nodes.
For this integration to work properly, you must connect each node of your MongoDB cluster to an {{< param "PRODUCT_NAME" >}} instance.
{{< /admonition >}}

We strongly recommend configuring a separate user for {{< param "PRODUCT_NAME" >}}, giving it only the strictly mandatory security privileges necessary for monitoring your node.
Refer to the [Percona documentation](https://github.com/percona/mongodb_exporter#permissions) for more information.

## Usage

```alloy
prometheus.exporter.mongodb "<LABEL>" {
    mongodb_uri = "<MONGODB_URI>"
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.mongodb`:

| Name                           | Type     | Description                                                                                                                            | Default | Required |
| ------------------------------ | -------- | -------------------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `mongodb_uri`                  | `secret` | MongoDB node connection URI.                                                                                                           |         | yes      |
| `log_level`                    | `string` | Set logging level (debug, info, warn, error)                                                                                           | `info`  | no       |
| `collect_all`                  | `bool`   | Enables all collectors.                                                                                                                | `true`  | no       |
| `compatible_mode`              | `bool`   | Enables metric names compatible with `mongodb_exporter` <v0.20.0.                                                                      | `true`  | no       |
| `direct_connect`               | `bool`   | Whether or not a direct connect should be made. Direct connections aren't valid if multiple hosts are specified or an SRV URI is used. | `false` | no       |
| `discovering_mode`             | `bool`   | Whether or not to enable autodiscover collections.                                                                                     | `false` | no       |
| `enable_coll_stats`            | `bool`   | Enables collecting collection statistics.                                                                                              | `false` | no       |
| `enable_currentop_metrics`     | `bool`   | Enables collecting current operation metrics.                                                                                          | `false` | no       |
| `enable_db_stats_free_storage` | `bool`   | Enables collecting free storage statistics from `dbStats`.                                                                             | `false` | no       |
| `enable_db_stats`              | `bool`   | Enables collecting database statistics.                                                                                                | `false` | no       |
| `enable_diagnostic_data`       | `bool`   | Enables collecting diagnostic data.                                                                                                    | `false` | no       |
| `enable_fcv`                   | `bool`   | Enables collecting Feature Compatibility Version (FCV) metrics.                                                                        | `false` | no       |
| `enable_index_stats`           | `bool`   | Enables collecting index statistics.                                                                                                   | `false` | no       |
| `enable_pbm_metrics`           | `bool`   | Enables collecting Percona Backup for MongoDB (PBM) metrics.                                                                           | `false` | no       |
| `enable_profile`               | `bool`   | Enables collecting profile metrics.                                                                                                    | `false` | no       |
| `enable_replicaset_config`     | `bool`   | Enables collecting replica set configuration.                                                                                          | `false` | no       |
| `enable_replicaset_status`     | `bool`   | Enables collecting replica set status.                                                                                                 | `false` | no       |
| `enable_shards`                | `bool`   | Enables collecting sharding information.                                                                                               | `false` | no       |
| `enable_top_metrics`           | `bool`   | Enables collecting top metrics.                                                                                                        | `false` | no       |

MongoDB node connection URI must be in the [`Standard Connection String Format`](https://docs.mongodb.com/manual/reference/connection-string/#std-label-connections-standard-connection-string-format)

## Blocks

The `prometheus.exporter.mongodb` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.mongodb` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.mongodb` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.mongodb` doesn't expose any component-specific debug metrics.

## Example

The following example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.mongodb`:

```alloy
prometheus.exporter.mongodb "example" {
  mongodb_uri = "mongodb://127.0.0.1:27017"
}

// Configure a prometheus.scrape component to collect MongoDB metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.mongodb.example.targets
  forward_to = [ prometheus.remote_write.default.receiver ]
}

prometheus.remote_write "default" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"
  }
}
```

Replace the following:

- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.mongodb` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.mongodb/
aliases:
  - ../prometheus.exporter.mongodb/ # /docs/alloy/latest/reference/components/prometheus.exporter.mongodb/
description: Learn about prometheus.exporter.mongodb
title: prometheus.exporter.mongodb
---

# prometheus.exporter.mongodb

The `prometheus.exporter.mongodb` component embeds percona's [`mongodb_exporter`](https://github.com/percona/mongodb_exporter).

{{< admonition type="note" >}}
This exporter doesn't collect metrics from multiple nodes. For this integration to work properly, you must have connect each node of your MongoDB cluster to an {{< param "PRODUCT_NAME" >}} instance.
{{< /admonition >}}

We strongly recommend configuring a separate user for {{< param "PRODUCT_NAME" >}}, giving it only the strictly mandatory security privileges necessary for monitoring your node.
Refer to the [Percona documentation](https://github.com/percona/mongodb_exporter#permissions) for more information.

## Usage

```alloy
prometheus.exporter.mongodb "LABEL" {
    mongodb_uri = "MONGODB_URI"
}
```

## Arguments

You can use the following arguments to configure the exporter's behavior.
Omitted fields take their default values.

| Name                         | Type      | Description                                                                                                                             | Default | Required |
| ---------------------------- | --------- | --------------------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `mongodb_uri`                | `string`  | MongoDB node connection URI.                                                                                                            |         | yes      |
| `direct_connect`             | `boolean` | Whether or not a direct connect should be made. Direct connections are not valid if multiple hosts are specified or an SRV URI is used. | false   | no       |
| `discovering_mode`           | `boolean` | Wheter or not to enable autodiscover collections.                                                                                       | false   | no       |
| `tls_basic_auth_config_path` | `string`  | Path to the file having Prometheus TLS config for basic auth. Only enable if you want to use TLS based authentication.                  |         | no       |

MongoDB node connection URI must be in the [`Standard Connection String Format`](https://docs.mongodb.com/manual/reference/connection-string/#std-label-connections-standard-connection-string-format)

For `tls_basic_auth_config_path`, check [`tls_config`](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config) for reference on the file format to be used.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.mongodb` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.mongodb` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.mongodb` does not expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.mongodb`:

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
    url = "REMOTE_WRITE_URL"
  }
}
```

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

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.memcached/
aliases:
  - ../prometheus.exporter.memcached/ # /docs/alloy/latest/reference/components/prometheus.exporter.memcached/
description: Learn about prometheus.exporter.memcached
title: prometheus.exporter.memcached
---

# prometheus.exporter.memcached

The `prometheus.exporter.memcached` component embeds
[memcached_exporter](https://github.com/prometheus/memcached_exporter) for collecting metrics from a Memcached server.

## Usage

```alloy
prometheus.exporter.memcached "LABEL" {
}
```

## Arguments

The following arguments are supported:

| Name      | Type       | Description                                         | Default             | Required |
| --------- | ---------- | --------------------------------------------------- | ------------------- | -------- |
| `address` | `string`   | The Memcached server address.                       | `"localhost:11211"` | no       |
| `timeout` | `duration` | The timeout for connecting to the Memcached server. | `"1s"`              | no       |

## Blocks

The following blocks are supported inside the definition of `prometheus.exporter.memcached`:

| Hierarchy  | Block          | Description                                             | Required |
| ---------- | -------------- | ------------------------------------------------------- | -------- |
| tls_config | [tls_config][] | TLS configuration for requests to the Memcached server. | no       |

[tls_config]: #tls_config-block

### tls_config block

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.memcached` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.memcached` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.memcached` does not expose any component-specific
debug metrics.

## Example

This example uses a `prometheus.exporter.memcached` component to collect metrics from a Memcached
server running locally, and scrapes the metrics using a [prometheus.scrape][scrape] component:

```alloy
prometheus.exporter.memcached "example" {
  address = "localhost:13321"
  timeout = "5s"
}

prometheus.scrape "example" {
  targets    = [prometheus.exporter.memcached.example.targets]
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

`prometheus.exporter.memcached` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

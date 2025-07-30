---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.consul/
aliases:
  - ../prometheus.exporter.consul/ # /docs/alloy/latest/reference/components/prometheus.exporter.consul/
description: Learn about prometheus.exporter.consul
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.exporter.consul
---

# `prometheus.exporter.consul`

The `prometheus.exporter.consul` component embeds the [`consul_exporter`](https://github.com/prometheus/consul_exporter) for collecting metrics from a Consul installation.

## Usage

```alloy
prometheus.exporter.consul "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.consul`:

| Name                       | Type       | Description                                                                                                                                                           | Default                   | Required |
| -------------------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------- | -------- |
| `allow_stale`              | `bool`     | Allows any Consul server (non-leader) to service a read.                                                                                                              | `true`                    | no       |
| `ca_file`                  | `string`   | File path to a PEM-encoded certificate authority used to validate the authenticity of a server certificate.                                                           |                           | no       |
| `cert_file`                | `string`   | File path to a PEM-encoded certificate used with the private key to verify the exporter's authenticity.                                                               |                           | no       |
| `concurrent_request_limit` | `string`   | Limit the maximum number of concurrent requests to consul, 0 means no limit.                                                                                          |                           | no       |
| `generate_health_summary`  | `bool`     | Collects information about each registered service and exports `consul_catalog_service_node_healthy`.                                                                 | `true`                    | no       |
| `insecure_skip_verify`     | `bool`     | Disable TLS host verification.                                                                                                                                        | `false`                   | no       |
| `key_file`                 | `string`   | File path to a PEM-encoded private key used with the certificate to verify the exporter's authenticity.                                                               |                           | no       |
| `kv_filter`                | `string`   | Only store keys that match this regular expression pattern.                                                                                                           | `".*"`                    | no       |
| `kv_prefix`                | `string`   | Prefix under which to look for KV pairs.                                                                                                                              |                           | no       |
| `require_consistent`       | `bool`     | Forces the read to be fully consistent.                                                                                                                               |                           | no       |
| `server_name`              | `string`   | When provided, this overrides the hostname for the TLS certificate. It can be used to ensure that the certificate name matches the hostname you declare.              |                           | no       |
| `server`                   | `string`   | Address (host and port) of the Consul instance to connect to. This could be a local {{< param "PRODUCT_NAME" >}} (localhost:8500), or the address of a Consul server. | `"http://localhost:8500"` | no       |
| `timeout`                  | `duration` | Timeout on HTTP requests to consul.                                                                                                                                   | `"500ms"`                 | no       |

## Blocks

The `prometheus.exporter.consul` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.consul` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.consul` doesn't expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.consul` doesn't expose any component-specific
debug metrics.

## Example

The following example uses a [`prometheus.scrape`][scrape] component to collect metrics from `prometheus.exporter.consul`:

```alloy
prometheus.exporter.consul "example" {
  server = "https://consul.example.com:8500"
}

// Configure a prometheus.scrape component to collect consul metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.consul.example.targets
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

`prometheus.exporter.consul` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.consul/
aliases:
  - ../prometheus.exporter.consul/ # /docs/alloy/latest/reference/components/prometheus.exporter.consul/
description: Learn about prometheus.exporter.consul
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-10-01
title: prometheus.exporter.consul
---

# `prometheus.exporter.consul`

The `prometheus.exporter.consul` component embeds the [`consul_exporter`](https://github.com/prometheus/consul_exporter) to collect metrics from a Consul cluster.

You can specify multiple `prometheus.exporter.consul` components by giving them different labels.

## Usage

```alloy
prometheus.exporter.consul "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.consul`:

| Name                       | Type       | Description                                                                                           | Default                   | Required |
| -------------------------- | ---------- | ----------------------------------------------------------------------------------------------------- | ------------------------- | -------- |
| `allow_stale`              | `bool`     | Allows any Consul server, including non-leaders, to service a read.                                   | `true`                    | no       |
| `ca_file`                  | `string`   | Path to the certificate authority that validates the Consul server's certificate.                     |                           | no       |
| `cert_file`                | `string`   | Path to the client certificate. Requires `key_file`.                                                  |                           | no       |
| `concurrent_request_limit` | `int`      | Limits the number of concurrent requests to Consul. `0` means no limit.                               | `0`                       | no       |
| `generate_health_summary`  | `bool`     | Collects information about each registered service and exports `consul_catalog_service_node_healthy`. | `true`                    | no       |
| `insecure_skip_verify`     | `bool`     | Disables TLS host verification.                                                                       | `false`                   | no       |
| `key_file`                 | `string`   | Path to the client private key. Requires `cert_file`.                                                 |                           | no       |
| `kv_filter`                | `string`   | Exports only keys that match this regular expression pattern.                                         | `".*"`                    | no       |
| `kv_prefix`                | `string`   | Prefix to search for KV pairs. Required to collect KV metrics.                                        |                           | no       |
| `require_consistent`       | `bool`     | Forces the read to be fully consistent.                                                               | `false`                   | no       |
| `server`                   | `string`   | Address of the Consul agent or server to connect to.                                                  | `"http://localhost:8500"` | no       |
| `server_name`              | `string`   | Overrides the hostname used to verify the TLS certificate.                                            |                           | no       |
| `timeout`                  | `duration` | Timeout on HTTP requests to Consul.                                                                   | `"500ms"`                 | no       |

The `server` argument accepts an address with or without a scheme.
If you omit the scheme, the component adds `http://`.
The address must include a host, and the scheme must be `http` or `https`.

Set `ca_file` to validate the Consul server's certificate.
The component falls back to the system certificate bundle when you don't set `ca_file`.
Use `server_name` when the hostname you connect to doesn't match the name in the server's certificate.

Set `cert_file` and `key_file` together to authenticate the component to Consul with a client certificate.
Set `insecure_skip_verify` to `true` to disable TLS host verification.
Use this setting only in development.

Consul access control list tokens come from the environment rather than from an argument.
Set `CONSUL_HTTP_TOKEN` or `CONSUL_HTTP_TOKEN_FILE` before you start {{< param "PRODUCT_NAME" >}} to authenticate against a cluster that uses access control lists.

The `allow_stale` and `require_consistent` arguments select the read consistency mode.
The component sends both settings to Consul without validating them, so set `allow_stale` to `false` when you set `require_consistent` to `true`.

The component collects KV metrics only when you set `kv_prefix`.
The `kv_filter` argument then selects which keys under that prefix to export.
The component exports only the values it can parse as numbers.

## Blocks

The `prometheus.exporter.consul` component doesn't support any blocks. You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.consul` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.consul` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.consul` doesn't expose any component-specific debug metrics.

## Examples

The following examples demonstrate basic metric collection and metric collection with custom TLS certificates.

### Collect metrics from Consul

The following example uses a [`prometheus.scrape`][scrape] component to collect metrics from `prometheus.exporter.consul`:

```alloy
prometheus.exporter.consul "example" {
  server = "https://consul.example.com:8500"
}

// Configure a prometheus.scrape component to collect Consul metrics.
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

### Collect metrics with custom TLS certificates

The following example validates the Consul server's certificate with a private certificate authority and authenticates with a client certificate:

```alloy
prometheus.exporter.consul "example" {
  server    = "https://consul.example.com:8500"
  ca_file   = "/etc/alloy/consul-ca.pem"
  cert_file = "/etc/alloy/consul-client.pem"
  key_file  = "/etc/alloy/consul-client-key.pem"
}

// Configure a prometheus.scrape component to collect Consul metrics.
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

---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/prometheus.exporter.dnsmasq/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/prometheus.exporter.dnsmasq/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/prometheus.exporter.dnsmasq/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.exporter.dnsmasq/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/prometheus.exporter.dnsmasq/
description: Learn about prometheus.exporter.dnsmasq
title: prometheus.exporter.dnsmasq
---

# prometheus.exporter.dnsmasq

The `prometheus.exporter.dnsmasq` component embeds
[dnsmasq_exporter](https://github.com/google/dnsmasq_exporter) for collecting statistics from a dnsmasq server.

## Usage

```river
prometheus.exporter.dnsmasq "LABEL" {
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
All arguments are optional. Omitted fields take their default values.

| Name            | Type     | Description                                          | Default                          | Required |
| --------------- | -------- | ---------------------------------------------------- | -------------------------------- | -------- |
| `address`       | `string` | The address of the dnsmasq server.                   | `"localhost:53"`                 | no       |
| `leases_file`   | `string` | The path to the dnsmasq leases file.                 | `"/var/lib/misc/dnsmasq.leases"` | no       |
| `expose_leases` | `bool`   | Expose dnsmasq leases as metrics (high cardinality). | `false`                          | no       |

## Exported fields

{{< docs/shared lookup="flow/reference/components/exporter-component-exports.md" source="agent" version="<AGENT_VERSION>" >}}

## Component health

`prometheus.exporter.dnsmasq` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.dnsmasq` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.dnsmasq` does not expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.dnsmasq`:

```river
prometheus.exporter.dnsmasq "example" {
  address = "localhost:53"
}

// Configure a prometheus.scrape component to collect github metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.dnsmasq.example.targets
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

`prometheus.exporter.dnsmasq` has exports that can be consumed by the following components:

- Components that consume [Targets](../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
aliases:
- /docs/grafana-cloud/agent/flow/reference/components/prometheus.exporter.vsphere/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/components/prometheus.exporter.vsphere/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/components/prometheus.exporter.vsphere/
- /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.exporter.vsphere/
canonical: https://grafana.com/docs/agent/latest/flow/reference/components/prometheus.exporter.vsphere/
title: prometheus.exporter.vsphere
description: Learn about prometheus.exporter.vsphere
---

# prometheus.exporter.vsphere

The `prometheus.exporter.vsphere` component embeds [`vmware_exporter`](https://github.com/grafana/vmware_exporter) to collect vSphere metrics

> **NOTE**: We recommend to use [otelcol.receiver.vcenter][] instead.

[otelcol.receiver.vcenter]: {{< relref "./otelcol.receiver.vcenter.md" >}}


## Usage

```river
prometheus.exporter.vsphere "LABEL" {
}
```

## Arguments

You can use the following arguments to configure the exporter's behavior.
Omitted fields take their default values.

| Name                         | Type      | Description                                                                                                                             | Default | Required |
| ---------------------------- | --------- | --------------------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `vsphere_url`                | `string`  | The url of the vCenter endpoint SDK     |         | no      |
| `vsphere_user`             | `string` | vCenter username. |    | no       |
| `vsphere_password`           | `secret` | vCenter password.   |    | no       |
| `request_chunk_size`         | `int`     | Number of managed objects to include in each request to vsphere when fetching performance counters.                                     | `256`   | no       |
| `collect_concurrency`        | `int`     | Number of concurrent requests to vSphere when fetching performance counters.                                                           | `8`     | no       |
| `discovery_interval` | `duration` | Interval on which to run vSphere managed object discovery. | `0` | no |
| `enable_exporter_metrics` | `boolean` | Enable the exporter metrics. | `true` | no |

-  Setting `discovery_interval` to a non-zero value will result in object discovery running in the background. Each scrape will use object data gathered during the last discovery. When this value is 0, object discovery occurs per scrape.


## Exported fields

{{< docs/shared lookup="flow/reference/components/exporter-component-exports.md" source="agent" version="<AGENT_VERSION>" >}}

## Component health

`prometheus.exporter.vsphere` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last
healthy values.

## Debug information

`prometheus.exporter.vsphere` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.vsphere` does not expose any component-specific
debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics
from `prometheus.exporter.vsphere`:

```river
prometheus.exporter.vsphere "example" {
    vsphere_url      = "https://127.0.0.1:8989/sdk"
    vsphere_user     = "user"
    vsphere_password = "pass"
}

// Configure a prometheus.scrape component to collect vsphere metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.vsphere.example.targets
  forward_to = [ prometheus.remote_write.default.receiver ]
}

prometheus.remote_write "default" {
  endpoint {
    url = "REMOTE_WRITE_URL"
  }
}
```

[scrape]: {{< relref "./prometheus.scrape.md" >}}

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.vsphere` has exports that can be consumed by the following components:

- Components that consume [Targets](../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

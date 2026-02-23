---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.vcenter/
aliases:
  - ../otelcol.receiver.vcenter/ # /docs/alloy/latest/reference/otelcol.receiver.vcenter/
title: otelcol.receiver.vcenter
labels:
  stage: experimental
  products:
    - oss
description: Learn about otelcol.receiver.vcenter
---

# `otelcol.receiver.vcenter`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.vcenter` accepts metrics from a vCenter or ESXi host running VMware vSphere APIs and forwards it to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.vcenter` is a wrapper over the upstream OpenTelemetry Collector [`vcenter`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`vcenter`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/vcenterreceiver
{{< /admonition >}}

You can specify multiple `otelcol.receiver.vcenter` components by giving them different labels.

The full list of metrics that can be collected can be found in [vcenter receiver documentation][vcenter metrics].

[vcenter metrics]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/receiver/vcenterreceiver/documentation.md

## Prerequisites

This receiver has been built to support ESXi and vCenter versions:

* 8
* 7.0

A “Read Only” user assigned to a vSphere with permissions to the vCenter server, cluster, and all subsequent resources being monitored must be specified in order for the receiver to retrieve information about them.

## Usage

```alloy
otelcol.receiver.vcenter "<LABEL>" {
  endpoint = "<VCENTER_ENDPOINT>"
  username = "<VCENTER_USERNAME>"
  password = "<VCENTER_PASSWORD>"

  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.vcenter`:

| Name                  | Type       | Description                                                               | Default | Required |
|-----------------------|------------|---------------------------------------------------------------------------|---------|----------|
| `endpoint`            | `string`   | Endpoint to a vCenter Server or ESXi host which has the SDK path enabled. |         | yes      |
| `username`            | `string`   | Username to use for authentication.                                       |         | yes      |
| `password`            | `string`   | Password to use for authentication.                                       |         | yes      |
| `collection_interval` | `duration` | Defines how often to collect metrics.                                     | `"1m"`  | no       |
| `initial_delay`       | `duration` | Defines how long this receiver waits before starting.                     | `"1s"`  | no       |
| `timeout`             | `duration` | Defines the timeout for the underlying HTTP client.                       | `"0s"`  | no       |

`endpoint` has the format `<protocol>://<hostname>`. For example, `https://vcsa.hostname.localnet`.

## Blocks

You can use the following blocks with `otelcol.receiver.vcenter`:

| Block                                        | Description                                                                | Required |
|----------------------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]                           | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics]             | Configures the metrics that this component generates to monitor its state. | no       |
| [`metrics`][metrics]                         | Configures which metrics will be sent to downstream components.            | no       |
| [`resource_attributes`][resource_attributes] | Configures resource attributes for metrics sent to downstream components.  | no       |
| [`tls`][tls]                                 | Configures TLS for the HTTP client.                                        | no       |
| `tls` > [`tpm`][tpm]                         | Configures TPM settings for the TLS key_file.                              | no       |

The > symbol indicates deeper levels of nesting.
For example, `tls` > `tpm` refers to a `tpm` block defined inside a `tls` block.

[tls]: #tls
[tpm]: #tpm
[debug_metrics]: #debug_metrics
[metrics]: #metrics
[resource_attributes]: #resource_attributes
[output]: #output

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `metrics`

| Name                                       | Type         | Description                                                    | Default | Required |
|--------------------------------------------|--------------|----------------------------------------------------------------|---------|----------|
| `vcenter.cluster.cpu.effective`            | [`metric`][] | Enables the `vcenter.cluster.cpu.effective` metric.            | `true`  | no       |
| `vcenter.cluster.cpu.limit`                | [`metric`][] | Enables the `vcenter.cluster.cpu.limit` metric.                | `true`  | no       |
| `vcenter.cluster.host.count`               | [`metric`][] | Enables the `vcenter.cluster.host.count` metric.               | `true`  | no       |
| `vcenter.cluster.memory.effective`         | [`metric`][] | Enables the `vcenter.cluster.memory.effective` metric.         | `true`  | no       |
| `vcenter.cluster.memory.limit`             | [`metric`][] | Enables the `vcenter.cluster.memory.limit` metric.             | `true`  | no       |
| `vcenter.cluster.vm_template.count`        | [`metric`][] | Enables the `vcenter.cluster.vm_template.count` metric.        | `true`  | no       |
| `vcenter.cluster.vm.count`                 | [`metric`][] | Enables the `vcenter.cluster.vm.count` metric.                 | `true`  | no       |
| `vcenter.cluster.vsan.congestions`         | [`metric`][] | Enables the `vcenter.cluster.vsan.congestions` metric.         | `true`  | no       |
| `vcenter.cluster.vsan.latency.avg`         | [`metric`][] | Enables the `vcenter.cluster.vsan.latency.avg` metric.         | `true`  | no       |
| `vcenter.cluster.vsan.operations`          | [`metric`][] | Enables the `vcenter.cluster.vsan.operations` metric.          | `true`  | no       |
| `vcenter.cluster.vsan.throughput`          | [`metric`][] | Enables the `vcenter.cluster.vsan.throughput` metric.          | `true`  | no       |
| `vcenter.datacenter.cluster.count`         | [`metric`][] | Enables the `vcenter.datacenter.cluster.count` metric.         | `true`  | no       |
| `vcenter.datacenter.cpu.limit`             | [`metric`][] | Enables the `vcenter.datacenter.cpu.limit` metric.             | `true`  | no       |
| `vcenter.datacenter.datastore.count`       | [`metric`][] | Enables the `vcenter.datacenter.datastore.count` metric.       | `true`  | no       |
| `vcenter.datacenter.disk.space`            | [`metric`][] | Enables the `vcenter.datacenter.disk.space` metric.            | `true`  | no       |
| `vcenter.datacenter.host.count`            | [`metric`][] | Enables the `vcenter.datacenter.host.count` metric.            | `true`  | no       |
| `vcenter.datacenter.memory.limit`          | [`metric`][] | Enables the `vcenter.datacenter.memory.limit` metric.          | `true`  | no       |
| `vcenter.datacenter.vm.count`              | [`metric`][] | Enables the `vcenter.datacenter.vm.count` metric.              | `true`  | no       |
| `vcenter.datastore.disk.usage`             | [`metric`][] | Enables the `vcenter.datastore.disk.usage` metric.             | `true`  | no       |
| `vcenter.datastore.disk.utilization`       | [`metric`][] | Enables the `vcenter.datastore.disk.utilization` metric.       | `true`  | no       |
| `vcenter.host.cpu.capacity`                | [`metric`][] | Enables the `vcenter.host.cpu.capacity` metric.                | `true`  | no       |
| `vcenter.host.cpu.reserved`                | [`metric`][] | Enables the `vcenter.host.cpu.reserved` metric.                | `true`  | no       |
| `vcenter.host.cpu.usage`                   | [`metric`][] | Enables the `vcenter.host.cpu.usage` metric.                   | `true`  | no       |
| `vcenter.host.cpu.utilization`             | [`metric`][] | Enables the `vcenter.host.cpu.utilization` metric.             | `true`  | no       |
| `vcenter.host.disk.latency.avg`            | [`metric`][] | Enables the `vcenter.host.disk.latency.avg` metric.            | `true`  | no       |
| `vcenter.host.disk.latency.max`            | [`metric`][] | Enables the `vcenter.host.disk.latency.max` metric.            | `true`  | no       |
| `vcenter.host.disk.throughput`             | [`metric`][] | Enables the `vcenter.host.disk.throughput` metric.             | `true`  | no       |
| `vcenter.host.memory.capacity`             | [`metric`][] | Enables the `vcenter.host.memory.capacity` metric.             | `false` | no       |
| `vcenter.host.memory.usage`                | [`metric`][] | Enables the `vcenter.host.memory.usage` metric.                | `true`  | no       |
| `vcenter.host.memory.utilization`          | [`metric`][] | Enables the `vcenter.host.memory.utilization` metric.          | `true`  | no       |
| `vcenter.host.network.packet.drop.rate`    | [`metric`][] | Enables the `vcenter.host.network.packet.drop.rate` metric.    | `true`  | no       |
| `vcenter.host.network.packet.error.rate`   | [`metric`][] | Enables the `vcenter.host.network.packet.error.rate` metric.   | `true`  | no       |
| `vcenter.host.network.packet.rate`         | [`metric`][] | Enables the `vcenter.host.network.packet.rate` metric.         | `true`  | no       |
| `vcenter.host.network.throughput`          | [`metric`][] | Enables the `vcenter.host.network.throughput` metric.          | `true`  | no       |
| `vcenter.host.network.usage`               | [`metric`][] | Enables the `vcenter.host.network.usage` metric.               | `true`  | no       |
| `vcenter.host.vsan.cache.hit_rate`         | [`metric`][] | Enables the `vcenter.host.vsan.cache.hit_rate` metric.         | `true`  | no       |
| `vcenter.host.vsan.congestions`            | [`metric`][] | Enables the `vcenter.host.vsan.congestions` metric.            | `true`  | no       |
| `vcenter.host.vsan.latency.avg`            | [`metric`][] | Enables the `vcenter.host.vsan.latency.avg` metric.            | `true`  | no       |
| `vcenter.host.vsan.operations`             | [`metric`][] | Enables the `vcenter.host.vsan.operations` metric.             | `true`  | no       |
| `vcenter.host.vsan.throughput`             | [`metric`][] | Enables the `vcenter.host.vsan.throughput` metric.             | `true`  | no       |
| `vcenter.resource_pool.cpu.shares`         | [`metric`][] | Enables the `vcenter.resource_pool.cpu.shares` metric.         | `true`  | no       |
| `vcenter.resource_pool.cpu.usage`          | [`metric`][] | Enables the `vcenter.resource_pool.cpu.usage` metric.          | `true`  | no       |
| `vcenter.resource_pool.memory.ballooned`   | [`metric`][] | Enables the `vcenter.resource_pool.memory.ballooned` metric.   | `true`  | no       |
| `vcenter.resource_pool.memory.granted`     | [`metric`][] | Enables the `vcenter.resource_pool.memory.granted` metric.     | `true`  | no       |
| `vcenter.resource_pool.memory.shares`      | [`metric`][] | Enables the `vcenter.resource_pool.memory.shares` metric.      | `true`  | no       |
| `vcenter.resource_pool.memory.swapped`     | [`metric`][] | Enables the `vcenter.resource_pool.memory.swapped` metric.     | `true`  | no       |
| `vcenter.resource_pool.memory.usage`       | [`metric`][] | Enables the `vcenter.resource_pool.memory.usage` metric.       | `true`  | no       |
| `vcenter.vm.cpu.readiness`                 | [`metric`][] | Enables the `vcenter.vm.cpu.readiness` metric.                 | `true`  | no       |
| `vcenter.vm.cpu.time`                      | [`metric`][] | Enables the `vcenter.vm.cpu.time` metric.                      | `false` | no       |
| `vcenter.vm.cpu.usage`                     | [`metric`][] | Enables the `vcenter.vm.cpu.usage` metric.                     | `true`  | no       |
| `vcenter.vm.cpu.utilization`               | [`metric`][] | Enables the `vcenter.vm.cpu.utilization` metric.               | `true`  | no       |
| `vcenter.vm.disk.latency.avg`              | [`metric`][] | Enables the `vcenter.vm.disk.latency.avg` metric.              | `true`  | no       |
| `vcenter.vm.disk.latency.max`              | [`metric`][] | Enables the `vcenter.vm.disk.latency.max` metric.              | `true`  | no       |
| `vcenter.vm.disk.throughput`               | [`metric`][] | Enables the `vcenter.vm.disk.throughput` metric.               | `true`  | no       |
| `vcenter.vm.disk.usage`                    | [`metric`][] | Enables the `vcenter.vm.disk.usage` metric.                    | `true`  | no       |
| `vcenter.vm.disk.utilization`              | [`metric`][] | Enables the `vcenter.vm.disk.utilization` metric.              | `true`  | no       |
| `vcenter.vm.memory.ballooned`              | [`metric`][] | Enables the `vcenter.vm.memory.ballooned` metric.              | `true`  | no       |
| `vcenter.vm.memory.granted`                | [`metric`][] | Enables the `vcenter.vm.memory.granted` metric.                | `false` | no       |
| `vcenter.vm.memory.swapped_ssd`            | [`metric`][] | Enables the `vcenter.vm.memory.swapped_ssd` metric.            | `true`  | no       |
| `vcenter.vm.memory.swapped`                | [`metric`][] | Enables the `vcenter.vm.memory.swapped` metric.                | `true`  | no       |
| `vcenter.vm.memory.usage`                  | [`metric`][] | Enables the `vcenter.vm.memory.usage` metric.                  | `true`  | no       |
| `vcenter.vm.memory.utilization`            | [`metric`][] | Enables the `vcenter.vm.memory.utilization` metric.            | `true`  | no       |
| `vcenter.vm.network.broadcast.packet.rate` | [`metric`][] | Enables the `vcenter.vm.network.broadcast.packet.rate` metric. | `false` | no       |
| `vcenter.vm.network.multicast.packet.rate` | [`metric`][] | Enables the `vcenter.vm.network.multicast.packet.rate` metric. | `false` | no       |
| `vcenter.vm.network.packet.drop.rate`      | [`metric`][] | Enables the `vcenter.vm.network.packet.drop.rate` metric.      | `true`  | no       |
| `vcenter.vm.network.packet.rate`           | [`metric`][] | Enables the `vcenter.vm.network.packet.rate` metric.           | `true`  | no       |
| `vcenter.vm.network.throughput`            | [`metric`][] | Enables the `vcenter.vm.network.throughput` metric.            | `true`  | no       |
| `vcenter.vm.network.usage`                 | [`metric`][] | Enables the `vcenter.vm.network.usage` metric.                 | `true`  | no       |
| `vcenter.vm.vsan.latency.avg`              | [`metric`][] | Enables the `vcenter.vm.vsan.latency.avg` metric.              | `true`  | no       |
| `vcenter.vm.vsan.operations`               | [`metric`][] | Enables the `vcenter.vm.vsan.operations` metric.               | `true`  | no       |
| `vcenter.vm.vsan.throughput`               | [`metric`][] | Enables the `vcenter.vm.vsan.throughput` metric.               | `true`  | no       |

[`metric`]: #metric

#### `metric`

| Name      | Type      | Description                   | Default | Required |
|-----------|-----------|-------------------------------|---------|----------|
| `enabled` | `boolean` | Whether to enable the metric. | `true`  | no       |

### `resource_attributes`

| Name                                   | Type                     | Description                                                            | Default | Required |
|----------------------------------------|--------------------------|------------------------------------------------------------------------|---------|----------|
| `vcenter.datacenter.name`              | [`resource_attribute`][] | Enables the `vcenter.datacenter.name` resource attribute.              | `true`  | no       |
| `vcenter.cluster.name`                 | [`resource_attribute`][] | Enables the `vcenter.cluster.name` resource attribute.                 | `true`  | no       |
| `vcenter.datastore.name`               | [`resource_attribute`][] | Enables the `vcenter.cluster.resource_pool` resource attribute.        | `true`  | no       |
| `vcenter.host.name`                    | [`resource_attribute`][] | Enables the `vcenter.host.name` resource attribute.                    | `true`  | no       |
| `vcenter.resource_pool.inventory_path` | [`resource_attribute`][] | Enables the `vcenter.resource_pool.inventory_path` resource attribute. | `true`  | no       |
| `vcenter.resource_pool.name`           | [`resource_attribute`][] | Enables the `vcenter.resource_pool.name` resource attribute.           | `true`  | no       |
| `vcenter.virtual_app.inventory_path`   | [`resource_attribute`][] | Enables the `vcenter.virtual_app.inventory_path` resource attribute.   | `true`  | no       |
| `vcenter.virtual_app.name`             | [`resource_attribute`][] | Enables the `vcenter.virtual_app.name` resource attribute.             | `true`  | no       |
| `vcenter.vm.id`                        | [`resource_attribute`][] | Enables the `vcenter.vm.id` resource attribute.                        | `true`  | no       |
| `vcenter.vm.name`                      | [`resource_attribute`][] | Enables the `vcenter.vm.name` resource attribute.                      | `true`  | no       |
| `vcenter.vm_template.id`               | [`resource_attribute`][] | Enables the `vcenter.vm_template.id` resource attribute.               | `true`  | no       |
| `vcenter.vm_template.name`             | [`resource_attribute`][] | Enables the `vcenter.vm_template.name` resource attribute.             | `true`  | no       |

[`resource_attribute`]: #resource_attribute

#### `resource_attribute`

| Name      | Type      | Description                               | Default | Required |
|-----------|-----------|-------------------------------------------|---------|----------|
| `enabled` | `boolean` | Whether to enable the resource attribute. | `true`  | no       |

### `tls`

The `tls` block configures TLS settings used for a server. If the `tls` block
isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.vcenter` doesn't export any fields.

## Component health

`otelcol.receiver.vcenter` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.vcenter` doesn't expose any component-specific debug information.

## Example

This example forwards received telemetry data through a batch processor before finally sending it to an OTLP-capable endpoint:

```alloy
otelcol.receiver.vcenter "default" {
  endpoint = "http://localhost:15672"
  username = "otelu"
  password = "password"

  output {
    metrics = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    metrics = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.vcenter` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

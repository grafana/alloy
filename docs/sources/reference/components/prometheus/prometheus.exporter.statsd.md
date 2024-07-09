---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.statsd/
aliases:
  - ../prometheus.exporter.statsd/ # /docs/alloy/latest/reference/components/prometheus.exporter.statsd/
description: Learn about prometheus.exporter.statsd
title: prometheus.exporter.statsd
---

# prometheus.exporter.statsd

The `prometheus.exporter.statsd` component embeds
[statsd_exporter](https://github.com/prometheus/statsd_exporter) for collecting StatsD-style metrics and exporting them as Prometheus metrics.

## Usage

```alloy
prometheus.exporter.statsd "LABEL" {
}
```

## Arguments

The following arguments can be used to configure the exporter's behavior.
All arguments are optional. Omitted fields take their default values.

| Name                    | Type     | Description                                                                                                              | Default | Required |
| ----------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------ | ------- | -------- |
| `listen_udp`            | `string` | The UDP address on which to receive statsd metric lines. Use "" to disable it.                                           | `:9125` | no       |
| `listen_tcp`            | `string` | The TCP address on which to receive statsd metric lines. Use "" to disable it.                                           | `:9125` | no       |
| `listen_unixgram`       | `string` | The Unixgram socket path to receive statsd metric lines in datagram. Use "" to disable it.                               |         | no       |
| `unix_socket_mode`      | `string` | The permission mode of the unix socket.                                                                                  | `755`   | no       |
| `mapping_config_path`   | `string` | The path to a YAML mapping file used to translate specific dot-separated StatsD metrics into labeled Prometheus metrics. |         | no       |
| `read_buffer`           | `int`    | Size (in bytes) of the operating system's transmit read buffer associated with the UDP or Unixgram connection.           |         | no       |
| `cache_size`            | `int`    | Maximum size of your metric mapping cache. Relies on least recently used replacement policy if max size is reached.      | `1000`  | no       |
| `cache_type`            | `string` | Metric mapping cache type. Valid options are "lru" and "random".                                                         | `lru`   | no       |
| `event_queue_size`      | `int`    | Size of internal queue for processing events.                                                                            | `10000` | no       |
| `event_flush_threshold` | `int`    | Number of events to hold in queue before flushing.                                                                       | `1000`  | no       |
| `event_flush_interval`  | `string` | Maximum time between event queue flushes.                                                                                | `200ms` | no       |
| `parse_dogstatsd_tags`  | `string` | Parse DogStatsd style tags.                                                                                              | `true`  | no       |
| `parse_influxdb_tags`   | `string` | Parse InfluxDB style tags.                                                                                               | `true`  | no       |
| `parse_librato_tags`    | `string` | Parse Librato style tags.                                                                                                | `true`  | no       |
| `parse_signalfx_tags`   | `string` | Parse SignalFX style tags.                                                                                               | `true`  | no       |
| `relay_addr`            | `string` | Relay address configuration (UDP endpoint in the format 'host:port').                                                    |         | no       |
| `relay_packet_length`   | `int`    | Maximum relay output packet length to avoid fragmentation.                                                               | `1400`  | no       |

At least one of `listen_udp`, `listen_tcp`, or `listen_unixgram` should be enabled.
Refer to the [`statsd_exporter` documentation](https://github.com/prometheus/statsd_exporter#metric-mapping-and-configuration) more information about the mapping `config file`.
Make sure the kernel parameter `net.core.rmem_max` is set to a value greater than the value specified in `read_buffer`.

### Blocks

The `prometheus.exporter.statsd` component doesn't support any blocks, and is configured fully through arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.statsd` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.statsd` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.statsd` doesn't expose any component-specific debug metrics.

## Example

This example uses a [`prometheus.scrape` component][scrape] to collect metrics from `prometheus.exporter.statsd`:

```alloy
prometheus.exporter.statsd "example" {
  listen_udp            = ""
  listen_tcp            = ":9125"
  listen_unixgram       = ""
  unix_socket_mode      = "755"
  mapping_config_path   = "mapTest.yaml"
  read_buffer           = 1
  cache_size            = 1000
  cache_type            = "lru"
  event_queue_size      = 10000
  event_flush_threshold = 1000
  event_flush_interval  = "200ms"
  parse_dogstatsd_tags  = true
  parse_influxdb_tags   = true
  parse_librato_tags    = true
  parse_signalfx_tags   = true
}

// Configure a prometheus.scrape component to collect statsd metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.statsd.example.targets
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
- `USERNAME`: The username to use for authentication to the `remote_write` API.
- `PASSWORD`: The password to use for authentication to the `remote_write` API.

[scrape]: ../prometheus.scrape/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.statsd` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.influxdb/
description: Learn about otelcol.receiver.influxdb
title: otelcol.receiver.influxdb
---

<span class="badge docs-labels__stage docs-labels__item">Experimental</span>

# otelcol.receiver.influxdb

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.influxdb` receives InfluxDB metrics, converts them into OpenTelemetry (OTEL) format, and forwards them to other otelcol.* components over the network.

You can specify multiple `otelcol.receiver.influxdb` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.influxdb "influxdb_metrics" {
  endpoint = "localhost:8086"  // InfluxDB metrics ingestion endpoint

  output {
    metrics = [otelcol.exporter.prometheus.influx_output.input]  // Forward metrics to Prometheus exporter
  }
}
```

## Arguments

`otelcol.receiver.influxdb` supports the following arguments:

| Name                 | Type           | Description                                                      | Default            | Required |
| -------------------- | -------------- | ---------------------------------------------------------------- | ------------------ | -------- |
| `endpoint`           | `string`       | `host:port` to listen for traffic on.                            | `"localhost:8086"` | no       |


By default, `otelcol.receiver.influxdb` listens for HTTP connections on `localhost`.
To expose the HTTP server to other machines on your network, configure `endpoint` with the IP address to listen on, or `0.0.0.0:8086` to listen on all network interfaces.

## Blocks

The following blocks are supported inside the definition of `otelcol.receiver.influxdb`:

| Hierarchy     | Block             | Description                                                                | Required |
| ------------- | ----------------- | -------------------------------------------------------------------------- | -------- |
| `output`      | `output`          | Configures where to send received metrics.                                  | yes      |

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.influxdb` doesn't export any fields.

## Component health

`otelcol.receiver.influxdb` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.influxdb` does not expose any component-specific debug information.

## Example

This example forwards received telemetry to Prometheus Remote Write (Mimir):

```alloy
otelcol.receiver.influxdb "influxdb_metrics" {
  endpoint = "localhost:8086"  // InfluxDB metrics ingestion endpoint

  output {
    metrics = [otelcol.exporter.prometheus.influx_output.input]  // Forward metrics to Prometheus exporter
  }
}

otelcol.exporter.prometheus "influx_output" {
  forward_to = [prometheus.remote_write.mimir.receiver]  // Forward metrics to Prometheus remote write (Mimir)
}

prometheus.remote_write "mimir" {
  endpoint {
    url = "https://prometheus-prod-13-prod-us-east-0.grafana.net/api/prom/push"

    basic_auth {
      username = "xxxxx"
      password = "xxxx=="
    }
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.influxdb` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

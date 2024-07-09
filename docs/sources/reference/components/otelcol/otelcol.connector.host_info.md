---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.connector.host_info/
aliases:
  - ../otelcol.connector.host_info/ # /docs/alloy/latest/reference/components/otelcol.connector.host_info/
description: Learn about otelcol.connector.host_info
title: otelcol.connector.host_info
---

# otelcol.connector.host_info

`otel.connector.host_info` accepts span data from other `otelcol` components and generates usage metrics.

## Usage

```alloy
otelcol.connector.host_info "LABEL" {
  output {
    metrics = [...]
  }
}
```

## Arguments

`otelcol.connector.host_info` supports the following arguments:

| Name                     | Type           | Description                                                        | Default       | Required |
| ------------------------ | -------------- | ------------------------------------------------------------------ | ------------- | -------- |
| `host_identifiers`       | `list(string)` | Ordered list of resource attributes used to identify unique hosts. | `["host.id"]` | no       |
| `metrics_flush_interval` | `duration`     | How often to flush generated metrics.                              | `"60s"`       | no       |

## Blocks

The following blocks are supported inside the definition of
`otelcol.connector.host_info`:

| Hierarchy | Block      | Description                                       | Required |
| --------- | ---------- | ------------------------------------------------- | -------- |
| output    | [output][] | Configures where to send received telemetry data. | yes      |
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

[output]: #output-block
[debug_metrics]: #debug_metrics-block

### output block

{{< docs/shared lookup="reference/components/output-block-metrics.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` traces telemetry data. It does not accept metrics and logs.

## Example

The example below accepts traces, adds the `host.id` resource attribute via the `otelcol.processor.resourcedetection` component,
creates usage metrics from these traces, and writes the metrics to Mimir.

```alloy
otelcol.receiver.otlp "otlp" {
  http {}
  grpc {}

  output {
    traces = [otelcol.processor.resourcedetection.otlp_resources.input]
  }
}

otelcol.processor.resourcedetection "otlp_resources" {
  detectors = ["system"]
  system {
    hostname_sources = [ "os" ]
    resource_attributes {
      host.id {
        enabled = true
      }
    }
  }
  output {
    traces = [otelcol.connector.host_info.default.input]
  }
}

otelcol.connector.host_info "default" {
  output {
    metrics = [otelcol.exporter.prometheus.otlp_metrics.input]
  }
}

otelcol.exporter.prometheus "otlp_metrics" {
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "https://prometheus-xxx.grafana.net/api/prom/push"
    basic_auth {
      username = env("PROMETHEUS_USERNAME")
      password = env("GRAFANA_CLOUD_API_KEY")
    }
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.connector.host_info` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.connector.host_info` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.prometheus/
aliases:
  - ../otelcol.receiver.prometheus/ # /docs/alloy/latest/reference/otelcol.receiver.prometheus/
description: Learn about otelcol.receiver.prometheus
title: otelcol.receiver.prometheus
---

# otelcol.receiver.prometheus

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.prometheus` receives Prometheus metrics, converts them to the
OpenTelemetry metrics format, and forwards them to other `otelcol.*`
components.

Multiple `otelcol.receiver.prometheus` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.receiver.prometheus "LABEL" {
  output {
    metrics = [...]
  }
}
```

## Arguments

`otelcol.receiver.prometheus` doesn't support any arguments and is configured fully
through inner blocks.

## Blocks

The following blocks are supported inside the definition of
`otelcol.receiver.prometheus`:

Hierarchy | Block | Description | Required
--------- | ----- | ----------- | --------
output | [output][] | Configures where to send received telemetry data. | yes

[output]: #output-block

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`receiver` | `MetricsReceiver` | A value that other components can use to send Prometheus metrics to.

## Component health

`otelcol.receiver.prometheus` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.receiver.prometheus` does not expose any component-specific debug
information.

## Example

This example uses the `otelcol.receiver.prometheus` component as a bridge
between the Prometheus and OpenTelemetry ecosystems. The component exposes a
receiver which the `prometheus.scrape` component uses to send Prometheus metric
data to. The metrics are converted to the OTLP format before they are forwarded
to the `otelcol.exporter.otlp` component to be sent to an OTLP-capable
endpoint:

```alloy
prometheus.scrape "default" {
    // Collect metrics from the default HTTP listen address.
    targets = [{"__address__"   = "127.0.0.1:12345"}]

    forward_to = [otelcol.receiver.prometheus.default.receiver]
}

otelcol.receiver.prometheus "default" {
  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = env("OTLP_ENDPOINT")
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.prometheus` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.receiver.prometheus` has exports that can be consumed by the following components:

- Components that consume [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

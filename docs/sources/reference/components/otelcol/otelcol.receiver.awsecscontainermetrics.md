---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.awsecscontainermetrics/
description: Learn about otelcol.receiver.awsecscontainermetrics
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.awsecscontainermetrics
---

# `otelcol.receiver.awsecscontainermetrics`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.awsecscontainermetrics` reads AWS ECS task- and container-level metadata and resource usage metrics (such as CPU, memory, network, and disk) and forwards them to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.awsecscontainermetrics` is a wrapper over the upstream OpenTelemetry Collector [`awsecscontainermetrics`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`awsecscontainermetrics`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/awsecscontainermetricsreceiver
{{< /admonition >}}

This receiver supports ECS Fargate and ECS on EC2 (it requires the [ECS Task Metadata Endpoint V4](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v4.html) which is automatically available within the task's containers). Therefore, you should run the Alloy collector using this receiver as a sidecar within the task you wish to monitor. Check the upstream receiver documentation for more details.

You can specify multiple `otelcol.receiver.awsecscontainermetrics` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.awsecscontainermetrics "<LABEL>" {
  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.awsecscontainermetrics`:

| Name            | Type                       | Description                                                              | Default | Required |
|-----------------|----------------------------|--------------------------------------------------------------------------|---------|----------|
| `collection_interval`        | `duration`                   | How frequently to collect and emit metrics.                                         |    "20s"     | no      |

## Blocks

You can use the following blocks with `otelcol.receiver.awsecscontainermetrics`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[debug_metrics]: #debug_metrics
[output]: #output

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.awsecscontainermetrics` doesn't export any fields.

## Component health

`otelcol.receiver.awsecscontainermetrics` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.awsecscontainermetrics` doesn't expose any component-specific debug information.

## Example

The following example collects 8 task-level metrics (out of 52 metrics) from an ECS task by forwarding the metrics to a filter processor.

```alloy
otelcol.receiver.awsecscontainermetrics "default" {
  collection_interval = "30s"

  output {
    metrics = [otelcol.processor.filter.default.input]
  }
}

otelcol.processor.filter "default" {
  error_mode = "ignore"

  metrics {
    metric = [
      `name == "ecs.task.memory.reserved"`,
      `name == "ecs.task.memory.utilized"`,
      `name == "ecs.task.cpu.reserved"`,
      `name == "ecs.task.cpu.utilized"`,
      `name == "ecs.task.network.rate.rx"`,
      `name == "ecs.task.network.rate.tx"`,
      `name == "ecs.task.storage.read_bytes"`,
      `name == "ecs.task.storage.write_bytes"`,
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = env("<OTLP_ENDPOINT>")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.awsecscontainermetrics` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

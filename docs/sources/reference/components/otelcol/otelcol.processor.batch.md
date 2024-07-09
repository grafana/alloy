---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.batch/
aliases:
  - ../otelcol.processor.batch/ # /docs/alloy/latest/reference/otelcol.processor.batch/
description: Learn about otelcol.processor.batch
title: otelcol.processor.batch
---

# otelcol.processor.batch

`otelcol.processor.batch` accepts telemetry data from other `otelcol`
components and places them into batches. Batching improves the compression of
data and reduces the number of outgoing network requests required to transmit
data. This processor supports both size and time based batching.

We strongly recommend that you configure the batch processor on every {{< param "PRODUCT_NAME" >}} that uses OpenTelemetry (otelcol) {{< param "PRODUCT_NAME" >}} components.
The batch processor should be defined in the pipeline after the `otelcol.processor.memory_limiter` as well as any sampling processors.
This is because batching should happen after any data drops such as sampling.

> **NOTE**: `otelcol.processor.batch` is a wrapper over the upstream
> OpenTelemetry Collector `batch` processor. Bug reports or feature requests
> will be redirected to the upstream repository, if necessary.

Multiple `otelcol.processor.batch` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.processor.batch "LABEL" {
  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

`otelcol.processor.batch` supports the following arguments:

Name                         | Type           | Description                                                             | Default   | Required
-----------------------------|----------------|-------------------------------------------------------------------------|-----------|---------
`timeout`                    | `duration`     | How long to wait before flushing the batch.                             | `"200ms"` | no
`send_batch_size`            | `number`       | Amount of data to buffer before flushing the batch.                     | `8192`    | no
`send_batch_max_size`        | `number`       | Upper limit of a batch size.                                            | `0`       | no
`metadata_keys`              | `list(string)` | Creates a different batcher for each key/value combination of metadata. | `[]`      | no
`metadata_cardinality_limit` | `number`       | Limit of the unique metadata key/value combinations.                    | `1000`    | no

`otelcol.processor.batch` accumulates data into a batch until one of the
following events happens:

* The duration specified by `timeout` elapses since the time the last batch was
  sent.

* The number of spans, log lines, or metric samples processed is greater than
  or equal to the number specified by `send_batch_size`.

Logs, traces, and metrics are processed independently.
For example, if `send_batch_size` is set to `1000`:
* The processor may, at the same time, buffer 1,000 spans,
  1,000 log lines, and 1,000 metric samples before flushing them.
* If there are enough spans for a batch of spans (1,000 or more), but not enough for a
  batch of metric samples (less than 1,000) then only the spans will be flushed.

Use `send_batch_max_size` to limit the amount of data contained in a single batch:
* When set to `0`, batches can be any size.
* When set to a non-zero value, `send_batch_max_size` must be greater than or equal to `send_batch_size`.
  Every batch will contain up to the `send_batch_max_size` number of spans, log lines, or metric samples.
  The excess spans, log lines, or metric samples will not be lost - instead, they will be added to
  the next batch.

For example, assume `send_batch_size` is set to the default `8192` and there
are currently 8,000 batched spans. If the batch processor receives 8,000 more
spans at once, its behavior depends on how `send_batch_max_size` is configured:
* If `send_batch_max_size` is set to `0`, the total batch size would be 16,000
  which would then be flushed as a single batch.
* If `send_batch_max_size` is set to `10000`, then the total batch size will be
  10,000 and the remaining 6,000 spans will be flushed in a subsequent batch.

`metadata_cardinality_limit` applies for the lifetime of the process.

Receivers should be configured with `include_metadata = true` so that metadata
keys are available to the processor.

Each distinct combination of metadata triggers the allocation of a new
background task in the {{< param "PRODUCT_NAME" >}} process that runs for the lifetime of the process, and each
background task holds one pending batch of up to `send_batch_size` records. Batching
by metadata can therefore substantially increase the amount of memory dedicated to batching.

The maximum number of distinct combinations is limited to the configured `metadata_cardinality_limit`,
which defaults to 1000 to limit memory impact.

## Blocks

The following blocks are supported inside the definition of
`otelcol.processor.batch`:

Hierarchy | Block      | Description                                       | Required
----------|------------|---------------------------------------------------|---------
output    | [output][] | Configures where to send received telemetry data. | yes
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

[output]: #output-block
[debug_metrics]: #debug_metrics-block

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics,
logs, or traces).

## Component health

`otelcol.processor.batch` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.processor.batch` does not expose any component-specific debug
information.

## Debug metrics

* `processor_batch_batch_send_size_bytes` (histogram): Number of bytes in batch that was sent.
* `processor_batch_batch_send_size_ratio` (histogram): Number of units in the batch.
* `processor_batch_metadata_cardinality_ratio` (gauge): Number of distinct metadata value combinations being processed.
* `processor_batch_timeout_trigger_send_ratio_total` (counter): Number of times the batch was sent due to a timeout trigger.
* `processor_batch_batch_size_trigger_send_ratio_total` (counter): Number of times the batch was sent due to a size trigger.

## Examples

### Basic usage

This example batches telemetry data before sending it to
[otelcol.exporter.otlp][] for further processing:

```alloy
otelcol.processor.batch "default" {
  output {
    metrics = [otelcol.exporter.otlp.production.input]
    logs    = [otelcol.exporter.otlp.production.input]
    traces  = [otelcol.exporter.otlp.production.input]
  }
}

otelcol.exporter.otlp "production" {
  client {
    endpoint = env("OTLP_SERVER_ENDPOINT")
  }
}
```

### Batching with a timeout

This example will buffer up to 10,000 spans, metric data points, or log records for up to 10 seconds.
Because `send_batch_max_size` is not set, the batch size may exceed 10,000.

```alloy
otelcol.processor.batch "default" {
  timeout = "10s"
  send_batch_size = 10000

  output {
    metrics = [otelcol.exporter.otlp.production.input]
    logs    = [otelcol.exporter.otlp.production.input]
    traces  = [otelcol.exporter.otlp.production.input]
  }
}

otelcol.exporter.otlp "production" {
  client {
    endpoint = env("OTLP_SERVER_ENDPOINT")
  }
}
```

### Batching based on metadata

Batching by metadata enables support for multi-tenant OpenTelemetry pipelines
with batching over groups of data having the same authorization metadata.

```alloy
otelcol.receiver.jaeger "default" {
  protocols {
    grpc {
      include_metadata = true
    }
    thrift_http {}
    thrift_binary {}
    thrift_compact {}
  }

  output {
    traces = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  // batch data by tenant id
  metadata_keys = ["tenant_id"]
  // limit to 10 batcher processes before raising errors
  metadata_cardinality_limit = 123

  output {
    traces  = [otelcol.exporter.otlp.production.input]
  }
}

otelcol.exporter.otlp "production" {
  client {
    endpoint = env("OTLP_SERVER_ENDPOINT")
  }
}
```

[otelcol.exporter.otlp]: ../otelcol.exporter.otlp/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.batch` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.batch` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

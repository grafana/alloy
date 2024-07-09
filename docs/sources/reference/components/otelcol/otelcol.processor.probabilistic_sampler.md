---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.probabilistic_sampler/
aliases:
  - ../otelcol.processor.probabilistic_sampler/ # /docs/alloy/latest/reference/otelcol.processor.probabilistic_sampler/
description: Learn about telcol.processor.probabilistic_sampler
title: otelcol.processor.probabilistic_sampler
---

# otelcol.processor.probabilistic_sampler

`otelcol.processor.probabilistic_sampler` accepts logs and traces data from other otelcol components and applies probabilistic sampling based on configuration options.

{{< admonition type="note" >}}
`otelcol.processor.probabilistic_sampler` is a wrapper over the upstream OpenTelemetry Collector Contrib `probabilistic_sampler` processor.
If necessary, bug reports or feature requests will be redirected to the upstream repository.
{{< /admonition >}}

You can specify multiple `otelcol.processor.probabilistic_sampler` components by giving them different labels.

## Usage

```alloy
otelcol.processor.probabilistic_sampler "LABEL" {
  output {
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

`otelcol.processor.probabilistic_sampler` supports the following arguments:

Name                  | Type      | Description                                                                                                          | Default     | Required
----------------------|-----------|----------------------------------------------------------------------------------------------------------------------|-------------|---------
`hash_seed`           | `uint32`  | An integer used to compute the hash algorithm.                                                                       | `0`         | no
`sampling_percentage` | `float32` | Percentage of traces or logs sampled.                                                                                | `0`         | no
`fail_closed`         | `bool`    | Whether to reject items with sampling-related errors.                                                                | `true`      | no
`attribute_source`    | `string`  | Defines where to look for the attribute in `from_attribute`.                                                         | `"traceID"` | no
`from_attribute`      | `string`  | The name of a log record attribute used for sampling purposes.                                                       | `""`        | no
`sampling_priority`   | `string`  | The name of a log record attribute used to set a different sampling priority from the `sampling_percentage` setting. | `""`        | no

`hash_seed` determines an integer to compute the hash algorithm. This argument could be used for both traces and logs.
When used for logs, it computes the hash of a log record.
For hashing to work, all collectors for a given tier, for example, behind the same load balancer, must have the same `hash_seed`.
It is also possible to leverage a different `hash_seed` at different collector tiers to support additional sampling requirements.

`sampling_percentage` determines the percentage at which traces or logs are sampled. All traces or logs are sampled if you set this argument to a value greater than or equal to 100.

`attribute_source` (logs only) determines where to look for the attribute in `from_attribute`. The allowed values are `traceID` or `record`.

`from_attribute` (logs only) determines the name of a log record attribute used for sampling purposes, such as a unique log record ID. The value of the attribute is only used if the trace ID is absent or if `attribute_source` is set to `record`.

`sampling_priority` (logs only) determines the name of a log record attribute used to set a different sampling priority from the `sampling_percentage` setting. 0 means to never sample the log record, and greater than or equal to 100 means to always sample the log record.

The `probabilistic_sampler` supports two types of sampling for traces:
1. `sampling.priority` [semantic convention](https://github.com/opentracing/specification/blob/master/semantic_conventions.md#span-tags-table) as defined by OpenTracing.
2. Trace ID hashing.

The `sampling.priority` semantic convention takes priority over trace ID hashing.
Trace ID hashing samples based on hash values determined by trace IDs.

The `probabilistic_sampler` supports sampling logs according to their trace ID, or by a specific log record attribute.

## Blocks

The following blocks are supported inside the definition of
`otelcol.processor.probabilistic_sampler`:

Hierarchy | Block      | Description                          | Required
----------|------------|--------------------------------------|---------
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

[debug_metrics]: #debug_metrics-block

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` OTLP-formatted data for any telemetry signal of these types:
* logs
* traces

## Component health

`otelcol.processor.probabilistic_sampler` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.processor.probabilistic_sampler` does not expose any component-specific debug
information.

## Examples

### Basic usage

```alloy
otelcol.processor.probabilistic_sampler "default" {
  hash_seed           = 123
  sampling_percentage = 15.3

  output {
    logs = [otelcol.exporter.otlp.default.input]
  }
}
```

### Sample 15% of the logs

```alloy
otelcol.processor.probabilistic_sampler "default" {
  sampling_percentage = 15

  output {
    logs = [otelcol.exporter.otlp.default.input]
  }
}
```

### Sample logs according to their "logID" attribute

```alloy
otelcol.processor.probabilistic_sampler "default" {
  sampling_percentage = 15
  attribute_source    = "record"
  from_attribute      = "logID"

  output {
    logs = [otelcol.exporter.otlp.default.input]
  }
}
```

### Sample logs according to a "priority" attribute

```alloy
otelcol.processor.probabilistic_sampler "default" {
  sampling_percentage = 15
  sampling_priority   = "priority"

  output {
    logs = [otelcol.exporter.otlp.default.input]
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.probabilistic_sampler` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.probabilistic_sampler` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

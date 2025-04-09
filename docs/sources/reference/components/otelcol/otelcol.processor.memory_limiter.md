---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.memory_limiter/
aliases:
  - ../otelcol.processor.memory_limiter/ # /docs/alloy/latest/reference/otelcol.processor.memory_limiter/
description: Learn about otelcol.processor.memory_limiter
title: otelcol.processor.memory_limiter
---

# otelcol.processor.memory_limiter

`otelcol.processor.memory_limiter` is used to prevent out of memory situations
on a telemetry pipeline by performing periodic checks of memory usage. If
usage exceeds the defined limits, data is dropped and garbage collections
are triggered to reduce it.

The `memory_limiter` component uses both soft and hard limits, where the hard limit
is always equal or larger than the soft limit. When memory usage goes above the
soft limit, the processor component drops data and returns errors to the
preceding components in the pipeline. When usage exceeds the hard
limit, the processor forces a garbage collection in order to try and free
memory. When usage is below the soft limit, no data is dropped and no forced
garbage collection is performed.

> **NOTE**: `otelcol.processor.memory_limiter` is a wrapper over the upstream
> OpenTelemetry Collector `memorylimiter` processor. Bug reports or feature
> requests will be redirected to the upstream repository, if necessary.

Multiple `otelcol.processor.memory_limiter` components can be specified by
giving them different labels.

## Usage

```alloy
otelcol.processor.memory_limiter "LABEL" {
  check_interval = "1s"

  limit = "50MiB" // alternatively, set `limit_percentage` and `spike_limit_percentage`

  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

`otelcol.processor.memory_limiter` supports the following arguments:

Name                     | Type       | Description                                                                            | Default        | Required
-------------------------|------------|----------------------------------------------------------------------------------------|----------------|---------
`check_interval`         | `duration` | How often to check memory usage.                                                       |                | yes
`limit`                  | `string`   | Maximum amount of memory targeted to be allocated by the process heap.                 | `"0MiB"`       | no
`spike_limit`            | `string`   | Maximum spike expected between the measurements of memory usage.                       | 20% of `limit` | no
`limit_percentage`       | `int`      | Maximum amount of total available memory targeted to be allocated by the process heap. | `0`            | no
`spike_limit_percentage` | ` int`     | Maximum spike expected between the measurements of memory usage.                       | `0`            | no

The arguments must define either `limit` or the `limit_percentage,
spike_limit_percentage` pair, but not both.

The configuration options `limit` and `limit_percentage` define the hard
limits. The soft limits are then calculated as the hard limit minus the
`spike_limit` or `spike_limit_percentage` values respectively. The recommended
value for spike limits is about 20% of the corresponding hard limit.

The recommended `check_interval` value is 1 second. If the traffic through the
component is spiky in nature, it is recommended to either decrease the interval
or increase the spike limit to avoid going over the hard limit.

The `limit` and `spike_limit` values must be larger than 1 MiB.

## Blocks

The following blocks are supported inside the definition of
`otelcol.processor.memory_limiter`:

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

`otelcol.processor.memory_limiter` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.processor.memory_limiter` does not expose any component-specific debug
information.
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.memory_limiter` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.memory_limiter` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
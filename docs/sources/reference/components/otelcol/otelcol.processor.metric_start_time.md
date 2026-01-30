---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.metric_start_time/
description: Learn about otelcol.processor.metric_start_time
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.processor.metric_start_time
---

# `otelcol.processor.metric_start_time`

`otelcol.processor.metric_start_time` accepts metrics from other `otelcol` components and sets the start time for cumulative metric datapoints which do not already have a start time.
This processor is commonly used with `otelcol.receiver.prometheus`, which produces metric points without a [start time][otlp-start-time].

Grafana Mimir ingests OTLP metric start times only when it is configured with the -distributor.otel-created-timestamp-zero-ingestion-enabled flag.
Without this configuration, setting start times in Alloy has no effect on ingestion behavior.

{{< admonition type="note" >}}
`otelcol.processor.metric_start_time` is a wrapper over the upstream OpenTelemetry Collector [`metricstarttime`][] processor.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`metricstarttime`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/metricstarttimeprocessor
{{< /admonition >}}

You can specify multiple `otelcol.processor.metric_start_time` components by giving them different labels.

[otlp-start-time]: https://github.com/open-telemetry/opentelemetry-proto/blob/v1.9.0/opentelemetry/proto/metrics/v1/metrics.proto#L181-L187

## Usage

```alloy
otelcol.processor.metric_start_time "<LABEL>" {
  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.processor.metric_start_time`:

| Name                      | Type       | Description                                                                                           | Default              | Required |
|---------------------------|------------|-------------------------------------------------------------------------------------------------------|----------------------|----------|
| `strategy`                | `string`   | Strategy to use for setting start time.                                                               | `"true_reset_point"` | no       |
| `gc_interval`             | `duration` | How long to wait before removing a metric from the cache.                                             | `"10m"`              | no       |
| `start_time_metric_regex` | `string`   | Regex for a metric name containing the start time. Only applies when strategy is `start_time_metric`. | `""`                 | no       |

### Strategies

The `strategy` argument determines how the processor handles missing start times for cumulative metrics. Valid values are:

#### `true_reset_point` (default)

Produces a stream of points that starts with a [True Reset Point][true-reset-point].
The true reset point has its start time set to its end timestamp, indicating the absolute value of the cumulative point when the collector first observed it.
Subsequent points reuse the start timestamp of the initial true reset point.

[true-reset-point]: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md#cumulative-streams-inserting-true-reset-points

**Pros:**
* The absolute value of the cumulative metric is preserved.
* It is possible to calculate the correct rate between any two points since the timestamps and values are not modified.

**Cons:**
* This strategy is **stateful** because the initial True Reset point is necessary to properly calculate rates on subsequent points.
* The True Reset point doesn't make sense semantically. It has a zero duration, but non-zero values.
* Many backends reject points with equal start and end timestamps.
  * If the True Reset point is rejected, the next point will appear to have a very large rate.

**Example transformation:**

| Input Timestamp | Input Value | Output Start Time | Output End Time | Output Value |
|-----------------|-------------|-------------------|-----------------|--------------|
| T1              | 10          | T1                | T1              | 10           |
| T2              | 15          | T1                | T2              | 15           |
| T3              | 25          | T1                | T3              | 25           |

#### `subtract_initial_point`

Drops the first point in a cumulative series, subtracting that point's value from subsequent points and using the initial point's timestamp as the start timestamp for subsequent points.

**Pros:**
* Cumulative semantics are preserved. This means that for a point with a given `[start, end]` interval, the cumulative value occurred in that interval.
* Rates over resulting timeseries are correct, even if points are lost. This strategy is not stateful.

**Cons:**
* The absolute value of counters is modified. This is generally not an issue, since counters are usually used to compute rates.
* The initial point is dropped, which loses information.

**Example transformation:**

| Input Timestamp | Input Value | Output Start Time | Output End Time | Output Value |
|-----------------|-------------|-------------------|-----------------|--------------|
| T1              | 10          | _(dropped)_       | _(dropped)_     | _(dropped)_  |
| T2              | 15          | T1                | T2              | 5            |
| T3              | 25          | T1                | T3              | 15           |

#### `start_time_metric`

Looks for the `process_start_time` metric (or a metric matching `start_time_metric_regex`) and uses its value as the start time for all other cumulative points in the batch of metrics.
If the start time metric is not found, it falls back to the time at which the collector started.

This strategy should only be used in limited circumstances:
* When your application has a metric with the start time in Unix seconds, such as `process_start_time_seconds`.
* The processor is used _before_ any batching, so that the batch of metrics all originate from a single application.
* This strategy can be used when the collector is run as a sidecar to the application, where the collector's start time is a good approximation of the application's start time.

**Cons:**
* If the collector's start time is used as a fallback and the collector restarts, it can produce rates that are incorrect and higher than expected.
* The process' start time isn't the time at which individual instruments or timeseries are initialized. It may result in lower rates if the first observation is significantly later than the process' start time.

**Example transformation:**

Given a `process_start_time_seconds` metric with value `T0`:

| Input Timestamp | Input Value | Output Start Time | Output End Time | Output Value |
|-----------------|-------------|-------------------|-----------------|--------------|
| T1              | 10          | T0                | T1              | 10           |
| T2              | 15          | T0                | T2              | 15           |
| T3              | 25          | T0                | T3              | 25           |

### Garbage collection

The `gc_interval` argument defines how often to check if any resources have not emitted data since the last check.
If a resource hasn't emitted any data, it's removed from the cache to free up memory. 
Any additional data from resources removed from the cache will be given a new start time.

## Blocks

You can use the following blocks with `otelcol.processor.metric_start_time`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[output]: #output
[debug_metrics]: #debug_metrics

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for metrics.

## Component health

`otelcol.processor.metric_start_time` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.processor.metric_start_time` doesn't expose any component-specific debug information.

## Examples

### Basic usage with default strategy

This example uses the default `true_reset_point` strategy to set start times for Prometheus metrics:

```alloy
otelcol.receiver.prometheus "default" {
  output {
    metrics = [otelcol.processor.metric_start_time.default.input]
  }
}

otelcol.processor.metric_start_time "default" {
  output {
    metrics = [otelcol.exporter.otlp.production.input]
  }
}

otelcol.exporter.otlp "production" {
  client {
    endpoint = sys.env("OTLP_SERVER_ENDPOINT")
  }
}
```

### Using subtract_initial_point strategy

This example uses the `subtract_initial_point` strategy, which preserves cumulative semantics and produces correct rates:

```alloy
otelcol.receiver.prometheus "default" {
  output {
    metrics = [otelcol.processor.metric_start_time.default.input]
  }
}

otelcol.processor.metric_start_time "default" {
  strategy = "subtract_initial_point"

  output {
    metrics = [otelcol.exporter.otlp.production.input]
  }
}

otelcol.exporter.otlp "production" {
  client {
    endpoint = sys.env("OTLP_SERVER_ENDPOINT")
  }
}
```

### Using start_time_metric strategy with custom regex

This example uses the `start_time_metric` strategy with a custom regex to find the start time metric:

```alloy
otelcol.receiver.prometheus "default" {
  output {
    metrics = [otelcol.processor.metric_start_time.default.input]
  }
}

otelcol.processor.metric_start_time "default" {
  strategy                 = "start_time_metric"
  gc_interval              = "1h"
  start_time_metric_regex  = "^.+_start_time$"

  output {
    metrics = [otelcol.exporter.otlp.production.input]
  }
}

otelcol.exporter.otlp "production" {
  client {
    endpoint = sys.env("OTLP_SERVER_ENDPOINT")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.metric_start_time` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.metric_start_time` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

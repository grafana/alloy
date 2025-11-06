---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.connector.spanmetrics/
aliases:
  - ../otelcol.connector.spanmetrics/ # /docs/alloy/latest/reference/components/otelcol.connector.spanmetrics/
description: Learn about otelcol.connector.spanmetrics
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.connector.spanmetrics
---

# `otelcol.connector.spanmetrics`

`otelcol.connector.spanmetrics` accepts span data from other `otelcol` components and aggregates Request, Error, and Duration (R.E.D) OpenTelemetry metrics from the spans:

* **Request** counts are computed as the number of spans seen per unique set of dimensions, including Errors. Multiple metrics can be aggregated if, for instance, a user wishes to view call counts just on `service.name` and `span.name`.

  Requests are tracked using a `calls` metric with a `status.code` datapoint attribute set to `Ok`:

  ```text
  calls { service.name="shipping", span.name="get_shipping/{shippingId}", span.kind="SERVER", status.code="Ok" }
  ```

* **Error** counts are computed from the number of spans with an `Error` status code.

  Errors are tracked using a `calls` metric with a `status.code` datapoint attribute set to `Error`:

  ```text
  calls { service.name="shipping", span.name="get_shipping/{shippingId}, span.kind="SERVER", status.code="Error" }
  ```

* **Duration** is computed from the difference between the span start and end times and inserted into the relevant duration histogram time bucket for each unique set dimensions.

  Span durations are tracked using a `duration` histogram metric:

  ```text
  duration { service.name="shipping", span.name="get_shipping/{shippingId}", span.kind="SERVER", status.code="Ok" }
  ```

{{< admonition type="note" >}}
`otelcol.connector.spanmetrics` is a wrapper over the upstream OpenTelemetry Collector [`spanmetrics`][] connector.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`spanmetrics`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/spanmetricsconnector
{{< /admonition >}}

You can specify multiple `otelcol.connector.spanmetrics` components by giving them different labels.

## Usage

```alloy
otelcol.connector.spanmetrics "<LABEL>" {
  histogram {
    ...
  }

  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.connector.spanmetrics`:

| Name                              | Type           | Description                                                                                           | Default                 | Required |
|-----------------------------------|----------------|-------------------------------------------------------------------------------------------------------|-------------------------|----------|
| `aggregation_temporality`         | `string`       | Configures whether to reset the metrics after flushing.                                               | `"CUMULATIVE"`          | no       |
| `dimensions_cache_size`           | `number`       | (Deprecated: use `aggregation_cardinality_limit` instead) How many dimensions to cache.               | `0`                     | no       |
| `exclude_dimensions`              | `list(string)` | List of dimensions to be excluded from the default set of dimensions.                                 | `[]`                    | no       |
| `metric_timestamp_cache_size`     | `number`       | Controls the size of a cache used to keep track of the last time a metric was flushed.                | `1000`                  | no       |
| `metrics_expiration`              | `duration`     | Time period after which metrics are considered stale and are removed from the cache.                  | `"0s"`                  | no       |
| `metrics_flush_interval`          | `duration`     | How often to flush generated metrics.                                                                 | `"60s"`                 | no       |
| `namespace`                       | `string`       | Metric namespace.                                                                                     | `"traces.span.metrics"` | no       |
| `resource_metrics_cache_size`     | `number`       | The size of the cache holding metrics for a service.                                                  | `1000`                  | no       |
| `resource_metrics_key_attributes` | `list(string)` | Limits the resource attributes used to create the metrics.                                            | `[]`                    | no       |
| `aggregation_cardinality_limit`   | `number`       | The maximum number of unique combinations of dimensions that will be tracked for metrics aggregation. | `0`                     | no       |
| `include_instrumentation_scope`   | `list(string)` | A list of instrumentation scope names to include from the traces.                                     | `[]`                    | no       |

The supported values for `aggregation_temporality` are:

* `"CUMULATIVE"`: The metrics won't be reset after they're flushed.
* `"DELTA"`: The metrics will be reset after they're flushed.

If `namespace` is set, the generated metric name will be added a `namespace.` prefix.

Setting `metrics_expiration` to `"0s"` means that the metrics will never expire.

`resource_metrics_cache_size` is mostly relevant for cumulative temporality. It helps avoid issues with increasing memory and with incorrect metric timestamp resets.

`metric_timestamp_cache_size` is only relevant for delta temporality span metrics.
It controls the size of a cache used to keep track of the last time a metric was flushed.
When a metric is evicted from the cache, its next data point will indicate a "reset" in the series.
Downstream components converting from delta to cumulative may handle these resets by setting cumulative counters back to 0.

`resource_metrics_key_attributes` can be used to avoid situations where resource attributes may change across service restarts, causing metric counters to break (and duplicate).
A resource doesn't need to have all of the attributes.
The list must include enough attributes to properly identify unique resources or risk aggregating data from more than one service and span.
For example, `["service.name", "telemetry.sdk.language", "telemetry.sdk.name"]`.

When the `aggregation_cardinality_limit` limit is reached, additional unique combinations will be dropped but registered under a new entry with `otel.metric.overflow="true"`. 
A value of `0` means no limit is applied.

## Blocks

You can use the following blocks with `otelcol.connector.spanmetrics`:

| Block                                      | Description                                                                                                                                                | Required |
|--------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|
| [`histogram`][histogram]                   | Configures the histogram derived from spans durations.                                                                                                     | yes      |
| `histogram` > [`dimension`][dimension]     | Span event attributes to add as dimensions to the duration metric, _on top of_ the default ones and the ones configured in the top-level `dimension` block | no       |
| `histogram` > [`explicit`][explicit]       | Configuration for a histogram with explicit buckets.                                                                                                       | no       |
| `histogram` > [`exponential`][exponential] | Configuration for a histogram with exponential buckets.                                                                                                    | no       |
| [`output`][output]                         | Configures where to send telemetry data.                                                                                                                   | yes      |
| [`calls_dimension`][calls_dimension]       | Span event attributes to add as dimensions to the calls metric, _on top of_ the default ones and the ones configured in the top-level `dimension` block    | no       |
| [`debug_metrics`][debug_metrics]           | Configures the metrics that this component generates to monitor its state.                                                                                 | no       |
| [`dimension`][dimension]                   | Dimensions to be added in addition to the default ones.                                                                                                    | no       |
| [`events`][events]                         | Configures the events metric.                                                                                                                              | no       |
| `events` > [`dimension`][dimension]        | Span event attributes to add as dimensions to the events metric, _on top of_ the default ones and the ones configured in the top-level `dimension` block.  | no       |
| [`exemplars`][exemplars]                   | Configures how to attach exemplars to histograms.                                                                                                          | no       |

You must specify either an [`exponential`][exponential] or an [`explicit`][explicit] block.
You can't specify both blocks in the same configuration.

[calls_dimension]: #calls_dimension
[dimension]: #dimension
[histogram]: #histogram
[exponential]: #exponential
[explicit]: #explicit
[exemplars]: #exemplars
[events]: #events
[output]: #output
[debug_metrics]: #debug_metrics

### `histogram`

{{< badge text="Required" >}}

The `histogram` block configures the histogram derived from spans' durations.

The following attributes are supported:

| Name      | Type     | Description                     | Default | Required |
|-----------|----------|---------------------------------|---------|----------|
| `disable` | `bool`   | Disable all histogram metrics.  | `false` | no       |
| `unit`    | `string` | Configures the histogram units. | `"ms"`  | no       |

The supported values for `unit` are:

* `"ms"`: milliseconds
* `"s"`: seconds

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block-metrics.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `dimension`

The `dimension` block configures dimensions to be added in addition to the default ones.

The default dimensions are:

* `service.name`
* `span.name`
* `span.kind`
* `status.code`

The default dimensions are always added if not listed in `exclude_dimensions`. If no additional dimensions are specified, only the default ones will be added.

The following attributes are supported:

| Name      | Type     | Description                                      | Default | Required |
|-----------|----------|--------------------------------------------------|---------|----------|
| `name`    | `string` | Span attribute or resource attribute to look up. |         | yes      |
| `default` | `string` | Value to use if the attribute is missing.        |         | no       |

`otelcol.connector.spanmetrics` looks for the `name` attribute in the span's collection of attributes.
If it's not found, the resource attributes will be checked.

If the attribute is missing in both the span and resource attributes:

* If `default` isn't set, the dimension will be omitted.
* If `default` is set, the dimension will be added and its value will be set to the value of `default`.

### `calls_dimension`

The attributes and behavior of the `calls_dimension` block match the [`dimension`][dimension] block.

### `events`

The `events` block configures the `events` metric, which tracks [span events][span-events].

The following attributes are supported:

| Name      | Type   | Description                | Default | Required |
|-----------|--------|----------------------------|---------|----------|
| `enabled` | `bool` | Enables all events metric. | `false` | no       |

At least one nested `dimension` block is required if `enabled` is set to `true`.

[span-events]: https://opentelemetry.io/docs/concepts/signals/traces/#span-events

### `exemplars`

The `exemplars` block configures how to attach exemplars to histograms.

The following attributes are supported:

| Name                 | Type     | Description                                                                 | Default | Required |
|----------------------|----------|-----------------------------------------------------------------------------|---------|----------|
| `enabled`            | `bool`   | Configures whether to add exemplars to histograms.                          | `false` | no       |
| `max_per_data_point` | `number` | The maximum number of exemplars to attach to a single metric data point.    | `5`     | no       |

`max_per_data_point` can help with reducing memory consumption.

### `exponential`

The `exponential` block configures a histogram with exponential buckets.

The following attributes are supported:

| Name       | Type     | Description                                                      | Default | Required |
|------------|----------|------------------------------------------------------------------|---------|----------|
| `max_size` | `number` | Maximum number of buckets per positive or negative number range. | `160`   | no       |

### `explicit`

The `explicit` block configures a histogram with explicit buckets.

The following attributes are supported:

| Name      | Type             | Description                | Default                                                                                                                      | Required |
|-----------|------------------|----------------------------|------------------------------------------------------------------------------------------------------------------------------|----------|
| `buckets` | `list(duration)` | List of histogram buckets. | `["2ms", "4ms", "6ms", "8ms", "10ms", "50ms", "100ms", "200ms", "400ms", "800ms", "1s", "1400ms", "2s", "5s", "10s", "15s"]` | no       |

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` traces telemetry data.
It doesn't accept metrics and logs.

## Handle resource attributes

`otelcol.connector.spanmetrics` is an OTLP-native component.
As such, it aims to preserve the resource attributes of spans.

1. For example, assume that there are two incoming resources spans with the same `service.name` and `k8s.pod.name` resource attributes.
   {{< collapse title="Example JSON of two incoming spans." >}}

   ```json
   {
     "resourceSpans": [
       {
         "resource": {
           "attributes": [
             {
               "key": "service.name",
               "value": { "stringValue": "TestSvcName" }
             },
             {
               "key": "k8s.pod.name",
               "value": { "stringValue": "first" }
             }
           ]
         },
         "scopeSpans": [
           {
             "spans": [
               {
                 "trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
                 "span_id": "086e83747d0e381e",
                 "name": "TestSpan",
                 "attributes": [
                   {
                     "key": "attribute1",
                     "value": { "intValue": "78" }
                   }
                 ]
               }
             ]
           }
         ]
       },
       {
         "resource": {
           "attributes": [
             {
               "key": "service.name",
               "value": { "stringValue": "TestSvcName" }
             },
             {
               "key": "k8s.pod.name",
               "value": { "stringValue": "first" }
             }
           ]
         },
         "scopeSpans": [
           {
             "spans": [
               {
                 "trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
                 "span_id": "086e83747d0e381b",
                 "name": "TestSpan",
                 "attributes": [
                   {
                     "key": "attribute1",
                     "value": { "intValue": "78" }
                   }
                 ]
               }
             ]
           }
         ]
       }
     ]
   }
   ```

   {{< /collapse >}}

1. `otelcol.connector.spanmetrics` will preserve the incoming `service.name` and `k8s.pod.name` resource attributes by attaching them to the output metrics resource.
   Only one metric resource will be created, because both span resources have identical resource attributes.
   {{< collapse title="Example JSON of one outgoing metric resource." >}}

   ```json
   {
     "resourceMetrics": [
       {
         "resource": {
           "attributes": [
             {
               "key": "service.name",
               "value": { "stringValue": "TestSvcName" }
             },
             {
               "key": "k8s.pod.name",
               "value": { "stringValue": "first" }
             }
           ]
         },
         "scopeMetrics": [
           {
             "scope": { "name": "spanmetricsconnector" },
             "metrics": [
               {
                 "name": "calls",
                 "sum": {
                   "dataPoints": [
                     {
                       "attributes": [
                         {
                           "key": "service.name",
                           "value": { "stringValue": "TestSvcName" }
                         },
                         {
                           "key": "span.name",
                           "value": { "stringValue": "TestSpan" }
                         },
                         {
                           "key": "span.kind",
                           "value": { "stringValue": "SPAN_KIND_UNSPECIFIED" }
                         },
                         {
                           "key": "status.code",
                           "value": { "stringValue": "STATUS_CODE_UNSET" }
                         }
                       ],
                       "startTimeUnixNano": "1702582936761872000",
                       "timeUnixNano": "1702582936761872012",
                       "asInt": "2"
                     }
                   ],
                   "aggregationTemporality": 2,
                   "isMonotonic": true
                 }
               }
             ]
           }
         ]
       }
     ]
   }
   ```

   {{< /collapse >}}

1. Now, assume that `otelcol.connector.spanmetrics` receives two incoming resource spans, each with a different value for the `k8s.pod.name` recourse attribute.
   {{< collapse title="Example JSON of two incoming spans." >}}

   ```json
   {
     "resourceSpans": [
       {
         "resource": {
           "attributes": [
             {
               "key": "service.name",
               "value": { "stringValue": "TestSvcName" }
             },
             {
               "key": "k8s.pod.name",
               "value": { "stringValue": "first" }
             }
           ]
         },
         "scopeSpans": [
           {
             "spans": [
               {
                 "trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
                 "span_id": "086e83747d0e381e",
                 "name": "TestSpan",
                 "attributes": [
                   {
                     "key": "attribute1",
                     "value": { "intValue": "78" }
                   }
                 ]
               }
             ]
           }
         ]
       },
       {
         "resource": {
           "attributes": [
             {
               "key": "service.name",
               "value": { "stringValue": "TestSvcName" }
             },
             {
               "key": "k8s.pod.name",
               "value": { "stringValue": "second" }
             }
           ]
         },
         "scopeSpans": [
           {
             "spans": [
               {
                 "trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
                 "span_id": "086e83747d0e381b",
                 "name": "TestSpan",
                 "attributes": [
                   {
                     "key": "attribute1",
                     "value": { "intValue": "78" }
                   }
                 ]
               }
             ]
           }
         ]
       }
     ]
   }
   ```

   {{< /collapse >}}

1. To preserve the values of all resource attributes, `otelcol.connector.spanmetrics` will produce two resource metrics.
   Each resource metric will have a different value for the `k8s.pod.name` recourse attribute.
   This way none of the resource attributes will be lost during the generation of metrics.
   {{< collapse title="Example JSON of two outgoing metric resources." >}}

   ```json
   {
     "resourceMetrics": [
       {
         "resource": {
           "attributes": [
             {
               "key": "service.name",
               "value": { "stringValue": "TestSvcName" }
             },
             {
               "key": "k8s.pod.name",
               "value": { "stringValue": "first" }
             }
           ]
         },
         "scopeMetrics": [
           {
             "scope": {
               "name": "spanmetricsconnector"
             },
             "metrics": [
               {
                 "name": "calls",
                 "sum": {
                   "dataPoints": [
                     {
                       "attributes": [
                         {
                           "key": "service.name",
                           "value": { "stringValue": "TestSvcName" }
                         },
                         {
                           "key": "span.name",
                           "value": { "stringValue": "TestSpan" }
                         },
                         {
                           "key": "span.kind",
                           "value": { "stringValue": "SPAN_KIND_UNSPECIFIED" }
                         },
                         {
                           "key": "status.code",
                           "value": { "stringValue": "STATUS_CODE_UNSET" }
                         }
                       ],
                       "startTimeUnixNano": "1702582936761872000",
                       "timeUnixNano": "1702582936761872012",
                       "asInt": "1"
                     }
                   ],
                   "aggregationTemporality": 2,
                   "isMonotonic": true
                 }
               }
             ]
           }
         ]
       },
       {
         "resource": {
           "attributes": [
             {
               "key": "service.name",
               "value": { "stringValue": "TestSvcName" }
             },
             {
               "key": "k8s.pod.name",
               "value": { "stringValue": "second" }
             }
           ]
         },
         "scopeMetrics": [
           {
             "scope": {
               "name": "spanmetricsconnector"
             },
             "metrics": [
               {
                 "name": "calls",
                 "sum": {
                   "dataPoints": [
                     {
                       "attributes": [
                         {
                           "key": "service.name",
                           "value": { "stringValue": "TestSvcName" }
                         },
                         {
                           "key": "span.name",
                           "value": { "stringValue": "TestSpan" }
                         },
                         {
                           "key": "span.kind",
                           "value": { "stringValue": "SPAN_KIND_UNSPECIFIED" }
                         },
                         {
                           "key": "status.code",
                           "value": { "stringValue": "STATUS_CODE_UNSET" }
                         }
                       ],
                       "startTimeUnixNano": "1702582936761872000",
                       "timeUnixNano": "1702582936761872012",
                       "asInt": "1"
                     }
                   ],
                   "aggregationTemporality": 2,
                   "isMonotonic": true
                 }
               }
             ]
           }
         ]
       }
     ]
   }
   ```

   {{< /collapse >}}

## Component health

`otelcol.connector.spanmetrics` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.connector.spanmetrics` doesn't expose any component-specific debug information.

## Examples

### Explicit histogram and extra dimensions

In the example below, `http.status_code` and `http.method` are additional dimensions on top of:

* `service.name`
* `span.name`
* `span.kind`
* `status.code`

```alloy
otelcol.receiver.otlp "default" {
  http {}
  grpc {}

  output {
    traces  = [otelcol.connector.spanmetrics.default.input]
  }
}

otelcol.connector.spanmetrics "default" {
  // Since a default is not provided, the http.status_code dimension will be omitted
  // if the span does not contain http.status_code.
  dimension {
    name = "http.status_code"
  }

  // If the span is missing http.method, the connector will insert
  // the http.method dimension with value 'GET'.
  dimension {
    name = "http.method"
    default = "GET"
  }

  dimensions_cache_size = 333

  aggregation_temporality = "DELTA"

  histogram {
    unit = "s"
    explicit {
      buckets = ["333ms", "777s", "999h"]
    }
  }

  // The period on which all metrics (whose dimension keys remain in cache) will be emitted.
  metrics_flush_interval = "33s"

  namespace = "test.namespace"

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

### Send metrics via a Prometheus remote write

The generated metrics can be sent to a Prometheus-compatible database such as Grafana Mimir.
However, extra steps are required to make sure all metric samples are received.
This is because `otelcol.connector.spanmetrics` aims to [preserve resource attributes](#handle-resource-attributes) in the metrics which it outputs.

Unfortunately, the [Prometheus data model](https://prometheus.io/docs/concepts/data_model/) has no notion of resource attributes.
This means that if `otelcol.connector.spanmetrics` outputs metrics with identical metric attributes, but different resource attributes, `otelcol.exporter.prometheus` converts the metrics into the same metric series.
This problem can be solved by doing **either** of the following:

* **Recommended approach:** Prior to `otelcol.connector.spanmetrics`, remove all resource attributes from the incoming spans which aren't needed by `otelcol.connector.spanmetrics`.

  {{< collapse title="Example configuration to remove unnecessary resource attributes." >}}

  ```alloy
  otelcol.receiver.otlp "default" {
    http {}
    grpc {}

    output {
      traces  = [otelcol.processor.transform.default.input]
    }
  }

  // Remove all resource attributes except the ones which
  // the otelcol.connector.spanmetrics needs.
  // If this is not done, otelcol.exporter.prometheus may fail to
  // write some samples due to an "err-mimir-sample-duplicate-timestamp" error.
  // This is because the spanmetricsconnector will create a new
  // metrics resource scope for each traces resource scope.
  otelcol.processor.transform "default" {
    error_mode = "ignore"

    trace_statements {
      context = "resource"
      statements = [
        // We keep only the "service.name" and "special.attr" resource attributes,
        // because they are the only ones which otelcol.connector.spanmetrics needs.
        //
        // There is no need to list "span.name", "span.kind", and "status.code"
        // here because they are properties of the span (and not resource attributes):
        // https://github.com/open-telemetry/opentelemetry-proto/blob/v1.0.0/opentelemetry/proto/trace/v1/trace.proto
        `keep_keys(attributes, ["service.name", "special.attr"])`,
      ]
    }

    output {
      traces  = [otelcol.connector.spanmetrics.default.input]
    }
  }

  otelcol.connector.spanmetrics "default" {
    histogram {
      explicit {}
    }

    dimension {
      name = "special.attr"
    }
    output {
      metrics = [otelcol.exporter.prometheus.default.input]
    }
  }

  otelcol.exporter.prometheus "default" {
    forward_to = [prometheus.remote_write.mimir.receiver]
  }

  prometheus.remote_write "mimir" {
    endpoint {
      url = "http://mimir:9009/api/v1/push"
    }
  }
  ```

  {{< /collapse >}}

* Or, after `otelcol.connector.spanmetrics`, copy each of the resource attributes as a metric datapoint attribute.
  This has the advantage that the resource attributes will be visible as metric labels.
  However, the {{< term "cardinality" >}}cardinality{{< /term >}} of the metrics may be much higher, which could increase the cost of storing and querying them.
  The example below uses the [`merge_maps`][merge_maps] OTTL function.

  {{< collapse title="Example configuration to add all resource attributes as metric datapoint attributes." >}}

  ```alloy
  otelcol.receiver.otlp "default" {
    http {}
    grpc {}

    output {
      traces  = [otelcol.connector.spanmetrics.default.input]
    }
  }

  otelcol.connector.spanmetrics "default" {
    histogram {
      explicit {}
    }

    dimension {
      name = "special.attr"
    }
    output {
      metrics = [otelcol.processor.transform.default.input]
    }
  }

  // Insert resource attributes as metric data point attributes.
  otelcol.processor.transform "default" {
    error_mode = "ignore"

    metric_statements {
      context = "datapoint"
      statements = [
        // "insert" means that a metric datapoint attribute will be inserted
        // only if an attribute with the same key does not already exist.
        `merge_maps(attributes, resource.attributes, "insert")`,
      ]
    }

    output {
      metrics = [otelcol.exporter.prometheus.default.input]
    }
  }

  otelcol.exporter.prometheus "default" {
    forward_to = [prometheus.remote_write.mimir.receiver]
  }

  prometheus.remote_write "mimir" {
    endpoint {
      url = "http://mimir:9009/api/v1/push"
    }
  }
  ```

  {{< /collapse >}}

If the resource attributes aren't treated in either of the ways described above, an error such as this one could be logged by `prometheus.remote_write`:
`the sample has been rejected because another sample with the same timestamp, but a different value, has already been ingested (err-mimir-sample-duplicate-timestamp)`.

{{< admonition type="note" >}}
In order for a Prometheus `target_info` metric to be generated, the incoming spans resource scope
attributes must contain `service.name` and `service.instance.id` attributes.

The `target_info` metric will be generated for each resource scope, while OpenTelemetry
metric names and attributes will be normalized to be compliant with Prometheus naming rules.
{{< /admonition >}}

[merge_maps]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/ottlfuncs/README.md#merge_maps
[prom-data-model]: https://prometheus.io/docs/concepts/data_model/

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.connector.spanmetrics` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.connector.spanmetrics` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

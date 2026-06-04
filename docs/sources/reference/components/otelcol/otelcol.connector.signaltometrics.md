---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.connector.signaltometrics/
description: Learn about otelcol.connector.signaltometrics
labels:
  stage: experimental
  products:
    - oss
title: otelcol.connector.signaltometrics
---

# `otelcol.connector.signaltometrics`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.connector.signaltometrics` accepts spans, metrics, and logs from other `otelcol` components and generates metrics from them using [OTTL][] expressions.
You can use it to produce sums, gauges, explicit bucket histograms, and exponential (native) histograms from any signal.

{{< admonition type="note" >}}
`otelcol.connector.signaltometrics` is a wrapper over the upstream OpenTelemetry Collector Contrib `signaltometrics` connector.
Bug reports or feature requests are redirected to the upstream repository if necessary.
{{< /admonition >}}

You can specify multiple `otelcol.connector.signaltometrics` components by giving them different labels.

## Usage

```alloy
otelcol.connector.signaltometrics "<LABEL>" {
  // One or more of spans, datapoints, or logs blocks.
  spans {
    name = "<METRIC_NAME>"
    // One of histogram, exponential_histogram, sum, or gauge.
    sum {
      value = "<OTTL_VALUE_EXPRESSION>"
    }
  }

  output {
    metrics = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.connector.signaltometrics`:

| Name         | Type     | Description                                                   | Default       | Required |
| ------------ | -------- | ------------------------------------------------------------- | ------------- | -------- |
| `error_mode` | `string` | How to react to errors when processing OTTL expressions.      | `"propagate"` | no       |

The `error_mode` argument determines how the connector reacts to errors that occur while evaluating an OTTL condition or expression during runtime data consumption.
This setting doesn't affect errors during OTTL parsing at configuration time, which always cause a startup failure.
You can set `error_mode` to one of the following values:

* `propagate`: Return the error up the pipeline. This results in the payload being dropped from {{< param "PRODUCT_NAME" >}}.
* `ignore`: Ignore the error and continue processing. If an error occurs, the record is skipped and the error is logged.
* `silent`: Ignore the error and continue processing. If an error occurs, the record is skipped and the error isn't logged.

## Blocks

You can use the following blocks with `otelcol.connector.signaltometrics`:

{{< docs/alloy-config >}}

| Block                                                              | Description                                                                 | Required |
| ------------------------------------------------------------------ | --------------------------------------------------------------------------- | -------- |
| [`output`][output]                                                 | Configures where to send received telemetry data.                           | yes      |
| [`datapoints`][datapoints]                                         | Configures a metric generated from metric data points.                      | no       |
| `datapoints` > [`attributes`][attributes]                          | Configures the attributes to group the generated metric by.                 | no       |
| `datapoints` > [`exponential_histogram`][exponential_histogram]    | Generates an exponential histogram metric.                                  | no       |
| `datapoints` > [`gauge`][gauge]                                    | Generates a gauge metric.                                                   | no       |
| `datapoints` > [`histogram`][histogram]                            | Generates an explicit bucket histogram metric.                              | no       |
| `datapoints` > [`include_resource_attributes`][include]            | Configures the resource attributes to include in the generated metric.      | no       |
| `datapoints` > [`sum`][sum]                                        | Generates a sum metric.                                                     | no       |
| [`debug_metrics`][debug_metrics]                                   | Configures the metrics that this component generates to monitor its state.  | no       |
| [`logs`][logs]                                                     | Configures a metric generated from log records.                             | no       |
| `logs` > [`attributes`][attributes]                                | Configures the attributes to group the generated metric by.                 | no       |
| `logs` > [`exponential_histogram`][exponential_histogram]          | Generates an exponential histogram metric.                                  | no       |
| `logs` > [`gauge`][gauge]                                          | Generates a gauge metric.                                                   | no       |
| `logs` > [`histogram`][histogram]                                  | Generates an explicit bucket histogram metric.                              | no       |
| `logs` > [`include_resource_attributes`][include]                  | Configures the resource attributes to include in the generated metric.      | no       |
| `logs` > [`sum`][sum]                                              | Generates a sum metric.                                                     | no       |
| [`spans`][spans]                                                   | Configures a metric generated from spans.                                   | no       |
| `spans` > [`attributes`][attributes]                               | Configures the attributes to group the generated metric by.                 | no       |
| `spans` > [`exponential_histogram`][exponential_histogram]         | Generates an exponential histogram metric.                                  | no       |
| `spans` > [`gauge`][gauge]                                         | Generates a gauge metric.                                                   | no       |
| `spans` > [`histogram`][histogram]                                 | Generates an explicit bucket histogram metric.                              | no       |
| `spans` > [`include_resource_attributes`][include]                 | Configures the resource attributes to include in the generated metric.      | no       |
| `spans` > [`sum`][sum]                                             | Generates a sum metric.                                                     | no       |

You must define at least one `spans`, `datapoints`, or `logs` block.

[output]: #output
[datapoints]: #datapoints
[logs]: #logs
[spans]: #spans
[attributes]: #attributes
[include]: #include_resource_attributes
[histogram]: #histogram
[exponential_histogram]: #exponential_histogram
[sum]: #sum
[gauge]: #gauge
[debug_metrics]: #debug_metrics

{{< /docs/alloy-config >}}

### `spans`

The `spans` block configures a metric generated from spans.

You can specify multiple `spans` blocks to define different metrics.

| Name          | Type           | Description                                            | Default | Required |
| ------------- | -------------- | ------------------------------------------------------ | ------- | -------- |
| `name`        | `string`       | Name of the generated metric.                          |         | yes      |
| `conditions`  | `list(string)` | OTTL conditions, ORed together, that select the data.  | `[]`    | no       |
| `description` | `string`       | Description of the generated metric.                   | `""`    | no       |
| `unit`        | `string`       | Unit associated with the generated metric.             | `""`    | no       |

Each `spans` block must contain exactly one of the [`histogram`][histogram], [`exponential_histogram`][exponential_histogram], [`sum`][sum], or [`gauge`][gauge] blocks, which defines the type of metric to generate.

The `conditions` argument accepts a list of [OTTL][] conditions that are ORed together.
The connector only processes data into the metric if at least one condition evaluates to `true`.
If you don't specify any conditions, the connector processes all data.

### `datapoints`

The `datapoints` block configures a metric generated from metric data points.

You can specify multiple `datapoints` blocks to define different metrics.

This block shares the same configuration structure as [`spans`][spans].
Refer to the [`spans`][spans] block documentation for the complete list of supported arguments and blocks.

### `logs`

The `logs` block configures a metric generated from log records.

You can specify multiple `logs` blocks to define different metrics.

This block shares the same configuration structure as [`spans`][spans].
Refer to the [`spans`][spans] block documentation for the complete list of supported arguments and blocks.

### `attributes`

The `attributes` block specifies a data point attribute to group the generated metric by.

Each unique combination of attribute values generates a separate data point on the metric.
You can specify multiple `attributes` blocks within a `spans`, `datapoints`, or `logs` block to group by multiple attributes.

The following arguments are supported:

| Name            | Type      | Description                                                  | Default | Required |
| --------------- | --------- | ------------------------------------------------------------ | ------- | -------- |
| `key`           | `string`  | Attribute key name.                                          |         | yes      |
| `default_value` | `any`     | Value used when the attribute is missing from the data.      |         | no       |
| `optional`      | `boolean` | Generate the metric even when the attribute is missing.      | `false` | no       |

You can set at most one of `default_value` or `optional` for a given attribute.
If neither is set, data points that don't have the attribute are dropped from the generated metric.

### `include_resource_attributes`

The `include_resource_attributes` block specifies a resource attribute to include in the generated metric.

If you don't specify any `include_resource_attributes` blocks, the connector includes all resource attributes.
If you specify one or more blocks, the connector includes only the listed resource attributes.

This block supports the same arguments as the [`attributes`][attributes] block.

### `histogram`

The `histogram` block generates an explicit bucket histogram metric.

| Name      | Type            | Description                                        | Default                  | Required |
| --------- | --------------- | -------------------------------------------------- | ------------------------ | -------- |
| `value`   | `string`        | OTTL expression that produces the recorded value.  |                          | yes      |
| `buckets` | `list(number)`  | Explicit bucket boundaries for the histogram.      | See below                | no       |
| `count`   | `string`        | OTTL expression that produces the recorded count.  | `""`                     | no       |

If you don't specify `buckets`, the connector uses the following default boundaries:
`[2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10000, 15000]`.

If you don't specify `count`, each matching record increments the count by one.

### `exponential_histogram`

The `exponential_histogram` block generates a base-2 exponential (native) histogram metric.

| Name       | Type     | Description                                        | Default | Required |
| ---------- | -------- | -------------------------------------------------- | ------- | -------- |
| `value`    | `string` | OTTL expression that produces the recorded value.  |         | yes      |
| `count`    | `string` | OTTL expression that produces the recorded count.  | `""`    | no       |
| `max_size` | `number` | Maximum number of buckets per positive or negative range. | `160`   | no       |

If you don't specify `count`, each matching record increments the count by one.

### `sum`

The `sum` block generates a sum metric.

| Name    | Type     | Description                                       | Default | Required |
| ------- | -------- | ------------------------------------------------- | ------- | -------- |
| `value` | `string` | OTTL expression that produces the summed value.   |         | yes      |

### `gauge`

The `gauge` block generates a gauge metric.

| Name    | Type     | Description                                       | Default | Required |
| ------- | -------- | ------------------------------------------------- | ------- | -------- |
| `value` | `string` | OTTL expression that produces the gauge value.    |         | yes      |

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for traces, metrics, and logs.
The connector generates metrics from the received telemetry according to the configured blocks and emits them to the components specified in `output`.

## Component health

`otelcol.connector.signaltometrics` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.connector.signaltometrics` doesn't expose any component-specific debug information.

## Examples

### Generate a native histogram from log lines

The following example generates an exponential (native) histogram of log body lengths and forwards it to Prometheus remote write.

```alloy
otelcol.connector.signaltometrics "default" {
  logs {
    name        = "log.body.length"
    description = "Distribution of log body lengths"
    exponential_histogram {
      value = "Len(body)"
    }
  }

  output {
    metrics = [otelcol.exporter.prometheus.default.input]
  }
}

otelcol.exporter.prometheus "default" {
  forward_to = [prometheus.remote_write.default.receiver]
}

prometheus.remote_write "default" {
  endpoint {
    url = "http://localhost:9090/api/v1/write"
  }
}
```

### Generate metrics from spans

The following example generates a span duration histogram and a request counter grouped by HTTP method.

```alloy
otelcol.connector.signaltometrics "default" {
  spans {
    name        = "span.duration"
    description = "Span duration in milliseconds"
    unit        = "ms"
    histogram {
      value = "Milliseconds(end_time - start_time)"
    }
  }

  spans {
    name        = "http.server.request.count"
    description = "Number of HTTP server requests"
    attributes {
      key = "http.request.method"
    }
    sum {
      value = "1"
    }
  }

  output {
    metrics = [otelcol.exporter.otlphttp.default.input]
  }
}
```

[OTTL]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.connector.signaltometrics` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.connector.signaltometrics` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

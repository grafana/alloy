---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.transform/
aliases:
  - ../otelcol.processor.transform/ # /docs/alloy/latest/reference/otelcol.processor.transform/
description: Learn about otelcol.processor.transform
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.processor.transform
---

# `otelcol.processor.transform`

`otelcol.processor.transform` accepts telemetry data from other `otelcol` components and modifies it using the [OpenTelemetry Transformation Language (OTTL)][OTTL].
OTTL statements consist of [OTTL functions][], which act on paths.
A path is a reference to a telemetry data such as:

* Resource attributes.
* Instrumentation scope name.
* Span attributes.

In addition to the [standard OTTL functions][OTTL functions], the processor defines its own functions to help with transformations specific to this processor.

Metrics-only functions:

* [`convert_sum_to_gauge`][convert_sum_to_gauge]
* [`convert_gauge_to_sum`][convert_gauge_to_sum]
* [`extract_count_metric`][extract_count_metric]
* [`extract_sum_metric`][extract_sum_metric]
* [`convert_summary_count_val_to_sum`][convert_summary_count_val_to_sum]
* [`convert_summary_quantile_val_to_gauge`][convert_summary_quantile_val_to_gauge]
* [`convert_summary_sum_val_to_sum`][convert_summary_sum_val_to_sum]
* [`copy_metric`][copy_metric]
* [`scale_metric`][scale_metric]
* [`aggregate_on_attributes`][aggregate_on_attributes]
* [`convert_exponential_histogram_to_histogram`][convert_exponential_histogram_to_histogram]
* [`aggregate_on_attribute_value`][aggregate_on_attribute_value]
* [`merge_histogram_buckets`][merge_histogram_buckets]

Traces-only functions:

* [`set_semconv_span_name`][set_semconv_span_name]

[OTTL][] statements can also contain constructs such as:

* [Booleans][OTTL booleans]:
  * `not true`
  * `not IsMatch(name, "http_.*")`
* [Boolean Expressions][OTTL boolean expressions] consisting of a `where` followed by one or more boolean values:
  * `set(attributes["whose_fault"], "ours") where attributes["http.status"] == 500`
  * `set(attributes["whose_fault"], "theirs") where attributes["http.status"] == 400 or attributes["http.status"] == 404`
* [Math expressions][OTTL math expressions]:
  * `1 + 1`
  * `end_time_unix_nano - start_time_unix_nano`
  * `sum([1, 2, 3, 4]) + (10 / 1) - 1`

{{< admonition type="note" >}}
There are two ways of inputting strings in {{< param "PRODUCT_NAME" >}} configuration files:

* Using quotation marks ([normal {{< param "PRODUCT_NAME" >}} syntax strings][strings]).
  Characters such as `\` and `"` must be escaped by preceding them with a `\` character.
* Using backticks ([raw {{< param "PRODUCT_NAME" >}} syntax strings][raw-strings]).
  No characters must be escaped.
  However, it's not possible to have backticks inside the string.

For example, the OTTL statement `set(description, "Sum") where type == "Sum"` can be written as:

* A normal {{< param "PRODUCT_NAME" >}} syntax string: `"set(description, \"Sum\") where type == \"Sum\""`.
* A raw {{< param "PRODUCT_NAME" >}} syntax string: ``` `set(description, "Sum") where type == "Sum"` ```.

Raw strings are generally more convenient for writing OTTL statements.

[strings]: ../../../../get-started/configuration-syntax/expressions/types_and_values/#strings
[raw-strings]: ../../../../get-started/configuration-syntax/expressions/types_and_values/#raw-strings
{{< /admonition >}}

{{< admonition type="note" >}}
`otelcol.processor.transform` is a wrapper over the upstream OpenTelemetry Collector [`transform`][] processor.
If necessary, bug reports or feature requests will be redirected to the upstream repository.

[`transform`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor
{{< /admonition >}}

You can specify multiple `otelcol.processor.transform` components by giving them different labels.

{{< admonition type="warning" >}}
`otelcol.processor.transform` allows you to modify all aspects of your telemetry.
Some specific risks are given below, but this isn't an exhaustive list.
It's important to understand your data before using this processor.

* [Unsound Transformations][]: Transformations between metric data types aren't defined in the [metrics data model][].
  To use these functions, you must understand the incoming data and know that it can be meaningfully converted to a new metric data type or can be used to create new metrics.
  * Although OTTL allows you to use the `set` function with `metric.data_type`, its implementation in the transform processor is a [no-op][].
    To modify a data type, you must use a specific function such as `convert_gauge_to_sum`.
* [Identity Conflict][]: Transformation of metrics can potentially affect a metric's identity, leading to an Identity Crisis.
  Be especially cautious when transforming a metric name and when reducing or changing existing attributes.
  Adding new attributes is safe.
* [Orphaned Telemetry][]: The processor allows you to modify `span_id`, `trace_id`, and `parent_span_id` for traces and `span_id`, and `trace_id` logs.
  Modifying these fields could lead to orphaned spans or logs.

[Unsound Transformations]: https://github.com/open-telemetry/opentelemetry-collector/blob/{{< param "OTEL_VERSION" >}}/docs/standard-warnings.md#unsound-transformations
[Identity Conflict]: https://github.com/open-telemetry/opentelemetry-collector/blob/{{< param "OTEL_VERSION" >}}/docs/standard-warnings.md#identity-conflict
[Orphaned Telemetry]: https://github.com/open-telemetry/opentelemetry-collector/blob/{{< param "OTEL_VERSION" >}}/docs/standard-warnings.md#orphaned-telemetry
[no-op]: https://en.wikipedia.org/wiki/NOP_(code)
[metrics data model]: https://github.com/open-telemetry/opentelemetry-specification/blob/main//specification/metrics/data-model.md
{{< /admonition >}}

## Usage

```alloy
otelcol.processor.transform "<LABEL>" {
  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments
[Arguments]: #arguments

You can use the following argument with `otelcol.processor.transform`:

| Name         | Type     | Description                                                        | Default       | Required |
|--------------|----------|--------------------------------------------------------------------|---------------|----------|
| `error_mode` | `string` | How to react to errors if they occur while processing a statement. | `"propagate"` | no       |

The supported values for `error_mode` are:

* `ignore`: Ignore errors returned by conditions, log them, and continue on to the next condition.
  This is the recommended mode.
* `silent`: Ignore errors returned by conditions, don't log them, and continue on to the next condition.
* `propagate`: Return the error up the pipeline.
  This will result in the payload being dropped from {{< param "PRODUCT_NAME" >}}.

## Blocks

You can use the following blocks with `otelcol.processor.transform`:

| Block                                    | Description                                                                                   | Required |
|------------------------------------------|-----------------------------------------------------------------------------------------------|----------|
| [`output`][output]                       | Configures where to send received telemetry data.                                             | yes      |
| [`debug_metrics`][debug_metrics]         | Configures the metrics that this component generates to monitor its state.                    | no       |
| [`log_statements`][log_statements]       | Statements which transform logs.                                                              | no       |
| [`metric_statements`][metric_statements] | Statements which transform metrics.                                                           | no       |
| [`statements`][statements]               | Statements which transform logs, metrics, and traces without specifying a context explicitly. | no       |
| [`trace_statements`][trace_statements]   | Statements which transform traces.                                                            | no       |

[trace_statements]: #trace_statements
[metric_statements]: #metric_statements
[log_statements]: #log_statements
[output]: #output
[debug_metrics]: #debug_metrics
[statements]: #statements
[OTTL Context]: #ottl-context

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `log_statements`

The `log_statements` block specifies statements which transform log telemetry signals.
Multiple `log_statements` blocks can be specified.

| Name         | Type           | Description                                                        | Default | Required |
|--------------|----------------|--------------------------------------------------------------------|---------|----------|
| `context`    | `string`       | OTTL Context to use when interpreting the associated statements.   |         | yes      |
| `statements` | `list(string)` | A list of OTTL statements.                                         |         | yes      |
| `conditions` | `list(string)` | Conditions for the statements to be executed.                      |         | no       |
| `error_mode` | `string`       | How to react to errors if they occur while processing a statement. |         | no       |

The supported values for `context` are:

* `resource`: Use when interacting only with OTLP resources (for example, resource attributes).
* `scope`: Use when interacting only with OTLP instrumentation scope (for example, the name of the instrumentation scope).
* `log`: Use when interacting only with OTLP logs.

Refer to [OTTL Context][] for more information about how to use contexts.

`conditions` is a list of multiple `where` clauses which will be processed as global conditions for the accompanying set of statements. 
The conditions are ORed together, which means only one condition needs to evaluate to true in order for the statements 
(including their individual `where` clauses) to be executed.

The allowed values for `error_mode` are the same as the ones documented in the [Arguments][] section.
If `error_mode` is not specified in `log_statements`, the top-level `error_mode` is applied.

### `metric_statements`

The `metric_statements` block specifies statements which transform metric telemetry signals.
Multiple `metric_statements` blocks can be specified.

| Name         | Type           | Description                                                        | Default | Required |
|--------------|----------------|--------------------------------------------------------------------|---------|----------|
| `context`    | `string`       | OTTL Context to use when interpreting the associated statements.   |         | yes      |
| `statements` | `list(string)` | A list of OTTL statements.                                         |         | yes      |
| `conditions` | `list(string)` | Conditions for the statements to be executed.                      |         | no       |
| `error_mode` | `string`       | How to react to errors if they occur while processing a statement. |         | no       |

The supported values for `context` are:

* `resource`: Use when interacting only with OTLP resources (for example, resource attributes).
* `scope`: Use when interacting only with OTLP instrumentation scope (for example, the name of the instrumentation scope).
* `metric`: Use when interacting only with individual OTLP metrics.
* `datapoint`: Use when interacting only with individual OTLP metric data points.

Refer to [OTTL Context][] for more information about how to use contexts.

`conditions` is a list of multiple `where` clauses which will be processed as global conditions for the accompanying set of statements. 
The conditions are ORed together, which means only one condition needs to evaluate to true in order for the statements 
(including their individual `where` clauses) to be executed.

The allowed values for `error_mode` are the same as the ones documented in the [Arguments][] section.
If `error_mode` is not specified in `metric_statements`, the top-level `error_mode` is applied.

### `statements`

The `statements` block specifies statements which transform logs, metrics, or traces telemetry signals.
There is no `context` configuration argument - the context will be inferred from the statement.
This inference is based on the path names, functions, and enums present in the statements.
At least one context must be capable of parsing all statements.

The `statements` block can replace the `log_statements`, `metric_statements`, and `trace_statements` blocks.
It can also be used alongside them.

| Name     | Type           | Description                                        | Default | Required |
|----------|----------------|----------------------------------------------------|---------|----------|
| `log`    | `list(string)` | A list of OTTL statements which transform logs.    | `[]`    | no       |
| `metric` | `list(string)` | A list of OTTL statements which transform metrics. | `[]`    | no       |
| `trace`  | `list(string)` | A list of OTTL statements which transform traces.  | `[]`    | no       |

The inference happens automatically because path names are prefixed with the context name.
In the following example, the inferred context value is `datapoint`, as it's the only context that supports parsing both datapoint and metric paths:

```alloy
statements {
    metric = [`set(metric.description, "test passed") where datapoint.attributes["test"] == "pass"`]
}
```

In the following example, the inferred context is `metric`, as `metric` is the context capable of parsing both metric and resource data:

```alloy
statements {
    metric = [
        `resource.attributes["test"], "passed"`,
        `set(metric.description, "test passed")`,
    ]
}
```

The primary benefit of context inference is that it enhances the efficiency of statement processing by linking them to the most suitable context.
This optimization ensures that data transformations are both accurate and performant,
leveraging the hierarchical structure of contexts to avoid unnecessary iterations and improve overall processing efficiency.
All of this happens automatically, leaving you to write OTTL statements without worrying about contexts.

### `trace_statements`

The `trace_statements` block specifies statements which transform trace telemetry signals.
Multiple `trace_statements` blocks can be specified.

| Name         | Type           | Description                                                        | Default | Required |
|--------------|----------------|--------------------------------------------------------------------|---------|----------|
| `context`    | `string`       | OTTL Context to use when interpreting the associated statements.   |         | yes      |
| `statements` | `list(string)` | A list of OTTL statements.                                         |         | yes      |
| `conditions` | `list(string)` | Conditions for the statements to be executed.                      |         | no       |
| `error_mode` | `string`       | How to react to errors if they occur while processing a statement. |         | no       |

The supported values for `context` are:

* `resource`: Use when interacting only with OTLP resources (for example, resource attributes).
* `scope`: Use when interacting only with OTLP instrumentation scope (for example, the name of the instrumentation scope).
* `span`: Use when interacting only with OTLP spans.
* `spanevent`: Use when interacting only with OTLP span events.

Refer to [OTTL Context][] for more information about how to use contexts.

`conditions` is a list of multiple `where` clauses which will be processed as global conditions for the accompanying set of statements. 
The conditions are ORed together, which means only one condition needs to evaluate to true in order for the statements 
(including their individual `where` clauses) to be executed.

The allowed values for `error_mode` are the same as the ones documented in the [Arguments][] section.
If `error_mode` is not specified in `trace_statements`, the top-level `error_mode` is applied.

### OTTL Context

Each context allows the transformation of its type of telemetry.
For example, statements associated with a `resource` context will be able to transform the resource's `attributes` and `dropped_attributes_count`.

Each type of `context` defines its own paths and enums specific to that context.
Refer to the OpenTelemetry documentation for a list of paths and enums for each context:

* [`resource`][OTTL resource context]
* [`scope`][OTTL scope context]
* [`span`][OTTL span context]
* [`spanevent`][OTTL spanevent context]
* [`log`][OTTL log context]
* [`metric`][OTTL metric context]
* [`datapoint`][OTTL datapoint context]

Contexts __NEVER__ supply access to individual items "lower" in the protobuf definition.

* This means statements associated to a `resource` __WILL NOT__ be able to access the underlying instrumentation scopes.
* This means statements associated to a `scope` __WILL NOT__ be able to access the underlying telemetry slices (spans, metrics, or logs).
* Similarly, statements associated to a `metric` __WILL NOT__ be able to access individual datapoints, but can access the entire datapoints slice.
* Similarly, statements associated to a `span` __WILL NOT__ be able to access individual SpanEvents, but can access the entire SpanEvents slice.

For practical purposes, this means that a context can't make decisions on its telemetry based on telemetry "lower" in the structure.
For example, __the following context statement isn't possible__ because it attempts to use individual datapoint attributes in the condition of a statement associated to a `metric`:

```alloy
metric_statements {
  context = "metric"
  statements = [
    "set(description, \"test passed\") where datapoints.attributes[\"test\"] == \"pass\"",
  ]
}
```

Context __ALWAYS__ supply access to the items "higher" in the protobuf definition that are associated to the telemetry being transformed.

* This means that statements associated to a `datapoint` have access to a datapoint's metric, instrumentation scope, and resource.
* This means that statements associated to a `spanevent` have access to a spanevent's span, instrumentation scope, and resource.
* This means that statements associated to a `span`/`metric`/`log` have access to the telemetry's instrumentation scope, and resource.
* This means that statements associated to a `scope` have access to the scope's resource.

For example, __the following context statement is possible__ because `datapoint` statements can access the datapoint's metric.

```alloy
metric_statements {
  context = "datapoint"
  statements = [
    "set(metric.description, \"test passed\") where attributes[\"test\"] == \"pass\"",
  ]
}
```

The protobuf definitions for OTLP signals are maintained on GitHub:

* [traces][traces protobuf]
* [metrics][metrics protobuf]
* [logs][logs protobuf]

Whenever possible, associate your statements to the context which the statement intens to transform.
The contexts are nested, and the higher-level contexts don't have to iterate through any of the contexts at a lower level.
For example, although you can modify resource attributes associated to a span using the `span` context, it's more efficient to use the `resource` context.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.processor.transform` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.processor.transform` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.processor.transform` doesn't expose any component-specific debug metrics.

## Examples

### Perform a transformation if an attribute doesn't exist

This example sets the attribute `test` to `pass` if the attribute `test` doesn't exist.

```alloy
otelcol.processor.transform "default" {
  error_mode = "ignore"

  trace_statements {
    context = "span"
    statements = [
      // Accessing a map with a key that doesn't exist will return nil.
      `set(attributes["test"], "pass") where attributes["test"] == nil`,
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```

### Rename a resource attribute

The are two ways to rename an attribute key.
One way is to set a new attribute and delete the old one:

```alloy
otelcol.processor.transform "default" {
  error_mode = "ignore"

  trace_statements {
    context = "resource"
    statements = [
      `set(attributes["namespace"], attributes["k8s.namespace.name"])`,
      `delete_key(attributes, "k8s.namespace.name")`,
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```

Another way is to update the key using regular expressions:

```alloy
otelcol.processor.transform "default" {
  error_mode = "ignore"

  trace_statements {
    context = "resource"
    statements = [
     `replace_all_patterns(attributes, "key", "k8s\\.namespace\\.name", "namespace")`,
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```

### Create an attribute from the contents of a log body

This example sets the attribute `body` to the value of the log body:

```alloy
otelcol.processor.transform "default" {
  error_mode = "ignore"

  log_statements {
    context = "log"
    statements = [
      `set(attributes["body"], body)`,
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```

### Combine two attributes

This example sets the attribute `test` to the value of attributes `service.name` and `service.version` combined.

```alloy
otelcol.processor.transform "default" {
  error_mode = "ignore"

  trace_statements {
    context = "resource"
    statements = [
      // The Concat function combines any number of strings, separated by a delimiter.
      `set(attributes["test"], Concat([attributes["service.name"], attributes["service.version"]], " "))`,
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```

### Parsing JSON logs

Given the following JSON body:

```json
{
  "name": "log",
  "attr1": "example value 1",
  "attr2": "example value 2",
  "nested": {
    "attr3": "example value 3"
  }
}
```

You can add specific fields as attributes on the log:

```alloy
otelcol.processor.transform "default" {
  error_mode = "ignore"

  log_statements {
    context = "log"

    statements = [
      // Parse body as JSON and merge the resulting map with the cache map, ignoring non-json bodies.
      // cache is a field exposed by OTTL that is a temporary storage place for complex operations.
      `merge_maps(cache, ParseJSON(body), "upsert") where IsMatch(body, "^\\{")`,

      // Set attributes using the values merged into cache.
      // If the attribute doesn't exist in cache then nothing happens.
      `set(attributes["attr1"], cache["attr1"])`,
      `set(attributes["attr2"], cache["attr2"])`,

      // To access nested maps you can chain index ([]) operations.
      // If nested or attr3 do no exist in cache then nothing happens.
      `set(attributes["nested.attr3"], cache["nested"]["attr3"])`,
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```

### Various transformations of attributes and status codes

The example takes advantage of context efficiency by grouping transformations with the context which it intends to transform.

```alloy
otelcol.receiver.otlp "default" {
  http {}
  grpc {}

  output {
    metrics = [otelcol.processor.transform.default.input]
    logs    = [otelcol.processor.transform.default.input]
    traces  = [otelcol.processor.transform.default.input]
  }
}

otelcol.processor.transform "default" {
  error_mode = "ignore"

  trace_statements {
    context = "resource"
    statements = [
      `keep_keys(attributes, ["service.name", "service.namespace", "cloud.region", "process.command_line"])`,
      `replace_pattern(attributes["process.command_line"], "password\\=[^\\s]*(\\s?)", "password=***")`,
      `limit(attributes, 100, [])`,
      `truncate_all(attributes, 4096)`,
    ]
  }

  trace_statements {
    context = "span"
    statements = [
      `set(status.code, 1) where attributes["http.path"] == "/health"`,
      `set(name, attributes["http.route"])`,
      `replace_match(attributes["http.target"], "/user/*/list/*", "/user/{userId}/list/{listId}")`,
      `limit(attributes, 100, [])`,
      `truncate_all(attributes, 4096)`,
    ]
  }

  metric_statements {
    context = "resource"
    statements = [
      `keep_keys(attributes, ["host.name"])`,
      `truncate_all(attributes, 4096)`,
    ]
  }

  metric_statements {
    context = "metric"
    statements = [
      `set(description, "Sum") where type == "Sum"`,
      `convert_sum_to_gauge() where name == "system.processes.count"`,
      `convert_gauge_to_sum("cumulative", false) where name == "prometheus_metric"`,
      `aggregate_on_attributes("sum") where name == "system.memory.usage"`,
    ]
  }

  metric_statements {
    context = "datapoint"
    statements = [
      `limit(attributes, 100, ["host.name"])`,
      `truncate_all(attributes, 4096)`,
    ]
  }

  log_statements {
    context = "resource"
    statements = [
      `keep_keys(attributes, ["service.name", "service.namespace", "cloud.region"])`,
    ]
  }

  log_statements {
    context = "log"
    statements = [
      `set(severity_text, "FAIL") where body == "request failed"`,
      `replace_all_matches(attributes, "/user/*/list/*", "/user/{userId}/list/{listId}")`,
      `replace_all_patterns(attributes, "value", "/account/\\d{4}", "/account/{accountId}")`,
      `set(body, attributes["http.route"])`,
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = sys.env("OTLP_ENDPOINT")
  }
}
```

### Using conditions

This example only runs the statements if the conditions are met:

```alloy
otelcol.processor.transform "default" {
  error_mode = "ignore"

  metric_statements {
    context = "metric"
    statements = [
      `set(metric.description, "Sum")`,
    ]
    conditions = [
      `metric.type == METRIC_DATA_TYPE_SUM`,
    ]
  }

  log_statements {
    context = "log"
    statements = [
      `set(log.body, log.attributes["http.route"])`,
    ]
    conditions = [
      `IsMap(log.body) and log.body["object"] != nil`,
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```

[traces protobuf]: https://github.com/open-telemetry/opentelemetry-proto/blob/v1.0.0/opentelemetry/proto/trace/v1/trace.proto
[metrics protobuf]: https://github.com/open-telemetry/opentelemetry-proto/blob/v1.0.0/opentelemetry/proto/metrics/v1/metrics.proto
[logs protobuf]: https://github.com/open-telemetry/opentelemetry-proto/blob/v1.0.0/opentelemetry/proto/logs/v1/logs.proto

[OTTL]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/README.md
[OpenTelemetry Transformation Language (OTTL)]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/README.md
[OTTL functions]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/ottlfuncs/README.md
[convert_sum_to_gauge]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#convert_sum_to_gauge
[convert_gauge_to_sum]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#convert_gauge_to_sum
[extract_count_metric]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#extract_count_metric
[extract_sum_metric]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#extract_sum_metric
[convert_summary_count_val_to_sum]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#convert_summary_count_val_to_sum
[convert_summary_quantile_val_to_gauge]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#convert_summary_quantile_val_to_gauge
[convert_summary_sum_val_to_sum]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#convert_summary_sum_val_to_sum
[copy_metric]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#copy_metric
[scale_metric]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#scale_metric
[aggregate_on_attributes]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#aggregate_on_attributes
[merge_histogram_buckets]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#merge_histogram_buckets
[set_semconv_span_name]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#set_semconv_span_name
[convert_exponential_histogram_to_histogram]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#convert_exponential_histogram_to_histogram
[aggregate_on_attribute_value]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/processor/transformprocessor#aggregate_on_attribute_value
[OTTL booleans]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/pkg/ottl#booleans
[OTTL math expressions]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/pkg/ottl#math-expressions
[OTTL boolean expressions]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/pkg/ottl#boolean-expressions
[OTTL resource context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottlresource/README.md
[OTTL scope context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottlscope/README.md
[OTTL span context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottlspan/README.md
[OTTL spanevent context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottlspanevent/README.md
[OTTL metric context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottlmetric/README.md
[OTTL datapoint context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottldatapoint/README.md
[OTTL log context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottllog/README.md

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.transform` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.transform` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

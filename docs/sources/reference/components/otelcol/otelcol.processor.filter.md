---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.filter/
aliases:
  - ../otelcol.processor.filter/ # /docs/alloy/latest/reference/otelcol.processor.filter/
description: Learn about otelcol.processor.filter
title: otelcol.processor.filter
---

# otelcol.processor.filter

`otelcol.processor.filter` accepts and filters telemetry data from other `otelcol`
components using the [OpenTelemetry Transformation Language (OTTL)][OTTL].
If any of the OTTL statements evaluates to true, the telemetry data is dropped.

OTTL statements consist of [OTTL Converter functions][], which act on paths.
A path is a reference to a telemetry data such as:
* Resource attributes.
* Instrumentation scope name.
* Span attributes.

In addition to the [standard OTTL Converter functions][OTTL Converter functions], the following metrics-only functions are used exclusively by the processor:
* [HasAttrKeyOnDataPoint][]
* [HasAttrOnDataPoint][]

[OTTL][] statements used in `otelcol.processor.filter` mostly contain constructs such as:
* [Booleans][OTTL booleans]:
  * `not true`
  * `not IsMatch(name, "http_.*")`
* [Math expressions][OTTL math expressions]:
  * `1 + 1`
  * `end_time_unix_nano - start_time_unix_nano`
  * `sum([1, 2, 3, 4]) + (10 / 1) - 1`

{{< admonition type="note" >}}
Raw {{< param "PRODUCT_NAME" >}} syntax strings can be used to write OTTL statements.
For example, the OTTL statement `attributes["grpc"] == true` is written in {{< param "PRODUCT_NAME" >}} syntax as \`attributes["grpc"] == true\`
{{< /admonition >}}

{{< admonition type="note" >}}
`otelcol.processor.filter` is a wrapper over the upstream OpenTelemetry Collector `filter` processor.
If necessary, bug reports or feature requests will be redirected to the upstream repository.
{{< /admonition >}}

You can specify multiple `otelcol.processor.filter` components by giving them different labels.

{{< admonition type="warning" >}}
Exercise caution when using `otelcol.processor.filter`:

- Make sure you understand schema/format of the incoming data and test the configuration thoroughly.
  In general, use a configuration that is as specific as possible ensure you retain only the data you want to keep.
- [Orphaned Telemetry][]: The processor allows dropping spans. Dropping a span may lead to
  orphaned spans if the dropped span is a parent. Dropping a span may lead to orphaned logs
  if the log references the dropped span.

[Orphaned Telemetry]: https://github.com/open-telemetry/opentelemetry-collector/blob/v0.85.0/docs/standard-warnings.md#orphaned-telemetry
{{< /admonition >}}

## Usage

```alloy
otelcol.processor.filter "LABEL" {
  output {
    metrics = [...]
    logs    = [...]
    traces  = [...]
  }
}
```

## Arguments

`otelcol.processor.filter` supports the following arguments:

Name         | Type     | Description                                                        | Default       | Required
------------ | -------- | ------------------------------------------------------------------ | ------------- | --------
`error_mode` | `string` | How to react to errors if they occur while processing a statement. | `"propagate"` | no

The supported values for `error_mode` are:
* `ignore`: Ignore errors returned by conditions, log them, and continue on to the next condition. This is the recommended mode.
* `silent`: Ignore errors returned by conditions, do not log them, and continue on to the next condition.
* `propagate`: Return the error up the pipeline. This will result in the payload being dropped from {{< param "PRODUCT_NAME" >}}.

## Blocks

The following blocks are supported inside the definition of
`otelcol.processor.filter`:

Hierarchy | Block       | Description                                       | Required
--------- | ----------- | ------------------------------------------------- | --------
traces    | [traces][]  | Statements which filter traces.                   | no
metrics   | [metrics][] | Statements which filter metrics.                  | no
logs      | [logs][]    | Statements which filter logs.                     | no
output    | [output][]  | Configures where to send received telemetry data. | yes
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

[traces]: #traces-block
[metrics]: #metrics-block
[logs]: #logs-block
[output]: #output-block
[debug_metrics]: #debug_metrics-block


### traces block

The `traces` block specifies statements that filter trace telemetry signals.
Only one `traces` block can be specified.

Name        | Type           | Description                                         | Default | Required
----------- | -------------- | --------------------------------------------------- | ------- | --------
`span`      | `list(string)` | List of OTTL statements filtering OTLP spans.       |         | no
`spanevent` | `list(string)` | List of OTTL statements filtering OTLP span events. |         | no

The syntax of OTTL statements depends on the OTTL context. See the OpenTelemetry documentation for more information:
* [OTTL span context][]
* [OTTL spanevent context][]

Statements are checked in order from "high level" to "low level" telemetry, in this order:
1. `span`
2. `spanevent`

If at least one `span` condition is satisfied, the `spanevent` conditions will not be checked.
Only one of the statements inside the list of statements has to be satisfied.

If all span events for a span are dropped, the span will be left intact.

### metrics block

The `metrics` block specifies statements that filter metric telemetry signals.
Only one `metrics` blocks can be specified.

Name        | Type           | Description                                               | Default | Required
----------- | -------------- | --------------------------------------------------------- | ------- | --------
`metric`    | `list(string)` | List of OTTL statements filtering OTLP metric.            |         | no
`datapoint` | `list(string)` | List of OTTL statements filtering OTLP metric datapoints. |         | no

The syntax of OTTL statements depends on the OTTL context. See the OpenTelemetry documentation for more information:
* [OTTL metric context][]
* [OTTL datapoint context][]

Statements are checked in order from "high level" to "low level" telemetry, in this order:
1. `metric`
2. `datapoint`

If at least one `metric` condition is satisfied, the `datapoint` conditions will not be checked.
Only one of the statements inside the list of statements has to be satisfied.

If all datapoints for a metric are dropped, the metric will also be dropped.

### logs block

The `logs` block specifies statements that filter log telemetry signals.
Only `logs` blocks can be specified.

Name            | Type           | Description                                    | Default | Required
--------------- | -------------- | ---------------------------------------------- | ------- | --------
`log_record`    | `list(string)` | List of OTTL statements filtering OTLP metric. |         | no

The syntax of OTTL statements depends on the OTTL context. See the OpenTelemetry documentation for more information:
* [OTTL log context][]

Only one of the statements inside the list of statements has to be satisfied.

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

`otelcol.processor.filter` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.processor.filter` does not expose any component-specific debug
information.

## Debug metrics

`otelcol.processor.filter` does not expose any component-specific debug metrics.

## Examples

### Drop spans which contain a certain span attribute

This example sets the attribute `test` to `pass` if the attribute `test` does not exist.

```alloy
otelcol.processor.filter "default" {
  error_mode = "ignore"

  traces {
    span = [
      "attributes[\"container.name\"] == \"app_container_1\"",
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```

Each `"` is [escaped][] with `\"` inside the {{< param "PRODUCT_NAME" >}} syntax string.

### Drop metrics based on either of two criteria

This example drops metrics which satisfy at least one of two OTTL statements:
* The metric name is `my.metric` and there is a `my_label` resource attribute with a value of `abc123 `.
* The metric is a histogram.

```alloy
otelcol.processor.filter "default" {
  error_mode = "ignore"

  metrics {
    metric = [
       "name == \"my.metric\" and resource.attributes[\"my_label\"] == \"abc123\""
       "type == METRIC_DATA_TYPE_HISTOGRAM"
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```


Some values in the {{< param "PRODUCT_NAME" >}} syntax string are [escaped][]:
* `\` is escaped with `\\`
* `"` is escaped with `\"`

### Drop non-HTTP spans and sensitive logs

```alloy
otelcol.processor.filter "default" {
  error_mode = "ignore"

  traces {
    span = [
      "attributes[\"http.request.method\"] == nil",
    ]
  }

  logs {
    log_record = [
      "IsMatch(body, \".*password.*\")",
      "severity_number < SEVERITY_NUMBER_WARN",
    ]
  }

  output {
    metrics = [otelcol.exporter.otlp.default.input]
    logs    = [otelcol.exporter.otlp.default.input]
    traces  = [otelcol.exporter.otlp.default.input]
  }
}
```

Each `"` is [escaped][] with `\"` inside the {{< param "PRODUCT_NAME" >}} syntax string.


Some values in the {{< param "PRODUCT_NAME" >}} syntax strings are [escaped][]:
* `\` is escaped with `\\`
* `"` is escaped with `\"`

[escaped]: ../../../../get-started/configuration-syntax/expressions/types_and_values/#strings


[OTTL]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.85.0/pkg/ottl/README.md
[OTTL span context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottlspan/README.md
[OTTL spanevent context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottlspanevent/README.md
[OTTL metric context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottlmetric/README.md
[OTTL datapoint context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottldatapoint/README.md
[OTTL log context]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/{{< param "OTEL_VERSION" >}}/pkg/ottl/contexts/ottllog/README.md
[OTTL Converter functions]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/ottlfuncs#converters
[HasAttrKeyOnDataPoint]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/filterprocessor/README.md#hasattrkeyondatapoint
[HasAttrOnDataPoint]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/filterprocessor/README.md#hasattrondatapoint
[OTTL booleans]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.85.0/pkg/ottl#booleans
[OTTL math expressions]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.85.0/pkg/ottl#math-expressions
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.processor.filter` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.processor.filter` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

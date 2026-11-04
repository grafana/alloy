---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.connector.exceptions/
description: Learn about otelcol.connector.exceptions
labels:
  stage: experimental
  products:
    - oss
review_date: 2026-10-15
title: otelcol.connector.exceptions
---

# `otelcol.connector.exceptions`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.connector.exceptions` accepts traces and logs from other `otelcol` components and outputs logs and metrics that describe the exceptions recorded in them.

{{< admonition type="note" >}}
`otelcol.connector.exceptions` is a wrapper over the upstream OpenTelemetry Collector Contrib [`exceptions`][] connector.
Bug reports or feature requests are redirected to the upstream repository if necessary.

[`exceptions`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/exceptionsconnector
{{< /admonition >}}

You can specify multiple `otelcol.connector.exceptions` components by giving them different labels.

`otelcol.connector.exceptions` treats a span event named `exception` and a log record with the event name `exception` as exceptions.
These follow the semantic conventions for exceptions in [spans][exceptions-spans] and [logs][exceptions-logs].

Depending on the signals you set in the `output` block, `otelcol.connector.exceptions` does the following:

* **Traces to logs**: Converts each `exception` span event into a log record that follows the semantic conventions for exceptions in logs.
  The log record carries the span attributes, the trace and span IDs, and the `exception.type`, `exception.message`, and `exception.stacktrace` attributes of the event.
  Use this to move exception tracking from spans to logs while applications still report exceptions as span events.
* **Logs to logs**: Forwards log records with the event name `exception` and drops all other log records.
  Use this to route exception logs to a dedicated backend.
* **Traces to metrics** and **logs to metrics**: Increments an `exceptions` counter metric for every exception found in spans or log records.

Traces to logs, traces to metrics, and logs to metrics require the `service.name` resource attribute.
`otelcol.connector.exceptions` skips any resource that doesn't set it.
Logs to logs forwards exception log records whether or not the resource sets `service.name`.

[exceptions-spans]: https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-spans/
[exceptions-logs]: https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-logs/

## Usage

```alloy
otelcol.connector.exceptions "<LABEL>" {
  output {
    logs    = [...]
    metrics = [...]
  }
}
```

## Arguments

You can use the following argument with `otelcol.connector.exceptions`:

| Name                         | Type   | Description                                          | Default | Required |
|------------------------------|--------|------------------------------------------------------|---------|----------|
| `disable_default_dimensions` | `bool` | Emits metric data points with no default dimensions. | `false` | no       |

You can't specify a [`dimension`](#dimension) block when `disable_default_dimensions` is `true`.
A `dimension` block already replaces the default dimensions.

## Blocks

You can use the following blocks with `otelcol.connector.exceptions`:

{{< docs/alloy-config >}}

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`output`][output]               | Configures where to send telemetry data.                                   | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |
| [`dimension`][dimension]         | Configures additional dimensions for log records and metrics.              | no       |
| [`exemplars`][exemplars]         | Configures how to attach exemplars to metrics.                             | no       |

[output]: #output
[debug_metrics]: #debug_metrics
[dimension]: #dimension
[exemplars]: #exemplars

{{< /docs/alloy-config >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `dimension`

The `dimension` block configures an attribute to add to the log records converted from span events and to the emitted metric data points.

{{< admonition type="note" >}}
Dimensions don't apply when logs are both the input and the output signal.
`otelcol.connector.exceptions` forwards matching log records unchanged, so they keep exactly the attributes they arrived with.
{{< /admonition >}}

Each converted log record and metric data point has the following attributes:

* `service.name`
* `span.name`
* `span.kind`
* `status.code`

The `span.name`, `span.kind`, and `status.code` attributes are only added when the exception comes from a span.

You can specify multiple `dimension` blocks.
When you don't specify any `dimension` block, {{< param "PRODUCT_NAME" >}} adds the `exception.type` and `exception.message` dimensions.
When you specify at least one `dimension` block, {{< param "PRODUCT_NAME" >}} adds only the dimensions you list.
To add no dimensions beyond the attributes listed previously, set [`disable_default_dimensions`](#arguments) to `true` and specify no `dimension` block.

The following arguments are supported:

| Name      | Type     | Description                                               | Default | Required |
|-----------|----------|-----------------------------------------------------------|---------|----------|
| `name`    | `string` | Span, span event, log record, or resource attribute name. |         | yes      |
| `default` | `string` | Value to use if the attribute is missing.                 |         | no       |

When the input signal is traces, `otelcol.connector.exceptions` looks for the `name` attribute in the span attributes, then in the span event attributes, and finally in the resource attributes.
When the input signal is logs, it looks in the log record attributes and then in the resource attributes.

If the attribute is missing:

* If you don't set `default`, `otelcol.connector.exceptions` omits the dimension.
* If you set `default`, `otelcol.connector.exceptions` adds the dimension with that value.

Each `dimension` block must have a unique `name`.
You can't use `service.name`, `span.name`, `span.kind`, or `status.code` as a dimension name because they're already added by default.

Converted log records always carry the `exception.type`, `exception.message`, and `exception.stacktrace` attributes when they're present on the span event, regardless of the configured dimensions.

### `exemplars`

The `exemplars` block configures how to attach exemplars to the `exceptions` metric.

The following arguments are supported:

| Name      | Type   | Description                                                         | Default | Required |
|-----------|--------|---------------------------------------------------------------------|---------|----------|
| `enabled` | `bool` | Attaches the trace ID and span ID of each exception as an exemplar. | `false` | no       |

`otelcol.connector.exceptions` only records an exemplar when the span or log record carries a trace ID.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` traces and logs telemetry data.
It doesn't accept metrics.

## Component health

`otelcol.connector.exceptions` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.connector.exceptions` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.connector.exceptions` doesn't expose any component-specific debug metrics.

## Examples

### Convert span exceptions to logs

The following example accepts traces, converts the exceptions recorded on spans into log records, writes the log records to Loki, and writes the traces to Tempo.

```alloy
otelcol.receiver.otlp "default" {
  grpc {
    endpoint = "0.0.0.0:4317"
  }

  output {
    traces = [
      otelcol.connector.exceptions.default.input,
      otelcol.exporter.otlphttp.tempo.input,
    ]
  }
}

otelcol.connector.exceptions "default" {
  output {
    logs = [otelcol.exporter.otlphttp.loki.input]
  }
}

otelcol.exporter.otlphttp "loki" {
  client {
    endpoint = "http://loki:3100/otlp"
  }
}

otelcol.exporter.otlphttp "tempo" {
  client {
    endpoint = "http://tempo:4318"
  }
}
```

### Filter exception logs and count exceptions

The following example accepts logs, forwards only the log records that describe exceptions to a dedicated backend, and writes a metric counting exceptions to Mimir.
The metric also has an `http.method` label taken from the log record or resource attributes.

```alloy
otelcol.receiver.otlp "default" {
  grpc {
    endpoint = "0.0.0.0:4317"
  }

  output {
    logs = [otelcol.connector.exceptions.default.input]
  }
}

otelcol.connector.exceptions "default" {
  dimension {
    name = "exception.type"
  }

  dimension {
    name    = "http.method"
    default = "unknown"
  }

  output {
    logs    = [otelcol.exporter.otlphttp.exceptions.input]
    metrics = [otelcol.exporter.prometheus.default.input]
  }
}

otelcol.exporter.otlphttp "exceptions" {
  client {
    endpoint = "http://exceptions-backend:4318"
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

The metric in Mimir looks like this:

```text
exceptions_total{service_name="shop-backend",exception_type="NullPointerException",http_method="GET"}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.connector.exceptions` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.connector.exceptions` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
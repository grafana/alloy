---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.connector.count/
aliases:
  - ../otelcol.connector.count/ # /docs/alloy/latest/reference/components/otelcol.connector.count/
description: Learn about otelcol.connector.count
labels:
  stage: experimental
  products:
    - oss
title: otelcol.connector.count
---

# `otelcol.connector.count`

`otelcol.connector.count` counts spans, span events, metrics, data points, log records and profiles from other `otelcol` components and outputs metrics from these counts.

{{< admonition type="note" >}}
`otelcol.connector.count` is a wrapper over the upstream OpenTelemetry Collector [`count`][] connector.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`count`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/connector/countconnector
{{< /admonition >}}

You can specify multiple `otelcol.connector.count` components by giving them different labels.

## Usage

```alloy
otelcol.connector.count "<LABEL>" {
  output {
    metrics = [...]
  }
}
```

## Blocks

You can use the following blocks with `otelcol.connector.count`:

| Block                            | Description                                                                | Required |
|----------------------------------|----------------------------------------------------------------------------|----------|
| [`spans`][spans]                 | Configures counts for spans.                                               | no       |
| [`span_events`][span_events]     | Configures counts for span events.                                         | no       |
| [`metrics`][metrics]             | Configures counts for metrics.                                             | no       |
| [`data_points`][data_points]     | Configures counts for data points.                                         | no       |
| [`logs`][logs]                   | Configures counts for log records.                                         | no       |
| [`profiles`][profiles]           | Configures counts for profiles.                                            | no       |
| [`output`][output]               | Configures where to send telemetry data.                                   | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[spans]: #spans
[span_events]: #span_events
[metrics]: #metrics
[data_points]: #data_points
[logs]: #logs
[profiles]: #profiles
[output]: #output
[debug_metrics]: #debug_metrics

### `spans`

#### Blocks

| Block            | Description                                             | Required |
|------------------|---------------------------------------------------------|----------|
| [`count`][count] | Configures a custom count (can be used multiple times). | yes      |

[count]: #count

### `span_events`

#### Blocks

| Block            | Description                                             | Required |
|------------------|---------------------------------------------------------|----------|
| [`count`][count] | Configures a custom count (can be used multiple times). | yes      |

[count]: #count

### `metrics`

#### Blocks

| Block            | Description                                             | Required |
|------------------|---------------------------------------------------------|----------|
| [`count`][count] | Configures a custom count (can be used multiple times). | yes      |

[count]: #count

### `data_points`

#### Blocks

| Block            | Description                                             | Required |
|------------------|---------------------------------------------------------|----------|
| [`count`][count] | Configures a custom count (can be used multiple times). | yes      |

[count]: #count

### `logs`

#### Blocks

| Block            | Description                                             | Required |
|------------------|---------------------------------------------------------|----------|
| [`count`][count] | Configures a custom count (can be used multiple times). | yes      |

[count]: #count

### `profiles`

#### Blocks

| Block            | Description                                             | Required |
|------------------|---------------------------------------------------------|----------|
| [`count`][count] | Configures a custom count (can be used multiple times). | yes      |

[count]: #count

### `count`

#### Arguments

| Name          | Type                            | Description                                                                 | Default | Required |
|---------------|---------------------------------|-----------------------------------------------------------------------------|---------|----------|
| `name`        | `string`                        | Name of the metric emitted for this count.                                  |         | yes       |
| `description` | `string`                        | Description of the metric emitted for this count.                           | `"The number of spans observed."` or <br> `"The number of span events observed."` or <br> `"The number of metrics observed."` or <br> `"The number of data points observed."` or <br> `"The number of log records observed."` or <br> `"The number of profiles observed."` <br> depending on the counted data. | no       |
| `conditions`  | `list(string)`                  | Data that matches any one of the conditions will be counted.                | `[]`    | no       |
| `attributes`  | [`map(attributes)`][attributes] | A separate count will be generated for each unique set of attribute values. | `{}`    | no       |

[attributes]: #attributes

### `attributes`

#### Arguments

| Name            | Type                 | Description                                                             | Default | Required |
|-----------------|----------------------|-------------------------------------------------------------------------|---------|----------|
| `key`           | `string`             | Key of the attribute which to generate a distinct count for each value. |         | yes      |
| `default_value` | `string` or `number` | Default value to use when the attribute is not present.                 | `""`    | no       |

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block-metrics.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` traces, metrics, logs and profiles telemetry data.

## Component health

`otelcol.connector.count` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.connector.count` doesn't expose any component-specific debug information.

## Example

### Default counts

The example below accepts metrics, logs and traces, and writes them respectively to Mimir, Loki and Tempo.
In addition it creates the default counts on them (everything is counted and the default metrics name and description are used) and writes the metrics to Mimir.

```alloy
otelcol.receiver.otlp "default" {
  grpc {
    endpoint = "0.0.0.0:4317"
  }

  output {
    metrics = [
      otelcol.exporter.otlp.mimir.input,
      otelcol.connector.count.default.input,
    ]
    logs = [
      otelcol.exporter.otlp.loki.input,
      otelcol.connector.count.default.input,
    ]
    traces  = [
      otelcol.exporter.otlp.tempo.input,
      otelcol.connector.count.default.input,
    ]
  }
}

otelcol.connector.count "default" {
  output {
    metrics = [
      otelcol.exporter.otlp.mimir.input,
    ]
  }
}

otelcol.exporter.otlphttp "mimir" {
  client {
    endpoint = sys.env("MIMIR_ENDPOINT) + "/otlp"
    auth     = otelcol.auth.basic.mimir.handler
  }
}

otelcol.auth.basic "mimir" {
  username = sys.env("MIMIR_USERNAME")
  password = sys.env("MIMIR_PASSWORD")
}

otelcol.exporter.otlp "loki" {
  client {
    endpoint = sys.env("LOKI_ENDPOINT") + "/otlp"
    auth     = otelcol.auth.basic.loki.handler
  }
}

otelcol.auth.basic "loki" {
  username = sys.env("LOKI_USERNAME")
  password = sys.env("LOKI_PASSWORD")
}

otelcol.exporter.otlp "tempo" {
  client {
    endpoint = sys.env("TEMPO_ENDPOINT")
    auth     = otelcol.auth.basic.tempo.handler
  }
}

otelcol.auth.basic "tempo" {
  username = sys.env("TEMPO_USERNAME")
  password = sys.env("TEMPO_PASSWORD")
}
```

### Custom counts on logs

The example below accepts logs and writes them to Loki.
In addition it creates custom counts on them and writes the metrics to Mimir.

```alloy
otelcol.receiver.otlp "default" {
  grpc {
    endpoint = "0.0.0.0:4317"
  }

  output {
    logs = [
      otelcol.exporter.otlp.loki.input,
      otelcol.connector.count.default.input,
    ]
  }
}

otelcol.connector.count "default" {
  logs {
    count {
      name = "count.all"
    }
    count {
      name = "count.conditions"
      description = "Logs count of the production environment"
      conditions = [
        'attributes["environment"] == "production"'
      ]
    }
    count {
      name = "count.attributes"
      description = "Logs count by environment and service"
      attributes = [
        {
          key = "environment"
        },
        {
          key = "service"
          default_value = "unknown"
        },
      ]
    }
  }

  output {
    metrics = [
      otelcol.exporter.otlp.mimir.input,
    ]
  }
}

otelcol.exporter.otlphttp "mimir" {
  client {
    endpoint = sys.env("MIMIR_ENDPOINT) + "/otlp"
    auth     = otelcol.auth.basic.mimir.handler
  }
}

otelcol.auth.basic "mimir" {
  username = sys.env("MIMIR_USERNAME")
  password = sys.env("MIMIR_PASSWORD")
}

otelcol.exporter.otlp "loki" {
  client {
    endpoint = sys.env("LOKI_ENDPOINT") + "/otlp"
    auth     = otelcol.auth.basic.loki.handler
  }
}

otelcol.auth.basic "loki" {
  username = sys.env("LOKI_USERNAME")
  password = sys.env("LOKI_PASSWORD")
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.connector.count` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.connector.count` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

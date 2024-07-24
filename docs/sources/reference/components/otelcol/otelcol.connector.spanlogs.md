---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.connector.spanlogs/
aliases:
  - ../otelcol.connector.spanlogs/ # /docs/alloy/latest/reference/components/otelcol.connector.spanlogs/
description: Learn about otelcol.connector.spanlogs
title: otelcol.connector.spanlogs
---

# otelcol.connector.spanlogs

`otelcol.connector.spanlogs` accepts traces telemetry data from other `otelcol`
components and outputs logs telemetry data for each span, root, or process.
This allows you to automatically build a mechanism for trace discovery.

{{< admonition type="note" >}}
`otelcol.connector.spanlogs` is a custom component unrelated to any components from the OpenTelemetry Collector.
It is based on the `automatic_logging` component in the [traces][] subsystem of Grafana Agent Static.

[traces]: https://grafana.com/docs/agent/latest/static/configuration/traces-config
{{< /admonition >}}

You can specify multiple `otelcol.connector.spanlogs` components by giving them different labels.

## Usage

```alloy
otelcol.connector.spanlogs "LABEL" {
  output {
    logs    = [...]
  }
}
```

## Arguments

`otelcol.connector.spanlogs` supports the following arguments:

| Name                 | Type           | Description                                   | Default | Required |
| -------------------- | -------------- | --------------------------------------------- | ------- | -------- |
| `spans`              | `bool`         | Log one line per span.                        | `false` | no       |
| `roots`              | `bool`         | Log one line for every root span of a trace.  | `false` | no       |
| `processes`          | `bool`         | Log one line for every process.               | `false` | no       |
| `span_attributes`    | `list(string)` | Additional span attributes to log.            | `[]`    | no       |
| `process_attributes` | `list(string)` | Additional process attributes to log.         | `[]`    | no       |
| `labels`             | `list(string)` | A list of keys that will be logged as labels. | `[]`    | no       |

The values listed in `labels` should be the values of either span or process attributes.

{{< admonition type="warning" >}}
Setting `spans` to `true` could lead to a high volume of logs.
{{< /admonition >}}

## Blocks

The following blocks are supported inside the definition of
`otelcol.connector.spanlogs`:

| Hierarchy | Block         | Description                                       | Required |
| --------- | ------------- | ------------------------------------------------- | -------- |
| overrides | [overrides][] | Overrides for keys in the log body.               | no       |
| output    | [output][]    | Configures where to send received telemetry data. | yes      |

[output]: #output-block
[overrides]: #overrides-block

### overrides block

The `overrides` block configures overrides for keys that will be logged in the body of the log line.

The following attributes are supported:

| Name                | Type     | Description                                                | Default  | Required |
| ------------------- | -------- | ---------------------------------------------------------- | -------- | -------- |
| `logs_instance_tag` | `string` | Indicates if the log line is for a span, root, or process. | `traces` | no       |
| `service_key`       | `string` | Log key for the service name of the resource.              | `svc`    | no       |
| `span_name_key`     | `string` | Log key for the name of the span.                          | `span`   | no       |
| `status_key`        | `string` | Log key for the status of the span.                        | `status` | no       |
| `duration_key`      | `string` | Log key for the duration of the span.                      | `dur`    | no       |
| `trace_id_key`      | `string` | Log key for the trace ID of the span.                      | `tid`    | no       |

### output block

{{< docs/shared lookup="reference/components/output-block-logs.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.connector.spanlogs` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.connector.spanlogs` does not expose any component-specific debug information.

## Example

The following configuration sends logs derived from spans to Loki.

Additionally, `otelcol.processor.attributes` is configured with a "hint" so that
`otelcol.exporter.loki` promotes the span's "attribute1" attribute to a Loki label.

```alloy
otelcol.receiver.otlp "default" {
  grpc {}

  output {
    traces = [otelcol.connector.spanlogs.default.input]
  }
}

otelcol.connector.spanlogs "default" {
  spans              = true
  roots              = true
  processes          = true
  labels             = ["attribute1", "res_attribute1"]
  span_attributes    = ["attribute1"]
  process_attributes = ["res_attribute1"]

  output {
    logs = [otelcol.processor.attributes.default.input]
  }
}

otelcol.processor.attributes "default" {
  action {
    key = "loki.attribute.labels"
    action = "insert"
    value = "attribute1"
  }

  output {
    logs = [otelcol.exporter.loki.default.input]
  }
}

otelcol.exporter.loki "default" {
  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "loki:3100"
  }
}
```

For an input trace like this...

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
            "key": "res_attribute1",
            "value": { "intValue": "78" }
          },
          {
            "key": "unused_res_attribute1",
            "value": { "stringValue": "str" }
          },
          {
            "key": "res_account_id",
            "value": { "intValue": "2245" }
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
                },
                {
                  "key": "unused_attribute1",
                  "value": { "intValue": "78" }
                },
                {
                  "key": "account_id",
                  "value": { "intValue": "2245" }
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

... the output log coming out of `otelcol.connector.spanlogs` will look like this:

```json
{
  "resourceLogs": [
    {
      "scopeLogs": [
        {
          "log_records": [
            {
              "body": {
                "stringValue": "span=TestSpan dur=0ns attribute1=78 svc=TestSvcName res_attribute1=78 tid=7bba9f33312b3dbb8b2c2c62bb7abe2d"
              },
              "attributes": [
                {
                  "key": "traces",
                  "value": { "stringValue": "span" }
                },
                {
                  "key": "attribute1",
                  "value": { "intValue": "78" }
                },
                {
                  "key": "res_attribute1",
                  "value": { "intValue": "78" }
                }
              ]
            },
            {
              "body": {
                "stringValue": "span=TestSpan dur=0ns attribute1=78 svc=TestSvcName res_attribute1=78 tid=7bba9f33312b3dbb8b2c2c62bb7abe2d"
              },
              "attributes": [
                {
                  "key": "traces",
                  "value": { "stringValue": "root" }
                },
                {
                  "key": "attribute1",
                  "value": { "intValue": "78" }
                },
                {
                  "key": "res_attribute1",
                  "value": { "intValue": "78" }
                }
              ]
            },
            {
              "body": {
                "stringValue": "svc=TestSvcName res_attribute1=78 tid=7bba9f33312b3dbb8b2c2c62bb7abe2d"
              },
              "attributes": [
                {
                  "key": "traces",
                  "value": { "stringValue": "process" }
                },
                {
                  "key": "res_attribute1",
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
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.connector.spanlogs` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.connector.spanlogs` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

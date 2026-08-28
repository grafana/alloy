---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.encoding.jsonlog/
description: Learn about otelcol.encoding.jsonlog
labels:
  stage: experimental
  products:
    - oss
title: otelcol.encoding.jsonlog
---

# `otelcol.encoding.jsonlog`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.encoding.jsonlog` encodes and decodes OpenTelemetry log records as JSON.
It exposes a handler that compatible `otelcol` components can use to marshal and unmarshal logs.

{{< admonition type="note" >}}
`otelcol.encoding.jsonlog` is a wrapper over the upstream OpenTelemetry Collector [`jsonlogencodingextension`][] extension.
Grafana Labs redirects bug reports or feature requests to the upstream repository when necessary.

[`jsonlogencodingextension`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/encoding/jsonlogencodingextension
{{< /admonition >}}

You can specify multiple `otelcol.encoding.jsonlog` components by giving them different labels.

## Usage

```alloy
otelcol.encoding.jsonlog "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `otelcol.encoding.jsonlog`:

| Name         | Type     | Description                                      | Default  | Required |
| ------------ | -------- | ------------------------------------------------ | -------- | -------- |
| `array_mode` | `bool`   | Whether to encode and decode JSON arrays.        | `true`   | no       |
| `mode`       | `string` | How to marshal fields from each log record.      | `"body"` | no       |

The `array_mode` argument controls the JSON document shape.
When `array_mode` is `true`, unmarshaled input must be a JSON array of objects, and marshaled output is a JSON array.
When `array_mode` is `false`, unmarshaled input can be a single JSON object or a stream of concatenated JSON objects, including newline-delimited JSON.
Marshaled output contains one JSON object per line when `array_mode` is `false` and there are multiple log records.

The `mode` argument supports the following values:

- `body` marshals each log record's body and requires the body to be a map.
- `body_with_inline_attributes` marshals each log record as an object containing its `body`, `resourceAttributes`, and `logAttributes` fields when those fields aren't empty.

The `mode` argument affects marshaling only.
During unmarshaling, the component stores each JSON object as a log record body.

## Blocks

The `otelcol.encoding.jsonlog` component does not support any blocks. You can configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                       | Description                                                                      |
| --------- | -------------------------- | -------------------------------------------------------------------------------- |
| `handler` | `capsule(otelcol.Handler)` | A handler that compatible `otelcol` components can use to encode and decode logs. |

## Component health

`otelcol.encoding.jsonlog` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.encoding.jsonlog` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.encoding.jsonlog` doesn't expose any component-specific debug metrics.

## Examples

The following examples configure JSON log encoding with default and non-default behavior.

### Use the default JSON log encoding

This example configures JSON log encoding with the default array format and body-only marshaling:

```alloy
otelcol.encoding.jsonlog "default" {
}
```

### Encode newline-delimited JSON with attributes

This example configures the component to marshal newline-delimited JSON objects that include log and resource attributes.
It also accepts a single JSON object or concatenated JSON objects when unmarshaling:

```alloy
otelcol.encoding.jsonlog "ndjson" {
  array_mode = false
  mode       = "body_with_inline_attributes"
}
```

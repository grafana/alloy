---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.encoding.text/
description: Learn about otelcol.encoding.text
labels:
  stage: experimental
  products:
    - oss
title: otelcol.encoding.text
---

# `otelcol.encoding.text`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.encoding.text` encodes and decodes OpenTelemetry log records as text.
It exposes a handler that compatible `otelcol` components can use to marshal and unmarshal logs.

{{< admonition type="note" >}}
`otelcol.encoding.text` is a wrapper over the upstream OpenTelemetry Collector [`textencodingextension`][] extension.
Grafana Labs redirects bug reports or feature requests to the upstream repository when necessary.

[`textencodingextension`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/encoding/textencodingextension
{{< /admonition >}}

You can specify multiple `otelcol.encoding.text` components by giving them different labels.

## Usage

```alloy
otelcol.encoding.text "<LABEL>" {
}
```

## Arguments

You can use the following arguments with `otelcol.encoding.text`:

| Name                     | Type     | Description                                                | Default    | Required |
| ------------------------ | -------- | ---------------------------------------------------------- | ---------- | -------- |
| `encoding`               | `string` | Character encoding to use when unmarshaling logs.          | `"utf8"`   | no       |
| `marshaling_separator`   | `string` | Separator to insert between marshaled log record bodies.   | `"\n"`     | no       |
| `unmarshaling_separator` | `string` | Regular expression that separates unmarshaled log records. | `"\r?\n"` | no       |

The `encoding` argument selects the character encoding used to decode input into log record bodies.
The component reports an invalid configuration if the upstream extension doesn't recognize the encoding.

The `marshaling_separator` argument separates consecutive log record bodies in marshaled output.

The `unmarshaling_separator` argument is a regular expression that splits input into log records.
Set `unmarshaling_separator` to an empty string to decode all input as one log record.

## Blocks

The `otelcol.encoding.text` component does not support any blocks. You can configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                       | Description                                                                      |
| --------- | -------------------------- | -------------------------------------------------------------------------------- |
| `handler` | `capsule(otelcol.Handler)` | A handler that compatible `otelcol` components can use to encode and decode logs. |

## Component health

`otelcol.encoding.text` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.encoding.text` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.encoding.text` doesn't expose any component-specific debug metrics.

## Examples

The following examples configure text encoding with default and non-default behavior.

### Use the default text encoding

This example configures UTF-8 text encoding with the default line separators:

```alloy
otelcol.encoding.text "default" {
}
```

### Decode Windows-1252 records separated by blank lines

This example decodes Windows-1252 input and splits records at one or more blank lines.
It uses two newline characters between bodies when it marshals multiple log records:

```alloy
otelcol.encoding.text "windows" {
  encoding               = "windows-1252"
  marshaling_separator   = "\n\n"
  unmarshaling_separator = "(\r?\n){2,}"
}
```

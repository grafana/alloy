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
If `unmarshaling_separator` is empty, the decoder waits until it reaches the end of the input and decodes as a single log record.

## Blocks

The `otelcol.encoding.text` component doesn't support any blocks.
You can configure this component with arguments.

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

### `otelcol.receiver.awss3`

This example uses `otelcol.encoding.text` to decode UTF-8 log records from S3 objects with keys that end in `.txt`.
The receiver splits records at one or more blank lines and forwards them to `otelcol.exporter.debug`:

```alloy
otelcol.encoding.text "default" {
	encoding               = "utf8"
	unmarshaling_separator = "(\r?\n){2,}"
}

otelcol.receiver.awss3 "default" {
	start_time = "2024-01-01 01:00"
	end_time   = "2024-01-02"

	s3downloader {
		region    = "us-west-1"
		s3_bucket = "mybucket"
		s3_prefix = "logs"
	}

	encoding {
		extension = otelcol.encoding.text.default.handler
		suffix    = ".txt"
	}

	output {
		logs = [otelcol.exporter.debug.default.input]
	}
}

otelcol.exporter.debug "default" {}
```

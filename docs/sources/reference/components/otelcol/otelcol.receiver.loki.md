---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.loki/
aliases:
  - ../otelcol.receiver.loki/ # /docs/alloy/latest/reference/otelcol.receiver.loki/
description: Learn about otelcol.receiver.loki
labels:
  stage: general-availability
  products:
    - oss
title: otelcol.receiver.loki
---

# `otelcol.receiver.loki`

`otelcol.receiver.loki` receives Loki log entries, converts them to the OpenTelemetry logs format, and forwards them to other `otelcol.*` components.

You can specify multiple `otelcol.receiver.loki` components by giving them different labels.

{{< admonition type="note" >}}
`otelcol.receiver.loki` is a custom component unrelated to any receivers from the upstream OpenTelemetry Collector.
{{< /admonition >}}

## Usage

```alloy
otelcol.receiver.loki "<LABEL>" {
  output {
    logs = [...]
  }
}
```

## Arguments

The `otelcol.receiver.loki` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `otelcol.receiver.loki`:

{{< docs/alloy-config >}}

| Block              | Description                                        | Required |
|--------------------|----------------------------------------------------|----------|
| [`labels`][labels] | Configures selective label forwarding.             | no       |
| [`output`][output] | Configures where to send converted telemetry data. | yes      |

[labels]: #labels
[output]: #output

{{< /docs/alloy-config >}}

### `labels`

The `labels` block configures which Loki labels are forwarded as OpenTelemetry log record attributes and allows renaming them during conversion.
When the `labels` block isn't provided, all Loki labels are forwarded as attributes, preserving backward compatibility.

You can use the following arguments with `labels`:

| Name      | Type              | Description                                                        | Default | Required |
|-----------|-------------------|--------------------------------------------------------------------|---------|----------|
| `include` | `list(string)`    | Allowlist of label names to forward. All others are dropped.       |         | no       |
| `exclude` | `list(string)`    | Blocklist of label names to drop. All others are forwarded.        |         | no       |
| `rename`  | `map(string)`     | Map of original label names to new attribute names.                |         | no       |

The `include` and `exclude` attributes are mutually exclusive.
Setting both results in a validation error.

The `rename` attribute is applied after `include` or `exclude` filtering.
It maps the original Loki label name to the desired OpenTelemetry attribute name.
Renamed keys are also reflected in the `loki.attribute.labels` hint attribute.

{{< admonition type="note" >}}
The `filename` label receives special handling: when present, the `log.file.path` and `log.file.name` attributes are always added to the log record regardless of filtering.
However, the `filename` label itself is subject to the `include` and `exclude` filters.
{{< /admonition >}}

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type           | Description                                                 |
|------------|----------------|-------------------------------------------------------------|
| `receiver` | `LogsReceiver` | A value that other components can use to send Loki logs to. |

## Component health

`otelcol.receiver.loki` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.loki` doesn't expose any component-specific debug information.

## Examples

### Basic usage

This example uses the `otelcol.receiver.loki` component as a bridge between the Loki and OpenTelemetry ecosystems.
The component exposes a receiver which the `loki.source.file` component uses to send Loki log entries to.
The logs are converted to the OTLP format before they're forwarded to the `otelcol.exporter.otlphttp` component to be sent to an OTLP-capable endpoint:

```alloy
loki.source.file "default" {
  targets = [
    {__path__ = "/tmp/foo.txt", "loki.format" = "logfmt"},
    {__path__ = "/tmp/bar.txt", "loki.format" = "json"},
  ]
  forward_to = [otelcol.receiver.loki.default.receiver]
}

otelcol.receiver.loki "default" {
  output {
    logs = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

### Selective label forwarding with renaming

This example only forwards the `job` label and renames it to `service.name`, dropping all other labels:

```alloy
loki.source.file "default" {
  targets = [
    {__path__ = "/var/log/*.log"},
  ]
  forward_to = [otelcol.receiver.loki.default.receiver]
}

otelcol.receiver.loki "default" {
  labels {
    include = ["job"]
    rename = {
      job = "service.name",
    }
  }

  output {
    logs = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

### Excluding specific labels

This example drops high-cardinality labels like `instance` and `stream`, forwarding all others:

```alloy
otelcol.receiver.loki "default" {
  labels {
    exclude = ["instance", "stream"]
  }

  output {
    logs = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = sys.env("<OTLP_ENDPOINT>")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.loki` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.receiver.loki` has exports that can be consumed by the following components:

- Components that consume [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

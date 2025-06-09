---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol.exporter.debug/
description: Learn about otelcol.exporter.debug
labels:
  stage: experimental
  products:
    - oss
title: otelcol.exporter.debug
---

# `otelcol.exporter.debug`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.debug` accepts telemetry data from other `otelcol` components and writes them to the console (stderr).
You can control the verbosity of the logs.

{{< admonition type="note" >}}
`otelcol.exporter.debug` is a wrapper over the upstream OpenTelemetry Collector [`debug`][] exporter.
If necessary, bug reports or feature requests are redirected to the upstream repository.

[`debug`]: https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/exporter/debugexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.debug` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.debug "<LABEL>" { }
```

## Arguments

You can use the following arguments with `otelcol.exporter.debug`:

| Name                  | Type     | Description                                                       | Default   | Required |
| --------------------- | -------- | ----------------------------------------------------------------- | --------- | -------- |
| `sampling_initial`    | `int`    | Number of messages initially logged each second.                  | `2`       | no       |
| `sampling_thereafter` | `int`    | Sampling rate after the initial messages are logged.              | `1`       | no       |
| `use_internal_logger` | `bool`   | Whether to use the internal logger or print directly to `stderr`. | `true`    | no       |
| `verbosity`           | `string` | Verbosity of the generated logs.                                  | `"basic"` | no       |

The `verbosity` argument must be one of:

* `"basic"`: A single-line summary of received data is logged to stderr, with a total count of telemetry records for every batch of received logs, metrics, or traces.
* `"normal"`: Produces the same output as `"basic"` verbosity.
* `"detailed"`: All details of every telemetry record are logged to stderr, typically writing multiple lines for every telemetry record.

The following example shows `"basic"` and `"normal"` output:

```text
ts=2024-06-13T11:24:13.782957Z level=info msg=TracesExporter component_path=/ component_id=otelcol.exporter.debug.default "resource spans": 1, spans: 2
```

The following example shows `"detailed"` output:

```text
ts=2024-06-13T11:24:13.782957Z level=info msg=TracesExporter component_path=/ component_id=otelcol.exporter.debug.default "resource spans"=1 spans=2
ts=2024-06-13T11:24:13.783101Z level=info msg="ResourceSpans #0
Resource SchemaURL: https://opentelemetry.io/schemas/1.4.0
Resource attributes:
     -> service.name: Str(telemetrygen)
ScopeSpans #0
ScopeSpans SchemaURL:
InstrumentationScope telemetrygen
Span #0
    Trace ID       : 3bde5d3ee82303571bba6e1136781fe4
    Parent ID      : 5e9dcf9bac4acc1f
    ID             : 2cf3ef2899aba35c
    Name           : okey-dokey
    Kind           : Server
    Start time     : 2023-11-11 04:49:03.509369393 +0000 UTC
    End time       : 2023-11-11 04:49:03.50949377 +0000 UTC
    Status code    : Unset
    Status message :
Attributes:
     -> net.peer.ip: Str(1.2.3.4)
     -> peer.service: Str(telemetrygen-client)
Span #1
    Trace ID       : 3bde5d3ee82303571bba6e1136781fe4
    Parent ID      :
    ID             : 5e9dcf9bac4acc1f
    Name           : lets-go
    Kind           : Client
    Start time     : 2023-11-11 04:49:03.50935117 +0000 UTC
    End time       : 2023-11-11 04:49:03.50949377 +0000 UTC
    Status code    : Unset
    Status message :
Attributes:
     -> net.peer.ip: Str(1.2.3.4)
     -> peer.service: Str(telemetrygen-server)
        {"kind": "exporter", "data_type": "traces", "name": "debug"}"
```

{{< admonition type="note" >}}
All instances of `\n` in the `"detailed"` example have been replaced with new lines.
{{< /admonition >}}

Setting `use_internal_logger` to `false` is useful if you would like to see actual new lines instead of `\n` in the collector logs.
However, by not using the internal logger you wouldn't see metadata in the log line such as `component_id=otelcol.exporter.debug.default`.
Multiline logs may also be harder to parse.

## Blocks

You can use the following block with `otelcol.exporter.debug`:

| Block                            | Description                                                                | Required |
| -------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[debug_metrics]: #debug_metrics

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.debug` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.debug` doesn't expose any component-specific debug information.

## Example

This example receives OTLP metrics, logs, and traces and writes them to the console:

```alloy
otelcol.receiver.otlp "default" {
    grpc {}
    http {}

    output {
        metrics = [otelcol.exporter.debug.default.input]
        logs    = [otelcol.exporter.debug.default.input]
        traces  = [otelcol.exporter.debug.default.input]
    }
}

otelcol.exporter.debug "default" {}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.debug` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
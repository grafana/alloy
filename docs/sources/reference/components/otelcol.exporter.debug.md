---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol.exporter.debug/
description: Learn about otelcol.exporter.debug
title: otelcol.exporter.debug
---

<span class="badge docs-labels__stage docs-labels__item">Experimental</span>

# otelcol.exporter.debug

`otelcol.exporter.debug` accepts telemetry data from other `otelcol` components and writes them to the console (stderr).
You can control the verbosity of the logs.

{{< admonition type="note" >}}
`otelcol.exporter.debug` is a wrapper over the upstream OpenTelemetry Collector `debug` exporter. 
If necessary, bug reports or feature requests are redirected to the upstream repository.
{{< /admonition >}}

Multiple `otelcol.exporter.debug` components can be specified by giving them different labels.

## Usage

```river
otelcol.exporter.debug "LABEL" { }
```

## Arguments

`otelcol.exporter.debug` supports the following arguments:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`verbosity`           | `string` | Verbosity of the generated logs. | `"normal"` | no
`sampling_initial`    | `int`    | Number of messages initially logged each second. | `2` | no
`sampling_thereafter` | `int`    | Sampling rate after the initial messages are logged. | `500` | no

The `verbosity` argument must be one of:
* `"basic"`: A single-line summary of received data is logged to stderr, with a total count of telemetry records for every batch of received logs, metrics or traces.
* `"normal"`: Currently, this produces the same output as `"basic"` verbosity.
* `"detailed"`: All details of every telemetry record are logged to stderr, typically writing multiple lines for every telemetry record.

Example of `"basic"` and `"normal"` output:
```
ts=2024-06-13T11:24:13.782957Z level=info msg=TracesExporter component_path=/ component_id=otelcol.exporter.debug.default "resource spans": 1, spans: 2
```

Example of `"detailed"` output:
```
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

Note that the above `"detailed"` example was edited to replace all instances of `\n` with a new line.

## Blocks

The following blocks are supported inside the definition of
`otelcol.exporter.debug`:

Hierarchy     | Block             | Description                                                                | Required
--------------|-------------------|----------------------------------------------------------------------------|---------
debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no

The `>` symbol indicates deeper levels of nesting. For example, `client > tls`
refers to a `tls` block defined inside a `client` block.

[debug_metrics]: #debug_metrics-block

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics,
logs, or traces).

## Component health

`otelcol.exporter.debug` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.exporter.debug` does not expose any component-specific debug
information.

## Example

This example scrapes Prometheus UNIX metrics and writes them to the console:

```river
prometheus.exporter.unix "default" { }

prometheus.scrape "default" {
    targets    = prometheus.exporter.unix.default.targets
    forward_to = [otelcol.receiver.prometheus.default.receiver]
}

otelcol.receiver.prometheus "default" {
    output {
        metrics = [otelcol.exporter.debug.default.input]
    }
}

otelcol.exporter.debug "default" {
    verbosity           = "detailed"
    sampling_initial    = 1
    sampling_thereafter = 1
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.debug` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
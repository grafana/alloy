---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.logging/
aliases:
  - ../otelcol.exporter.logging/ # /docs/alloy/latest/reference/components/otelcol.exporter.logging/
description: Learn about otelcol.exporter.logging
title: otelcol.exporter.logging
---

# otelcol.exporter.logging

{{< admonition type="warning" >}}
The OpenTelemetry Collector `logging` exporter is deprecated.
It will be removed from the upstream Collector repository in September 2024.
`otelcol.exporter.logging` may also be removed in a future Alloy version.
Use `otelcol.exporter.debug` instead.
{{< /admonition >}}

`otelcol.exporter.logging` accepts telemetry data from other `otelcol` components
and writes them to the console.

This component writes logs at the info level. The [logging config block][] must be
configured to write logs at the info level.

{{< admonition type="note" >}}
`otelcol.exporter.logging` is a wrapper over the upstream OpenTelemetry Collector `logging` exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

Multiple `otelcol.exporter.logging` components can be specified by giving them
different labels.

[logging config block]: ../../../config-blocks/logging/

## Usage

```alloy
otelcol.exporter.logging "LABEL" { }
```

## Arguments

`otelcol.exporter.logging` supports the following arguments:

Name                  | Type     | Description                                          | Default    | Required
----------------------|----------|------------------------------------------------------|------------|---------
`verbosity`           | `string` | Verbosity of the generated logs.                     | `"normal"` | no
`sampling_initial`    | `int`    | Number of messages initially logged each second.     | `2`        | no
`sampling_thereafter` | `int`    | Sampling rate after the initial messages are logged. | `500`      | no

The `verbosity` argument must be one of `"basic"`, `"normal"`, or `"detailed"`.

## Blocks

The following blocks are supported inside the definition of
`otelcol.exporter.logging`:

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

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics,
logs, or traces).

## Component health

`otelcol.exporter.logging` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.exporter.logging` does not expose any component-specific debug
information.

## Example

This example scrapes prometheus unix metrics and writes them to the console:

```alloy
prometheus.exporter.unix "default" { }

prometheus.scrape "default" {
    targets    = prometheus.exporter.unix.default.targets
    forward_to = [otelcol.receiver.prometheus.default.receiver]
}

otelcol.receiver.prometheus "default" {
    output {
        metrics = [otelcol.exporter.logging.default.input]
    }
}

otelcol.exporter.logging "default" {
    verbosity           = "detailed"
    sampling_initial    = 1
    sampling_thereafter = 1
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.logging` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
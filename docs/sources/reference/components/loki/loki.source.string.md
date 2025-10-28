---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.string/
aliases:
  - ../loki.source.string/ # /docs/alloy/latest/reference/components/loki.source.string/
description: Learn about loki.string
labels:
  stage: general-availability
  products:
    - oss
title: loki.source.string
---

# `loki.source.string`

`loki.source.string` receives log entries as string from other components and can be ingested to any loki component.

You can specify multiple `loki.source.string` components by giving them different labels.

## Usage

```alloy
loki.source.string "<LABEL>" {
    source      = <TARGET>
    forward_to  = <RECEIVER>
}
```

## Arguments

The components consumes the value from the source and converts them to log entries passing them to other components receivers in `forward_to` 

| Name                    | Type                 | Description                                                | Default | Required |
| ----------------------- | -------------------- | ---------------------------------------------------------- | ------- | -------- |
| `source`                | `string`             | A value pointing to a string source.                       |         | yes      |
| `forward_to`            | `LogsReceiver`       | Receiver to send log entries to.                           |         | yes      |

## Blocks

The `loki.source.string` component doesn't support any blocks.

## Exported fields

`loki.source.string` doesn't export any fields.

## Component health

`loki.source.string` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.source.string` doesn't expose any component-specific debug information.

## Example

This example creates a pipeline that reads string response from remote `/data` endpoint passes it to `loki.source.string` which converts it to [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters) and then echo the value to stdout:

```alloy
remote.http "server" {
  url = "http://localhost:2112/data"
}
loki.source.string "stringer" {
    source      = remote.http.server.content
    forward_to  = loki.echo.print.receiver
}
loki.echo "print" { }
```
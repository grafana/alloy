---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.string/
aliases:
  - ../loki.source.string/ # /docs/alloy/latest/reference/components/loki.source.string/
description: Learn about loki.source.string
labels:
  stage: general-availability
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: loki.source.string
---

# `loki.source.string`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`loki.source.string` receives strings from other components such as `local.file` or `remote.http`. 
It then converts them to logs which can be forwarded to `loki.*` components such as `loki.process` and `loki.write`.

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
| `forward_to`            | `LogsReceiver`       | Receiver to send log entries to.                           |         | yes      |
| `source`                | `string`             | A value pointing to a string source.                       |         | yes      |

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

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.string` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.echo/
aliases:
  - ../loki.echo/ # /docs/alloy/latest/reference/components/loki.echo/
description: Learn about loki.echo
labels:
  stage: general-availability
  products:
    - oss
title: loki.echo
---

# `loki.echo`

`loki.echo` receives log entries from other `loki` components and prints them to the process' standard output, `stdout`.

You can specify multiple `loki.echo` components by giving them different labels.

## Usage

```alloy
loki.echo "<LABEL>" {}
```

## Arguments

The `loki.echo` component doesn't support any arguments.

## Blocks

The `loki.echo` component doesn't support any blocks.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type           | Description                                                   |
| ---------- | -------------- | ------------------------------------------------------------- |
| `receiver` | `LogsReceiver` | A value that other components can use to send log entries to. |

## Component health

`loki.echo` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.echo` doesn't expose any component-specific debug information.

## Example

This example creates a pipeline that reads log files from `/var/log` and prints log lines to echo:

```alloy
local.file_match "varlog" {
  path_targets = [{
    __path__ = "/var/log/*log",
    job      = "varlog",
  }]
}

loki.source.file "logs" {
  targets    = local.file_match.varlog.targets
  forward_to = [loki.echo.example.receiver]
}

loki.echo "example" { }
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.echo` has exports that can be consumed by the following components:

- Components that consume [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

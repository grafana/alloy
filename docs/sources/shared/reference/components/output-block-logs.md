---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/output-block-logs/
description: Shared content, output block logs
headless: true
---

The `output` block configures a set of components to forward resulting telemetry data to.

The following arguments are supported:

| Name   | Type                     | Description                        | Default | Required |
| ------ | ------------------------ | ---------------------------------- | ------- | -------- |
| `logs` | `list(otelcol.Consumer)` | List of consumers to send logs to. | `[]`    | no       |

You must specify the `output` block, but all its arguments are optional.
By default, telemetry data is dropped.
Configure the `logs` argument accordingly to send telemetry data to other components, .

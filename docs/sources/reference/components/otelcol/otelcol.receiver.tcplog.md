---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.tcplog/
description: Learn about otelcol.receiver.tcplog
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.tcplog
---

# `otelcol.receiver.tcplog`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.tcplog` accepts log messages over a TCP connection and forwards them as logs to other `otelcol.*` components.

{{< admonition type="note" >}}
`otelcol.receiver.tcplog` is a wrapper over the upstream OpenTelemetry Collector [`tcplog`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`tcplog`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/tcplogreceiver
{{< /admonition >}}

You can specify multiple `otelcol.receiver.tcplog` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.tcplog "<LABEL>" {
  listen_address = "<IP_ADDRESS:PORT>"

  output {
    logs    = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.tcplog`:

| Name                 | Type     | Description                                                                                                 | Default   | Required |
|----------------------|----------|-------------------------------------------------------------------------------------------------------------|-----------|----------|
| `listen_address`     | `string` | The `<HOST:PORT>` address to listen to for logs messages.                                                   |           | yes      |
| `add_attributes`     | `bool`   | Add `net.*` attributes to log messages according to [OpenTelemetry semantic conventions][net-semconv].      | `false`   | no       |
| `encoding`           | `string` | The encoding of the log messages.                                                                           | `"utf-8"` | no       |
| `max_log_size`       | `string` | The maximum size of a log entry to read before failing.                                                     | `"1MiB"`  | no       |
| `one_log_per_packet` | `bool`   | Skip log tokenization, improving performance when messages always contain one log and multiline isn't used. | `false`   | no       |

The `encoding` argument specifies the encoding of the incoming log messages.
`encoding` must be one of `utf-8`, `utf-16le`, `utf-16be`, `ascii`, `big5`, or `nop`.
Refer to the upstream receiver [documentation][encoding-documentation] for more details.

The `max_log_size` argument has a minimum value of `64KiB`.

## Blocks

You can use the following blocks with `otelcol.receiver.tcplog`:

| Block                                  | Description                                                                                     | Required |
| -------------------------------------- | ----------------------------------------------------------------------------------------------- | -------- |
| [`output`][output]                     | Configures where to send received telemetry data.                                               | yes      |
| [`debug_metrics`][debug_metrics]       | Configures the metrics that this component generates to monitor its state.                      | no       |
| [`multiline`][multiline]               | Configures rules for multiline parsing of incoming messages                                     | no       |
| [`retry_on_failure`][retry_on_failure] | Configures the retry behavior when the receiver encounters an error downstream in the pipeline. | no       |
| [`tls`][tls]                           | Configures TLS for the TCP server.                                                              | no       |
| `tls` > [`tpm`][tpm]                   | Configures TPM settings for the TLS `key_file`.                                                 | no       |

The > symbol indicates deeper levels of nesting.
For example, `tls` > `tpm` refers to a `tpm` block defined inside a `tls` block.

[tls]: #tls
[tpm]: #tpm
[multiline]: #multiline
[retry_on_failure]: #retry_on_failure
[debug_metrics]: #debug_metrics
[output]: #output

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block-logs.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `multiline`

The `multiline` block configures logic for splitting incoming log entries.
The following arguments are supported:

| Name                 | Type     | Description                                                     | Default | Required |
|----------------------|----------|-----------------------------------------------------------------|---------|----------|
| `line_end_pattern`   | `string` | A regular expression that matches the end of a log entry.       |         | no       |
| `line_start_pattern` | `string` | A regular expression that matches the beginning of a log entry. |         | no       |
| `omit_pattern`       | `bool`   | Omit the start/end pattern from the split log entries.          | `false` | no       |

A `multiline` block must contain either `line_start_pattern` or `line_end_pattern`.

If a `multiline` block isn't set, log entries won't be split.

### `retry_on_failure`

The `retry_on_failure` block configures the retry behavior when the receiver encounters an error downstream in the pipeline.
A backoff algorithm is used to delay the retry upon subsequent failures.
The following arguments are supported:

| Name               | Type       | Description                                                                                                               | Default | Required |
|--------------------|------------|---------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `enabled`          | `bool`     | If set to `true` and an error occurs, the receiver will pause reading the log files and resend the current batch of logs. | `false` | no       |
| `initial_interval` | `duration` | The time to wait after first failure to retry.                                                                            | `"1s"`  | no       |
| `max_elapsed_time` | `duration` | The maximum age of a message before the data is discarded.                                                                | `"5m"`  | no       |
| `max_interval`     | `duration` | The maximum time to wait after applying backoff logic.                                                                    | `"30s"` | no       |

If `max_elapsed_time` is set to `0` data is never discarded.

### `tls`

The `tls` block configures TLS settings used for a server.
If the `tls` block isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.tcplog` doesn't export any fields.

## Component health

`otelcol.receiver.tcplog` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.tcplog` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.receiver.tcplog` doesn't expose any component-specific debug metrics.

## Example

This example receives log messages from TCP and logs them.

```alloy
otelcol.receiver.tcplog "default" {
    listen_address = "localhost:1515"
    output {
        logs = [otelcol.exporter.debug.default.input]
    }
}

otelcol.exporter.debug "default" {}
```

[encoding-documentation]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/tcplogreceiver/README.md#supported-encodings
[net-semconv]: https://github.com/open-telemetry/semantic-conventions/blob/main/docs/attributes-registry/network.md#network-attributes

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.tcplog` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

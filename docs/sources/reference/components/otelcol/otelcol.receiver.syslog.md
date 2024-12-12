---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.syslog/
description: Learn about otelcol.receiver.syslog
title: otelcol.receiver.syslog
---

# otelcol.receiver.syslog

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.syslog` accepts syslog messages over the network and forwards them as logs to other `otelcol.*` components.
It supports syslog protocols [RFC5424][] and [RFC3164][] and can receive data over `TCP` or `UDP`.

{{< admonition type="note" >}}
`otelcol.receiver.syslog` is a wrapper over the upstream OpenTelemetry Collector `syslog` receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

You can specify multiple `otelcol.receiver.syslog` components by giving them different labels.

[RFC5424]: https://www.rfc-editor.org/rfc/rfc5424
[RFC3164]: https://www.rfc-editor.org/rfc/rfc3164

## Usage

```alloy
otelcol.receiver.syslog "LABEL" {
  tcp { ... }
  udp { ... }

  output {
    logs    = [...]
  }
}
```

## Arguments

The following arguments are supported:

| Name                              | Type     | Description                                                        | Default   | Required |
|-----------------------------------|----------|--------------------------------------------------------------------|-----------|----------|
| `protocol`                        | `string` | The syslog protocol that the syslog server supports.               | `rfc5424` | no       |
| `location`                        | `string` | The geographic time zone to use when parsing an RFC3164 timestamp. | `UTC`     | no       |
| `enable_octet_counting`           | `bool`   | Whether to enable RFC6587 octet counting.                          | `false`   | no       |
| `max_octets`                      | `int`    | The maximum octets for messages when octet counting is enabled.    | `8192`    | no       |
| `allow_skip_pri_header`           | `bool`   | Allow parsing records without a priority header.                   | `false`   | no       |
| `non_transparent_framing_trailer` | `string` | The framing trailer when using RFC6587 Non-Transparent-Framing.    | `nil`     | no       |

The `protocol` argument specifies the syslog format supported by the receiver.
`protocol` must be one of `rfc5424`, `rfc3164`

The `location` argument specifies a Time Zone identifier. The available locations depend on the local IANA Time Zone database.
See [this wikipedia entry][tz-wiki] for a non-comprehensive list.

The `non_transparent_framing_trailer` argument must be one of `LF`, `NUL`.

The `non_transparent_framing_trailer` and `enable_octet_counting` arguments can't be used with a UDP syslog server.

[tz-wiki]: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones

## Blocks

The following blocks are supported inside the definition of
`otelcol.receiver.syslog`:


| Hierarchy        | Block                | Description                                                                                     | Required |
|------------------|----------------------|-------------------------------------------------------------------------------------------------|----------|
| udp              | [udp][]              | Configures a UDP syslog server to receive syslog messages.                                      | no*       |
| udp > multiline  | [multiline][]        | Configures rules for multiline parsing of incoming messages.                                    | no       |
| udp > async      | [async][]            | Configures rules for asynchronous parsing of incoming messages.                                 | no       |
| tcp              | [tcp][]              | Configures a TCP syslog server to receive syslog messages.                                      | no*       |
| tcp > multiline  | [multiline][]        | Configures rules for multiline parsing of incoming messages                                     | no       |
| tcp > tls        | [tls][]              | Configures TLS for the TCP syslog server.                                                       | no       |
| retry_on_failure | [retry_on_failure][] | Configures the retry behavior when the receiver encounters an error downstream in the pipeline. | no       |
| debug_metrics    | [debug_metrics][]    | Configures the metrics that this component generates to monitor its state.                      | no       |
| output           | [output][]           | Configures where to send received telemetry data.                                               | yes      |

A syslog receiver must have either a `udp` or `tcp` block configured.

The `>` symbol indicates deeper levels of nesting. For example, `tcp > tls`
refers to a `tls` block defined inside a `tcp` block.

[tls]: #tls-block
[udp]: #udp-block
[tcp]: #tcp-block
[multiline]: #multiline-block
[async]: #async-block
[retry_on_failure]: #retry-on-failure-block
[debug_metrics]: #debug_metrics-block
[output]: #output-block

### udp block

The `udp` block configures a UDP syslog server.
The following arguments are supported:

| Name                            | Type     | Description                                                                                                  | Default | Required |
|---------------------------------|----------|--------------------------------------------------------------------------------------------------------------|---------|----------|
| `listen_address`                | `string` | The `<host:port>` address to listen to for syslog messages.                                                  |         | yes      |
| `one_log_per_packet`            | `bool`   | Skip log tokenization, improving performance when messages always contain one log and multiline is not used. | `false` | no       |
| `add_attributes`                | `bool`   | Add net.* attributes to log messages according to OpenTelemetry semantic conventions.                        | `false` | no       |
| `encoding`                      | `string` | The encoding of the syslog messages.                                                                         | `utf-8` | no       |
| `preserve_leading_whitespaces`  | `bool`   | Preserves leading whitespace in messages when set to `true`.                                                 | `false` | no       |
| `preserve_trailing_whitespaces` | `bool`   | Preserves trailing whitespace in messages when set to `true`.                                                 | `false` | no       |

The `encoding` argument specifies the encoding of the incoming syslog messages.
`encoding` must be one of `utf-8`, `utf-16le`, `utf-16be`, `ascii`, `big5`, `nop`. 
Refer to the upstream receiver [documentation][encoding-documentation] for more details.

### multiline block

The `multiline` block configures logic for splitting incoming log entries.
The following arguments are supported:

| Name                 | Type     | Description                                                     | Default | Required |
|----------------------|----------|-----------------------------------------------------------------|---------|----------|
| `line_start_pattern` | `string` | A regular expression that matches the beginning of a log entry. |         | no       |
| `line_end_pattern`   | `string` | A regular expression that matches the end of a log entry.       |         | no       |
| `omit_pattern`       | `bool`   | Omit the start/end pattern from the split log entries.          | `false` | no       |

A `multiline` block must contain either `line_start_pattern` or `line_end_pattern`.

If a `multiline` block is not set, log entries will not be split.

### async block

The `async` block configures concurrent asynchronous readers for a UDP syslog server.
The following arguments are supported:

| Name               | Type  | Description                                                                      | Default | Required |
|--------------------|-------|----------------------------------------------------------------------------------|---------|----------|
| `readers`          | `int` | The number of goroutines to concurrently read from the UDP syslog server.        | `1`     | no       |
| `processors`       | `int` | The number of goroutines to concurrently process logs before sending downstream. | `1`     | no       |
| `max_queue_length` | `int` | The maximum number of messages to wait for an available processor.               | `100`   | no       |

If `async` is not set, a single goroutine will read and process messages synchronously.

### tcp block

The `tcp` block configures a TCP syslog server.
The following arguments are supported:

| Name                            | Type     | Description                                                                                                  | Default | Required |
|---------------------------------|----------|--------------------------------------------------------------------------------------------------------------|---------|----------|
| `listen_address`                | `string` | The `<host:port>` address to listen to for syslog messages.                                                  |         | yes      |
| `max_log_size`                  | `string` | The maximum size of a log entry to read before failing.                                                      | `1MiB`  | no       |
| `one_log_per_packet`            | `bool`   | Skip log tokenization, improving performance when messages always contain one log and multiline is not used. | `false` | no       |
| `add_attributes`                | `bool`   | Add net.* attributes to log messages according to OpenTelemetry semantic conventions.                        | `false` | no       |
| `encoding`                      | `string` | The encoding of the syslog messages.                                                                         | `utf-8` | no       |
| `preserve_leading_whitespaces`  | `bool`   | Preserves leading whitespace in messages when set to `true`.                                                  | `false` | no       |
| `preserve_trailing_whitespaces` | `bool`   | Preserves trailing whitespace in messages when set to `true`.                                                 | `false` | no       |

The `encoding` argument specifies the encoding of the incoming syslog messages.
`encoding` must be one of `utf-8`, `utf-16le`, `utf-16be`, `ascii`, `big5`, `nop`. 
See the upstream receiver [documentation][encoding-documentation] for more details.

The `max_log_size` argument has a minimum value of `64KiB`

### tls block

The `tls` block configures TLS settings used for a server. If the `tls` block
isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### retry on failure block

The `retry_on_failure` block configures the retry behavior when the receiver encounters an error downstream in the pipeline.
A backoff algorithm is used to delay the retry upon subsequent failures.
The following arguments are supported:

| Name               | Type       | Description                                                                                               | Default      | Required |
|--------------------|------------|-----------------------------------------------------------------------------------------------------------|--------------|----------|
| `enabled`          | `bool`     | If true, the receiver will pause reading a file and attempt to resend the current batch of logs on error. | `false`      | no       |
| `initial_interval` | `duration` | The time to wait after first failure to retry.                                                            | `1s`   | no       |
| `max_interval`     | `duration` | The maximum time to wait after applying backoff logic.                                                    | `30s` | no       |
| `max_elapsed_time` | `duration` | The maximum age of a message before the data is discarded.                                                | `5m`  | no       |

If `max_elapsed_time` is set to `0` data will never be discarded.

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### output block

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.syslog` does not export any fields.

## Component health

`otelcol.receiver.syslog` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.receiver.syslog` does not expose any component-specific debug
information.

## Debug metrics

`otelcol.receiver.syslog` does not expose any component-specific debug metrics.

## Example

This example proxies syslog messages from the `otelcol.receiver.syslog` receiver to the 
`otelcol.exporter.syslog` component, and then sends them on to a `loki.source.syslog` component
before being logged by a `loki.echo` component. This shows how the `otelcol` syslog components
can be used to proxy syslog messages before sending them to another destination.

Using the `otelcol` syslog components in this way results in the messages being forwarded as sent,
attempting to use the `loki.source.syslog` component for a similar proxy use case requires
careful mapping of any structured data fields through the `otelcol.processor.transform` component. A
very simple example of that can be found in the [`otelcol.exporter.syslog`][exporter-examples] documentation.

```alloy
otelcol.receiver.syslog "default" {
    protocol = "rfc5424"
    tcp {
        listen_address = "localhost:1515"
    }
    output {
        logs = [otelcol.exporter.syslog.default.input]
    }
}

otelcol.exporter.syslog "default" {
    endpoint = "localhost"
    network = "tcp"
    port = 1514
    protocol = "rfc5424"
    enable_octet_counting = false
    tls {
        insecure = true
    }
}

loki.source.syslog "default" {
  listener {
    address = "localhost:1514"
    protocol = "tcp"
    syslog_format = "rfc5424"
    label_structured_data = true
    use_rfc5424_message = true
  }
  forward_to = [loki.echo.default.receiver]
}

loki.echo "default" {}
```

[exporter-examples]: ../otelcol.exporter.syslog/#use-the-otelcolprocessortransform-component-to-format-logs-from-lokisourcesyslog
[encoding-documentation]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/syslogreceiver/README.md#supported-encodings
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.syslog` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

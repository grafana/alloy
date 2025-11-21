---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.syslog/
description: Learn about otelcol.exporter.syslog
labels:
  stage: public-preview
  products:
    - oss
title: otelcol.exporter.syslog
---

# `otelcol.exporter.syslog`

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.syslog` accepts logs from other `otelcol` components and writes them over the network using the syslog protocol.
It supports syslog protocols [RFC5424][] and [RFC3164][] and can send data over `TCP` or `UDP`.

{{< admonition type="note" >}}
`otelcol.exporter.syslog` is a wrapper over the upstream OpenTelemetry Collector [`syslog`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`syslog`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/syslogexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.syslog` components by giving them different labels.

[RFC5424]: https://www.rfc-editor.org/rfc/rfc5424
[RFC3164]: https://www.rfc-editor.org/rfc/rfc3164

## Usage

```alloy
otelcol.exporter.syslog "LABEL" {
  endpoint = "HOST"
}
```

### Supported Attributes

The exporter creates one syslog message for each log record based on the following attributes of the log record.
If an attribute is missing, the default value is used. The log's timestamp field is used for the syslog message's time.
RFC3164 only supports a subset of the attributes supported by RFC5424, and the default values aren't the same between the two protocols.
Refer to the [OpenTelemetry documentation][upstream_readme] for the exporter for more details.

| Attribute name    | Type   | RFC5424 Default value | RFC3164 supported | RFC3164 Default value |
| ----------------- | ------ | --------------------- | ----------------- | --------------------- |
| `appname`         | string | `-`                   | yes               | empty string          |
| `hostname`        | string | `-`                   | yes               | `-`                   |
| `message`         | string | empty string          | yes               | empty string          |
| `msg_id`          | string | `-`                   | no                |                       |
| `priority`        | int    | `165`                 | yes               | `165`                 |
| `proc_id`         | string | `-`                   | no                |                       |
| `structured_data` | map    | `-`                   | no                |                       |
| `version`         | int    | `1`                   | no                |                       |

[upstream_readme]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/<OTEL_VERSION>/exporter/syslogexporter

## Arguments

You can use the following arguments with `otelcol.exporter.syslog`:

| Name                    | Type       | Description                                           | Default     | Required |
| ----------------------- | ---------- | ----------------------------------------------------- | ----------- | -------- |
| `endpoint`              | `string`   | The endpoint to send syslog formatted logs to.        |             | yes      |
| `network`               | `string`   | The type of network connection to use to send logs.   | `"tcp"`     | no       |
| `port`                  | `int`      | The port where the syslog server accepts connections. | `514`       | no       |
| `protocol`              | `string`   | The syslog protocol that the syslog server supports.  | `"rfc5424"` | no       |
| `enable_octet_counting` | `bool`     | Whether to enable rfc6587 octet counting.             | `false`     | no       |
| `timeout`               | `duration` | Time to wait before marking a request as failed.      | `"5s"`      | no       |

The `network` argument specifies if the syslog endpoint is using the TCP or UDP protocol.
`network` must be one of `tcp`, `udp`.

The `protocol` argument specifies the syslog format supported by the endpoint.
`protocol` must be one of `rfc5424`, `rfc3164`.

## Blocks

You can use the following blocks with `otelcol.exporter.syslog`:

| Block                                  | Description                                                                    | Required |
| -------------------------------------- | ------------------------------------------------------------------------------ | -------- |
| [`debug_metrics`][debug_metrics]       | Configures the metrics that this component generates to monitor its state.     | no       |
| [`retry_on_failure`][retry_on_failure] | Configures retry mechanism for failed requests.                                | no       |
| [`sending_queue`][sending_queue]       | Configures batching of data before sending.                                    | no       |
| `sending_queue` > [`batch`][batch]     | Configures batching requests based on a timeout and a minimum number of items. | no       |
| [`tls`][tls]                           | Configures TLS for a TCP connection.                                           | no       |
| `tls` > [`tpm`][tpm]                   | Configures TPM settings for the TLS key_file.                                  | no       |

The > symbol indicates deeper levels of nesting.
For example, `tls` > `tpm` refers to a `tpm` block defined inside a `tls` block.

[tls]: #tls
[tpm]: #tpm
[sending_queue]: #sending_queue
[batch]: #batch
[retry_on_failure]: #retry_on_failure
[debug_metrics]: #debug_metrics

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to the syslog server are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures queueing and batching for the exporter.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS settings used for a connection to a TCP syslog server.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tpm`

The `tpm` block configures retrieving the TLS `key_file` from a trusted device.

{{< docs/shared lookup="reference/components/otelcol-tls-tpm-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
| ------- | ------------------ | ---------------------------------------------------------------- |
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for logs. Other telemetry signals are ignored.

## Component health

`otelcol.exporter.syslog` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.syslog` doesn't expose any component-specific debug information.

## Examples

### TCP endpoint without TLS

This example creates an exporter to send data to a syslog server expecting RFC5424-compliant messages over TCP without TLS:

```alloy
otelcol.exporter.syslog "default" {
  endpoint = "localhost"
  tls {
      insecure             = true
      insecure_skip_verify = true
  }
}
```

### Use the `otelcol.processor.transform` component to format logs from `loki.source.syslog`

This example shows one of the methods for annotating your Loki messages into the format expected by the exporter using a `otelcol.receiver.loki` component in addition to the `otelcol.processor.transform` component.
This example assumes that the log messages being parsed have come from a `loki.source.syslog` component.
This is just an example of some of the techniques that can be applied, and not a fully functioning example for a specific incoming log.

```alloy
otelcol.receiver.loki "default" {
  output {
    logs = [otelcol.processor.transform.syslog.input]
  }
}

otelcol.processor.transform "syslog" {
  error_mode = "ignore"

  log_statements {
    context = "log"

    statements = [
      `set(attributes["message"], attributes["__syslog_message"])`,
      `set(attributes["appname"], attributes["__syslog_appname"])`,
      `set(attributes["hostname"], attributes["__syslog_hostname"])`,

      // To set structured data you can chain index ([]) operations.
      `set(attributes["structured_data"]["auth@32473"]["user"], attributes["__syslog_message_sd_auth_32473_user"])`,
      `set(attributes["structured_data"]["auth@32473"]["user_host"], attributes["__syslog_message_sd_auth_32473_user_host"])`,
      `set(attributes["structured_data"]["auth@32473"]["valid"], attributes["__syslog_message_sd_auth_32473_authenticated"])`,
    ]
  }

  output {
    metrics = []
    logs    = [otelcol.exporter.syslog.default.input]
    traces  = []
  }
}
```

### Use the `otelcol.processor.transform` component to format OpenTelemetry logs

This example shows one of the methods for annotating your messages in the OpenTelemetry log format into the format expected by the exporter using an `otelcol.processor.transform` component.
This example assumes that the log messages being parsed have come from another OpenTelemetry receiver in JSON format (or have been transformed to OpenTelemetry logs using an `otelcol.receiver.loki` component).
This is just an example of some of the techniques that can be applied, and not a fully functioning example for a specific incoming log format.

```alloy
otelcol.processor.transform "syslog" {
  error_mode = "ignore"

  log_statements {
    context = "log"

    statements = [
      // Parse body as JSON and merge the resulting map with the cache map, ignoring non-json bodies.
      // cache is a field exposed by OTTL that is a temporary storage place for complex operations.
      `merge_maps(cache, ParseJSON(body), "upsert") where IsMatch(body, "^\\{")`,

      // Set some example syslog attributes using the values from a JSON message body
      // If the attribute doesn't exist in cache then nothing happens.
      `set(attributes["message"], cache["log"])`,
      `set(attributes["appname"], cache["application"])`,
      `set(attributes["hostname"], cache["source"])`,

      // To set structured data you can chain index ([]) operations.
      `set(attributes["structured_data"]["auth@32473"]["user"], attributes["user"])`,
      `set(attributes["structured_data"]["auth@32473"]["user_host"], cache["source"])`,
      `set(attributes["structured_data"]["auth@32473"]["valid"], cache["authenticated"])`,

      // Example priority setting, using facility 1 (user messages) and default to Info
      `set(attributes["priority"], 14)`,
      `set(attributes["priority"], 12) where severity_number == SEVERITY_NUMBER_WARN`,
      `set(attributes["priority"], 11) where severity_number == SEVERITY_NUMBER_ERROR`,
      `set(attributes["priority"], 10) where severity_number == SEVERITY_NUMBER_FATAL`,
    ]
  }

  output {
    metrics = []
    logs    = [otelcol.exporter.syslog.default.input]
    traces  = []
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.syslog` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

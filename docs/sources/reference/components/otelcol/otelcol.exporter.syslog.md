---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.syslog/
description: Learn about otelcol.exporter.syslog
title: otelcol.exporter.syslog
---

# otelcol.exporter.syslog

{{< docs/shared lookup="stability/public_preview.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.syslog` accepts logs from other `otelcol` components and writes them over the network using the syslog protocol. 
It supports syslog protocols [RFC5424][RFC5424] and [RFC3164][RFC3164] and can send data over `TCP` or `UDP`.

{{< admonition type="note" >}}
`otelcol.exporter.syslog` is a wrapper over the upstream OpenTelemetry Collector `syslog` exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
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

<!-- 
The next few paragraphs were copied from the OTel docs:
https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/syslogexporter
-->

### RFC5424

When configured with `protocol = rfc5424`, the exporter creates one syslog message for each log record,
based on the following record-level attributes of the log.
If an attribute is missing, the default value is used.
The log's timestamp field is used for the syslog message's time.

| Attribute name    | Type   | Default value  |
| ----------------- | ------ | -------------- |
| `appname`         | string | `-`            |
| `hostname`        | string | `-`            |
| `message`         | string | empty string   |
| `msg_id`          | string | `-`            |
| `priority`        | int    | `165`          |
| `proc_id`         | string | `-`            |
| `structured_data` | map    | `-`            |
| `version`         | int    | `1`            |

Here's a simplified representation of an input log record:

```json
{
  "body": "",
  "timeUnixNano": 1065903255003000000,
  "attributes":
  {
    "appname": "su",
    "hostname": "mymachine.example.com",
    "message": "'su root' failed for lonvick on /dev/pts/8",
    "priority": 34,
  }
}
```

And here's the output message based on the above log record:

```console
<34>1 2003-10-11T22:14:15.003Z mymachine.example.com su - - - 'su root' failed for lonvick on /dev/pts/8
```

Here'a another example, this includes the structured data and other attributes:

```json
{
  "body": "",
  "timeUnixNano": 1438811939693012000,
  "attributes":
  {
    "appname": "SecureAuth0",
    "hostname": "192.168.2.132",
    "message": "Found the user for retrieving user's profile",
    "msg_id": "ID52020",
    "priority": 86,
    "proc_id": "23108",
    "structured_data":
    {
      "SecureAuth@27389":
      {
        "UserHostAddress":"192.168.2.132",
        "Realm":"SecureAuth0",
        "UserID":"Tester2",
        "PEN":"27389"
      }
    },
    "version": 1
  }
}
```

Output:

```console
<86>1 2015-08-05T21:58:59.693012Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile
```

### RFC3164

When configured with `protocol = rfc3164`, the exporter creates one syslog message for each log record,
based on the following record-level attributes of the log.
If an attribute is missing, the default value is used.
The log's timestamp field is used for the syslog message's time.

| Attribute name    | Type   | Default value  |
| ----------------- | ------ | -------------- |
| `appname`         | string | empty string   |
| `hostname`        | string | `-`            |
| `message`         | string | empty string   |
| `priority`        | int    | `165`          |

Here's a simplified representation of an input log record:

```json
{
  "body": "",
  "timeUnixNano": 1697062455000000000,
  "attributes":
  {
    "appname": "su",
    "hostname": "mymachine",
    "message": "'su root' failed for lonvick on /dev/pts/8",
    "priority": 34
  }
}
```

Output:

```console
<34>Oct 11 22:14:15 mymachine su: 'su root' failed for lonvick on /dev/pts/8
```

<!--
This ends the section copied from the OTel docs
-->

## Arguments

`otelcol.exporter.syslog` supports the following arguments:

| Name                   | Type      | Description                                                               | Default                           | Required |
|------------------------|-----------|---------------------------------------------------------------------------|-----------------------------------|----------|
| `endpoint`             | `string`  | The endpoint to send syslog formatted logs to.                            |                                   | yes      |
| `network`              | `string`  | The type of network connection to use to send logs.                       | tcp                               | no       |
| `port`                 | `int`     | The port where the syslog server accepts connections.                     | 514                               | no       |
| `protocol`             | `string`  | The syslog protocol that the syslog server supports.                      | rfc5424                           | no       |
| `enable_octet_counting`| `bool`    | Whether to enable rfc6587 octet counting.                                 | false                             | no       |
| `timeout`              | `duration`| Time to wait before marking a request as failed.                          | 5s                                | no       |

The `network` argument specifies if the syslog endpoint is using the TCP or UDP protocol. 
`network` must be one of `tcp`, `udp`

The `protocol` argument specifies the syslog format supported by the endpoint.
`protocol` must be one of `rfc5424`, `rfc3164`

## Blocks

The following blocks are supported inside the definition of `otelcol.exporter.syslog`:

| Hierarchy        | Block                | Description                                                                | Required |
|------------------|----------------------|----------------------------------------------------------------------------|----------|
| tls              | [tls][]              | Configures TLS for a TCP connection.                                       | no       |
| sending_queue    | [sending_queue][]    | Configures batching of data before sending.                                | no       |
| retry_on_failure | [retry_on_failure][] | Configures retry mechanism for failed requests.                            | no       |
| debug_metrics    | [debug_metrics][]    | Configures the metrics that this component generates to monitor its state. | no       |

[tls]: #tls-block
[sending_queue]: #sending_queue-block
[retry_on_failure]: #retry_on_failure-block
[debug_metrics]: #debug_metrics-block

### tls block

The `tls` block configures TLS settings used for a connection to a TCP syslog server.

{{< docs/shared lookup="reference/components/otelcol-tls-client-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### sending_queue block

The `sending_queue` block configures an in-memory buffer of batches before data is sent to the syslog server.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### retry_on_failure block

The `retry_on_failure` block configures how failed requests to the syslog server are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name   | Type               | Description
|--------|--------------------|-----------------------------------------------------------------
|`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for logs. Other telemetry signals are ignored.

## Component health

`otelcol.exporter.syslog` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.syslog` doesn't expose any component-specific debug information.

## Examples

### TCP endpoint without TLS

This example creates an exporter to send data to a syslog server expecting rfc5424 compliant messages over TCP without TLS:

```alloy
otelcol.exporter.syslog "default" {
  endpoint = "localhost"
  tls {
      insecure             = true
      insecure_skip_verify = true
  }
}
```

### Using the `otelcol.processor.transform` component to format logs from `loki.source.syslog`

This example shows one of the methods for annotating your loki messages into the format expected 
by the exporter using a `otelcol.receiver.loki` component in addition to the `otelcol.processor.transform` 
component. This example assumes that the log messages being parsed have come from a `loki.source.syslog` 
component. This is just an example of some of the techniques that can be applied, and not a fully functioning 
example for a specific incoming log.

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

### Using the `otelcol.processor.transform` component to format otel logs

This example shows one of the methods for annotating your messages in the OpenTelemetry log format into the format expected 
by the exporter using an `otelcol.processor.transform` component. This example assumes that the log messages being 
parsed have come from another OpenTelemetry receiver in JSON format (or have been transformed to OpenTelemetry logs using 
an `otelcol.receiver.loki` component). This is just an example of some of the techniques that can be applied, and not a 
fully functioning example for a specific incoming log format.

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

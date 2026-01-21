---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.cloudflare/
description: Learn about otelcol.receiver.cloudflare
labels:
  stage: experimental
  products:
    - oss
title: otelcol.receiver.cloudflare
---

# `otelcol.receiver.cloudflare`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.cloudflare` receives logs sent by Cloudflare [LogPush](https://developers.cloudflare.com/logs/logpush/) jobs.

{{< admonition type="note" >}}
`otelcol.receiver.cloudflare` is a wrapper over the upstream OpenTelemetry Collector [`cloudflare`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`cloudflare`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/cloudflarereceiver
{{< /admonition >}}

You can specify multiple `otelcol.receiver.cloudflare` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.cloudflare "<LABEL>" {
  endpoint = "<HOST>:<PORT>"

  output {
    logs = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.cloudflare`:

| Name               | Type                | Description                                                                                                | Default                | Required |
| ------------------ | ------------------- | ---------------------------------------------------------------------------------------------------------- | ---------------------- | -------- |
| `endpoint`         | `string`            | The `<HOST:PORT>` endpoint address on which the receiver awaits requests from Cloudflare.                  |                        | yes      |
| `secret`           | `string`            | If this value is set, the receiver expects to see it in any valid requests under the `X-CF-Secret` header. |                        | no       |
| `attributes`       | `map[string]string` | Sets log attributes from message fields. Only string, boolean, integer, or float fields can be mapped.     |                        | no       |
| `delimiter`        | `string`            | The separator to join nested fields in the log message when setting attributes.                            | `"."`                  | no       |
| `timestamp_field`  | `string`            | Log field name that contains timestamp.                                                                    | `"EdgeStartTimestamp"` | no       |
| `timestamp_format` | `string`            | One of `unix`, `unixnano`, or `rfc3339`, matching how your LogPush job encodes the timestamp field.        | `"rfc3339"`            | no       |

When the `attributes` configuration is empty, the receiver will automatically ingest all fields from the log messages as attributes, using the original field names as attribute names.

Refer to the upstream receiver [documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/cloudflarereceiver#configuration) for more details.

## Blocks

You can use the following blocks with `otelcol.receiver.cloudflare`:

| Block              | Description                                       | Required |
| ------------------ | ------------------------------------------------- | -------- |
| [`output`][output] | Configures where to send received telemetry data. | yes      |
| [`tls`][tls]       | Custom server TLS configuration.                  | no       |

[output]: #output
[tls]: #tls

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls`

The `tls` block configures TLS settings used for a server.
If the `tls` block isn't provided, TLS won't be used for connections to the server.

{{< docs/shared lookup="reference/components/otelcol-tls-server-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.cloudflare` doesn't export any fields.

## Component health

`otelcol.receiver.cloudflare` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.cloudflare` doesn't expose any component-specific debug information.

## Example

The following example receives logs from Cloudflare and forwards them through a batch processor:

```alloy
otelcol.receiver.cloudflare "default" {
  endpoint = "<HOST>:<PORT>"
  secret = "1234567890abcdef1234567890abcdef"
  timestamp_field = "EdgeStartTimestamp"
  timestamp_format = "rfc3339"
  attributes = {
    ClientIP = "http_request.client_ip",
    ClientRequestURI = "http_request.uri",
  }

  tls {
    cert_file = "/path/to/cert.pem"
    key_file = "/path/to/key.pem"
  }

  output {
    logs = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    logs = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
  client {
    endpoint = env("<OTLP_ENDPOINT>")
  }
}
```

Replace the following:

- _`<HOST>`_: The hostname or IP address where the receiver listens for Cloudflare LogPush requests.
- _`<PORT>`_: The port number where the receiver listens for Cloudflare LogPush requests.
- _`<OTLP_ENDPOINT>`_: The OTLP endpoint URL for your observability backend.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.cloudflare` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

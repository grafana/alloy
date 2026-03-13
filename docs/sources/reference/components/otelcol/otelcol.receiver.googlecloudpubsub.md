---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.receiver.googlecloudpubsub/
description: Learn about otelcol.receiver.googlecloudpubsub
labels:
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: otelcol.receiver.googlecloudpubsub
---

# `otelcol.receiver.googlecloudpubsub`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.receiver.googlecloudpubsub` receives OpenTelemetry signals from a Google Cloud Pub/Sub subscription and forwards them to other `otelcol.*` components for processing or export.

{{< admonition type="note" >}}
`otelcol.receiver.googlecloudpubsub` is a wrapper over the upstream OpenTelemetry Collector [`googlecloudpubsub`][] receiver.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`googlecloudpubsub`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/googlecloudpubsubreceiver
{{< /admonition >}}

You can specify multiple `otelcol.receiver.googlecloudpubsub` components by giving them different labels.

## Usage

```alloy
otelcol.receiver.googlecloudpubsub "<LABEL>" {
  subscription = "projects/<PROJECT-ID>/subscriptions/<SUBSCRIPTION-NAME>"

  output {
    logs = [...]
    metrics = [...]
    trace = [...]
  }
}
```

## Arguments

You can use the following arguments with `otelcol.receiver.googlecloudpubsub`:

| Name                    | Type       | Description                                                                                                                                                                                                                | Default | Required |
| ----------------------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `subscription`          | `string`   | The subscription name to receive OTLP data from. The subscription name should be a fully qualified resource name, for example: `projects/otel-project/subscriptions/otlp`.                                                 | `""`    | yes      |
| `compression`           | `string`   | The compression used on data received from the subscription. Only `gzip` is supported. This is only used when no content-encoding attribute is present.                                                                    | `""`    | no       |
| `encoding`              | `string`   | The encoding used to receive data from the subscription. This can either be `otlp_proto_trace`, `otlp_proto_metric`, `otlp_proto_log` or an encoding extension. This is only used when no media type attribute is present. | `""`    | no       |
| `endpoint`              | `string`   | Override the default Pub/Sub endpoint. This is useful when connecting to the Pub/Sub emulator instance or switching between [global and regional service endpoints][].                                                     | `""`    | no       |
| `ignore_encoding_error` | `bool`     | Ignore errors when the configured encoder fails to decode Pub/Sub messages. Ignoring the error causes the receiver to drop the message.                                                                                    | false   | no       |
| `insecure`              | `bool`     | Allows performing insecure SSL connections and transfers. This is useful when connecting to a local emulator instance. Only has effect if you set `endpoint`.                                                              | false   | no       |
| `project`               | `string`   | The Google Cloud Project project identifier.                                                                                                                                                                               | `""`    | no       |
| `timeout`               | `Duration` | Timeout for calls to the Pub/Sub API.                                                                                                                                                                                      | `"12s"` | no       |

[global and regional service endpoints]: https://cloud.google.com/pubsub/docs/reference/service_apis_overview#service_endpoints

## Blocks

You can use the following blocks with `otelcol.receiver.googlecloudpubsub`:

| Block                            | Description                                                                | Required |
| -------------------------------- | -------------------------------------------------------------------------- | -------- |
| [`output`][output]               | Configures where to send received telemetry data.                          | yes      |
| [`debug_metrics`][debug_metrics] | Configures the metrics that this component generates to monitor its state. | no       |

[debug_metrics]: #debug_metrics
[output]: #output

### `output`

{{< badge text="Required" >}}

{{< docs/shared lookup="reference/components/output-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`otelcol.receiver.googlecloudpubsub` doesn't export any fields.

## Component health

`otelcol.receiver.googlecloudpubsub` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.receiver.googlecloudpubsub` doesn't expose any component-specific debug information.

## Example

The following example collects signals from Google Cloud Pub/Sub subscription and forwards logs through a batch processor:

```alloy
otelcol.receiver.googlecloudpubsub "default" {
  subscription = "projects/my-gcp-project/subscriptions/my-pubsub-subscription"

  output {
    logs = [otelcol.processor.batch.default.input]
  }
}

otelcol.processor.batch "default" {
  output {
    logs = [otelcol.exporter.otlphttp.default.input]
  }
}

otelcol.exporter.otlphttp "default" {
  client {
    endpoint = env("<OTLP_ENDPOINT>")
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.receiver.googlecloudpubsub` can accept arguments from the following components:

- Components that export [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-exporters)

`otelcol.receiver.googlecloudpubsub` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

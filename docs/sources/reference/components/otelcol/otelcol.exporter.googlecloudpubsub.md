---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.googlecloudpubsub/
description: Learn about otelcol.exporter.googlecloudpubsub
labels:
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: otelcol.exporter.googlecloudpubsub
---

# `otelcol.exporter.googlecloudpubsub`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.googlecloudpubsub` accepts metrics, traces, and logs from other `otelcol` components and sends it to Google Cloud Pub/Sub Topic.

{{< admonition type="note" >}}
`otelcol.exporter.googlecloudpubsub` is a wrapper over the upstream OpenTelemetry Collector [`googlecloudpubsub`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`googlecloudpubsub`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/googlecloudpubsubexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.googlecloudpubsub` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.googlecloudpubsub "<LABEL>" {
    project = "<PROJECT-ID>"
    topic   = "projects/<PROJECT-ID>/topics/<TOPIC-NAME>"
}
```

### Authenticating

Refer to the [Google Cloud Pub/Sub Exporter][] and [Google Cloud Exporter][] documentation for more detailed information about authentication.

[Google Cloud Pub/Sub Exporter]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/googlecloudpubsubexporter/README.md
[Google Cloud Exporter]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/googlecloudexporter#prerequisite-authenticating

## Arguments

You can use the following arguments with `otelcol.exporter.googlecloudpubsub`:

| Name          | Type       | Description                                                                                                                                                            | Default                                         | Required |
| ------------- |------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------| -------- |
| `topic`       | `string`   | The topic name to send OTLP data over. The topic name should be a fully qualified resource name, for example, `projects/otel-project/topics/otlp`.                     | `""`                                            | yes      |
| `compression` | `string`   | The compression used on the data sent to the topic. Only `gzip` is supported. Default is no compression.                                                               | `""`                                            | no       |
| `endpoint`    | `string`   | Override the default Pub/Sub endpoint. This is useful when connecting to the Pub/Sub emulator instance or switching between [global and regional service endpoints][]. | `""`                                            | no       |
| `insecure`    | `bool`     | Allows performing insecure SSL connections and transfers. This is useful when connecting to a local emulator instance. Only has effect if you set `endpoint`.          | `false`                                         | no       |
| `project`     | `string`   | Google Cloud Platform project identifier.                                                                                                                              | Fetch from credentials                          | no       |
| `timeout`     | `Duration` | Timeout for calls to the Pub/Sub API.                                                                                                                                  | `"12s"`                                         | no       |
| `user_agent`  | `string`   | Override the user agent string on requests to Cloud Monitoring. This only applies to metrics. Specify `{{version}}` to include the application version number.         | `"opentelemetry-collector-contrib {{version}}"` | no       |

[global and regional service endpoints]: https://cloud.google.com/pubsub/docs/reference/service_apis_overview#service_endpoints

## Blocks

You can use the following blocks with `otelcol.exporter.googlecloudpubsub`:

| Block                                  | Description                                                                                     | Required |
| -------------------------------------- | ----------------------------------------------------------------------------------------------- | -------- |
| [`debug_metrics`][debug_metrics]       | Configures the metrics that this component generates to monitor its state.                      | no       |
| [`ordering`][ordering]                 | Configures the [Pub/Sub ordering](https://cloud.google.com/pubsub/docs/ordering) feature.       | no       |
| [`retry_on_failure`][retry_on_failure] | Configures the retry behavior when the receiver encounters an error downstream in the pipeline. | no       |
| [`sending_queue`][sending_queue]       | Configures batching of data before sending.                                                     | no       |
| [`watermark`][watermark]               | Behaviour of how the ce-time attribute is set.                                                  | no       |

[debug_metrics]: #debug_metrics
[ordering]: #ordering
[retry_on_failure]: #retry_on_failure
[sending_queue]: #sending_queue
[watermark]: #watermark

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `ordering`

The following arguments are supported:

| Name                        | Type     | Description                                                                                                                                                                            | Default | Required |
| --------------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `enabled`                   | `bool`   | Enables ordering.                                                                                                                                                                      | `false` | no       |
| `from_resource_attribute`   | `string` | Resource attribute used as the ordering key. Required when `enabled` is `true`. If the resource attribute is missing or has an empty value, messages aren't ordered for this resource. | `""`    | no       |
| `remove_resource_attribute` | `string` | Whether the ordering key resource attribute specified `from_resource_attribute` should be removed from the resource attributes.                                                        | `""`    | no       |

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to Datadog are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures an in-memory buffer of batches before data is sent to the HTTP server.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `watermark`

The following arguments are supported:

| Name          | Type       | Description                                                                                                                                                                                | Default | Required |
| ------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------- | -------- |
| `behavior`    | `string`   | `current` sets the `ce-time` attribute to the system clock, `earliest` sets the attribute to the smallest timestamp of all the messages.                                                   | `""`    | no       |
| `allow_drift` | `Duration` | The maximum difference the `ce-time` attribute can have from the system clock. If you set `allow_drift` to `0s` and `behavior` to `earliest`, the maximum drift from the clock is allowed. | `0s`    | no       |

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                 |
|---------|--------------------|-------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal , including metrics, logs, and traces.

## Component health

`otelcol.exporter.googlecloudpubsub` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.googlecloudpubsub` doesn't expose any component-specific debug information.

## Example

This example scrapes logs from local files through a receiver for conversion to OpenTelemetry format before finally sending them to Pub/Sub.

This configuration includes the recommended `memory_limiter` and `batch` plugins, which avoid high reporting latency and ensure the collector stays stable by dropping telemetry when memory limits are reached.

```alloy
local.file_match "logs" {
  path_targets = [{
    __address__ = "localhost",
    __path__    = "/var/log/{syslog,messages,*.log}",
    instance    = constants.hostname,
    job         = "integrations/node_exporter",
  }]
}

loki.source.file "logs" {
  targets    = local.file_match.logs.targets
  forward_to = [otelcol.receiver.loki.gcp.receiver]
}

otelcol.receiver.loki "gcp" {
  output {
    logs = [otelcol.processor.memory_limiter.gcp.input]
  }
}

otelcol.processor.memory_limiter "gcp" {
  check_interval = "1s"
  limit = "200MiB"

  output {
    metrics = [otelcol.processor.batch.gcp.input]
    logs = [otelcol.processor.batch.gcp.input]
    traces = [otelcol.processor.batch.gcp.input]
  }
}

otelcol.processor.batch "gcp" {
  output {
    metrics = [otelcol.exporter.googlecloudpubsub.default.input]
    logs = [otelcol.exporter.googlecloudpubsub.default.input]
    traces = [otelcol.exporter.googlecloudpubsub.default.input]
  }
}

otelcol.exporter.googlecloudpubsub "default" {
  project = "my-gcp-project"
  topic = "projects/<my-gcp-project>/topics/my-pubsub-topic"
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.googlecloudpubsub` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

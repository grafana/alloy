---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.datadog/
description: Learn about otelcol.exporter.datadog
aliases:
  - ../otelcol.exporter.datadog/ # /docs/alloy/latest/reference/components/otelcol.exporter.datadog/
labels:
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: otelcol.exporter.datadog
---

# `otelcol.exporter.datadog`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.datadog` accepts metrics and traces telemetry data from other `otelcol` components and sends it to Datadog.

{{< admonition type="note" >}}
`otelcol.exporter.datadog` is a wrapper over the upstream OpenTelemetry Collector [`datadog`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`datadog`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/datadogexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.datadog` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.datadog "<LABEL>" {
    api {
        api_key = "<YOUR_API_KEY_HERE>"
    }
}
```

## Arguments

You can use the following arguments with `otelcol.exporter.datadog`:

| Name                         | Type       | Description                                                                      | Default | Required |
|------------------------------|------------|----------------------------------------------------------------------------------|---------|----------|
| `hostname`                   | `string`   | The fallback hostname used for payloads without hostname-identifying attributes. |         | no       |
| `hostname_detection_timeout` | `duration` | The timeout for hostname detection.                                              | `25s`   | no       |
| `only_metadata`              | `bool`     | Whether to send only metadata.                                                   | `false` | no       |

If `hostname` is unset, the hostname is determined automatically.
For more information, refer to the Datadog [Fallback hostname logic](https://docs.datadoghq.com/opentelemetry/schema_semantics/hostname/?tab=datadogexporter#fallback-hostname-logic) documentation.
This option won't change the hostname applied to metrics or traces if they already have hostname-identifying attributes.

## Blocks

You can use the following blocks with `otelcol.exporter.datadog`:

| Block                                  | Description                                                                    | Required |
|----------------------------------------|--------------------------------------------------------------------------------|----------|
| [`api`][api]                           | Configures authentication with Datadog                                         | yes      |
| [`client`][client]                     | Configures the HTTP client used to send telemetry data.                        | no       |
| [`debug_metrics`][debug_metrics]       | Configures the metrics that this component generates to monitor its state.     | no       |
| [`host_metadata`][host_metadata]       | Host metadata specific configuration.                                          | no       |
| [`logs`][logs]                         | Logs exporter specific configuration.                                          | no       |
| [`metrics`][metrics]                   | Metric exporter specific configuration.                                        | no       |
| metrics > [`exporter`][exporter]       | Metric Exporter specific configuration.                                        | no       |
| metrics > [`histograms`][histograms]   | Histograms specific configuration.                                             | no       |
| metrics > [`summaries`][summaries]     | Summaries specific configuration                                               | no       |
| metrics > [`sums`][sums]               | Sums specific configuration                                                    | no       |
| [`retry_on_failure`][retry_on_failure] | Configures retry mechanism for failed requests.                                | no       |
| [`sending_queue`][queue]               | Configures batching of data before sending.                                    | no       |
| `sending_queue` > [`batch`][batch]     | Configures batching requests based on a timeout and a minimum number of items. | no       |
| [`traces`][traces]                     | Trace exporter specific configuration.                                         | no       |

The > symbol indicates deeper levels of nesting.
For example, `metrics` > `summaries` refers to a `summaries` block defined inside a `metrics` block.

[api]: #api
[traces]: #traces
[logs]: #logs
[metrics]: #metrics
[exporter]: #exporter
[histograms]: #histograms
[sums]: #sums
[summaries]: #summaries
[host_metadata]: #host_metadata
[client]: #client
[retry_on_failure]: #retry_on_failure
[queue]: #sending_queue
[debug_metrics]: #debug_metrics

### `api`

{{< badge text="Required" >}}

The `api` block configures authentication with the Datadog API.
This is required to send telemetry to Datadog.
If you don't provide the `api` block, you can't send telemetry to Datadog.

The following arguments are supported:

| Name                  | Type     | Description                                           | Default           | Required |
|-----------------------|----------|-------------------------------------------------------|-------------------|----------|
| `api_key`             | `secret` | API Key for Datadog                                   |                   | yes      |
| `fail_on_invalid_key` | `bool`   | Whether to exit at startup on an invalid API key      | `false`           | no       |
| `site`                | `string` | The site of the Datadog intake to send Agent data to. | `"datadoghq.com"` | no       |

### `client`

The `client` block configures the HTTP client used by the component.
Not all fields are supported by the Datadog Exporter.

The following arguments are supported:

| Name                      | Type       | Description                                                                 | Default | Required |
|---------------------------|------------|-----------------------------------------------------------------------------|---------|----------|
| `disable_keep_alives`     | `bool`     | Disable HTTP keep-alive.                                                    |         | no       |
| `idle_conn_timeout`       | `duration` | Time to wait before an idle connection closes itself.                       | `"45s"` | no       |
| `insecure_skip_verify`    | `bool`     | Ignores insecure server TLS certificates.                                   |         | no       |
| `max_conns_per_host`      | `int`      | Limits the total (dialing,active, and idle) number of connections per host. |         | no       |
| `max_idle_conns_per_host` | `int`      | Limits the number of idle HTTP connections the host can keep open.          | `5`     | no       |
| `max_idle_conns`          | `int`      | Limits the number of idle HTTP connections the client can keep open.        | `100`   | no       |
| `read_buffer_size`        | `string`   | Size of the read buffer the HTTP client uses for reading server responses.  |         | no       |
| `timeout`                 | `duration` | Time to wait before marking a request as failed.                            | `"15s"` | no       |
| `write_buffer_size`       | `string`   | Size of the write buffer the HTTP client uses for writing requests.         |         | no       |

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `host_metadata`

The `host_metadata` block configures the host metadata configuration.
Host metadata is the information used to populate the infrastructure list and the host map, and provide host tags functionality within the Datadog app.

The following arguments are supported:

| Name              | Type           | Description                                                | Default              | Required |
|-------------------|----------------|------------------------------------------------------------|----------------------|----------|
| `enabled`         | `bool`         | Enable the host metadata functionality                     | `true`               | no       |
| `hostname_source` | `string`       | Source for the hostname of host metadata.                  | `"config_or_system"` | no       |
| `tags`            | `list(string)` | List of host tags to be sent as part of the host metadata. |                      | no       |

By default, the exporter only sends host metadata for a single host, whose name is chosen according to `host_metadata::hostname_source`.

Valid values for `hostname_source` are:

* `"first_resource"` picks the host metadata hostname from the resource attributes on the first OTLP payload that gets to the exporter. 
  If the first payload lacks hostname-like attributes, it will fallback to 'config_or_system' behavior. **Don't use this hostname source if receiving data from multiple hosts**.
* `"config_or_system"` picks the host metadata hostname from the 'hostname' setting, falling back to system and cloud provider APIs.

### `logs`

The `logs` block configures the logs exporter settings.

The following arguments are supported:

| Name                | Type     | Description                                                                                                                                   | Default                                    | Required |
|---------------------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------|----------|
| `batch_wait`        | `int`    | The maximum time in seconds the logs agent waits to fill each batch of logs before sending.                                                   | `5`                                        | no       |
| `compression_level` | `int`    | Accepts values from 0 (no compression) to 9 (maximum compression but higher resource usage). Only used if `use_compression` is set to `true`. | `6`                                        | no       |
| `endpoint`          | `string` | The host of the Datadog intake server to send logs to.                                                                                        | `"https://http-intake.logs.datadoghq.com"` | no       |
| `use_compression`   | `bool`   | Available when sending logs via HTTPS. Compresses logs if enabled.                                                                            | `true`                                     | no       |

If `use_compression` is disabled, `compression_level` has no effect.

If `endpoint` is unset, the value is obtained through the `site` parameter in the [`api`][] section.

### `metrics`

The `metrics` block configures Metric specific exporter settings.

The following arguments are supported:

| Name        | Type     | Description                                                             | Default                       | Required |
|-------------|----------|-------------------------------------------------------------------------|-------------------------------|----------|
| `delta_ttl` | `number` | The number of seconds values are kept in memory for calculating deltas. | `3600`                        | no       |
| `endpoint`  | `string` | The host of the Datadog intake server to send metrics to.               | `"https://api.datadoghq.com"` | no       |

Any of the subset of resource attributes in the [semantic mapping list](https://docs.datadoghq.com/opentelemetry/guide/semantic_mapping/) are converted to Datadog conventions and set to metric tags whether `resource_attributes_as_tags` is enabled or not.

If `endpoint` is unset, the value is obtained through the `site` parameter in the [`api`][] section.

### `exporter`

The `exporter` block configures the metric exporter settings.

The following arguments are supported:

| Name                                     | Type   | Description                                                                               | Default | Required |
|------------------------------------------|--------|-------------------------------------------------------------------------------------------|---------|----------|
| `instrumentation_scope_metadata_as_tags` | `bool` | Set to `false` to not add metadata about the instrumentation scope that created a metric. | `true`  | no       |
| `resource_attributes_as_tags`            | `bool` | Set to `true` to add resource attributes of a metric to its metric tags.                  | `false` | no       |

### `histograms`

The `histograms` block configures the histogram settings.

The following arguments are supported:

| Name                       | Type     | Description                                                               | Default           | Required |
|----------------------------|----------|---------------------------------------------------------------------------|-------------------|----------|
| `mode`                     | `string` | How to report histograms.                                                 | `"distributions"` | no       |
| `send_aggregation_metrics` | `bool`   | Whether to report sum, count, min, and max as separate histogram metrics. | `false`           | no       |

Valid values for `mode` are:

* `"distributions"` to report metrics as Datadog distributions (recommended).
* `"nobuckets"` to not report bucket metrics.
* `"counters"` to report one metric per histogram bucket.

### `summaries`

The `summaries` block configures the summary settings.

The following arguments are supported:

| Name   | Type     | Description              | Default    | Required |
|--------|----------|--------------------------|------------|----------|
| `mode` | `string` | How to report summaries. | `"gauges"` | no       |

Valid values for `mode` are:

* `"noquantiles"` to not report quantile metrics.
* `"gauges"` to report one gauge metric per quantile.

### `sums`

The `sums` block configures the sums settings.

The following arguments are supported:

| Name                                 | Type     | Description                                                    | Default      | Required |
|--------------------------------------|----------|----------------------------------------------------------------|--------------|----------|
| `cumulative_monotonic_mode`          | `string` | How to report cumulative monotonic sums.                       | `"to_delta"` | no       |
| `initial_cumulative_monotonic_value` | `string` | How to report the initial value for cumulative monotonic sums. | `"auto"`     | no       |

Valid values for `cumulative_monotonic_mode` are:

* `"to_delta"` to calculate delta for sum in the client side and report as Datadog counts.
* `"raw_value"` to report the raw value as a Datadog gauge.

Valid values for `initial_cumulative_monotonic_value` are:

* `"auto"` reports the initial value if its start timestamp is set, and it happens after the process was started.
* `"drop"` always drops the initial value.
* `"keep"` always reports the initial value.

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to Datadog are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures queueing and batching for the exporter.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `traces`

The `traces` block configures the trace exporter settings.

The following arguments are supported:

| Name                             | Type           | Description                                                                                        | Default                               | Required |
|----------------------------------|----------------|----------------------------------------------------------------------------------------------------|---------------------------------------|----------|
| `compute_stats_by_span_kind`     | `bool`         | Enables APM stats computation based on `span.kind`                                                 | `true`                                | no       |
| `compute_top_level_by_span_kind` | `bool`         | Enables top-level span identification based on `span.kind`                                         | `false`                               | no       |
| `endpoint`                       | `string`       | The host of the Datadog intake server to send traces to.                                           | `"https://trace.agent.datadoghq.com"` | no       |
| `ignore_resources`               | `list(string)` | A blocklist of regular expressions can be provided to disable traces based on their resource name. |                                       | no       |
| `peer_tags_aggregation`          | `bool`         | Enables aggregation of peer related tags in Datadog exporter                                       | `false`                               | no       |
| `peer_tags`                      | `list(string)` | List of supplementary peer tags that go beyond the defaults.                                       |                                       | no       |
| `span_name_as_resource_name`     | `bool`         | Use OpenTelemetry semantic convention for span naming                                              | `true`                                | no       |
| `span_name_remappings`           | `map(string)`  | A map of Datadog span operation name keys and preferred name values to update those names to.      |                                       | no       |
| `trace_buffer`                   | `number`       | Specifies the number of outgoing trace payloads to buffer before dropping                          | `10`                                  | no       |

If `compute_stats_by_span_kind` is disabled, only top-level and measured spans will have stats computed.
If you are sending OTel traces and want stats on non-top-level spans, this flag must be set to `true`.
If you are sending OTel traces and don't want stats computed by span kind, you must disable this flag and disable `compute_top_level_by_span_kind`.

If `endpoint` is unset, the value is obtained through the `site` parameter in the [`api`][] section.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                 |
|---------|--------------------|-------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.datadog` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.datadog` doesn't expose any component-specific debug information.

## Example

### Forward Prometheus Metrics

This example forwards Prometheus metrics from {{< param "PRODUCT_NAME" >}} through a receiver for conversion to Open Telemetry format before finally sending them to Datadog.
If you are using the US Datadog APIs, the `api` field is required for the exporter to function.

```alloy
prometheus.exporter.self "default" {
}

prometheus.scrape "metamonitoring" {
  targets    = prometheus.exporter.self.default.targets
  forward_to = [otelcol.receiver.prometheus.default.receiver]
}

otelcol.receiver.prometheus "default" {
  output {
    metrics = [otelcol.exporter.datadog.default.input]
  }
}


otelcol.exporter.datadog "default" {
    api {
        api_key = "API_KEY"
    }

     metrics {
        endpoint = "https://api.ap1.datadoghq.com"
        resource_attributes_as_tags = true
    }
}
```

### Full OTel pipeline

This example forwards metrics and traces received in Datadog format to {{< param "PRODUCT_NAME" >}}, converts them to OTel format, and exports them to Datadog.

```alloy
otelcol.receiver.datadog "default" {
    output {
        metrics = [otelcol.exporter.otlphttp.default.input, otelcol.exporter.datadog.default input]
        traces  = [otelcol.exporter.otlphttp.default.input, otelcol.exporter.datadog.default.input]
    }
}

otelcol.exporter.otlphttp "default" {
    client {
        endpoint = "database:4317"
    }
}

otelcol.exporter.datadog "default" {
    client {
        timeout = "10s"
    }

    api {
        api_key             = "abc"
        fail_on_invalid_key = true
    }

    traces {
        endpoint             = "https://trace.agent.datadoghq.com"
        ignore_resources     = ["(GET|POST) /healthcheck"]
        span_name_remappings = {
            "instrumentation:express.server" = "express",
        }
    }

    metrics {
        delta_ttl = 1200
        endpoint  = "https://api.datadoghq.com"

        exporter {
            resource_attributes_as_tags = true
        }

        histograms {
            mode = "counters"
        }

        sums {
            initial_cumulative_monotonic_value = "keep"
        }

        summaries {
            mode = "noquantiles"
        }
    }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.datadog` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

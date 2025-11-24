---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.splunkhec/
description: Learn about otelcol.exporter.splunkhec
labels:
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: otelcol.exporter.splunkhec
---

# `otelcol.exporter.splunkhec`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.splunkhec` accepts metrics and traces telemetry data from other `otelcol` components and sends it to Splunk HEC.

{{< admonition type="note" >}}
`otelcol.exporter.splunkhec` is a wrapper over the upstream OpenTelemetry Collector [`splunkhec`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`splunkhec`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/splunkhecexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.splunkhec` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.splunkhec "<LABEL>" {
    splunk {
        token = "<YOUR_SPLUNK_TOKEN>"
    }
    client {
        endpoint = "http://splunk.yourdomain.com:8088"
    }
}
```

## Arguments

The `otelcol.exporter.splunkhec` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `otelcol.exporter.splunkhec`:

| Block                                                      | Description                                                                    | Required |
|------------------------------------------------------------|--------------------------------------------------------------------------------|----------|
| [`splunk`][splunk]                                         | Configures the Splunk HEC exporter.                                            | yes      |
| `splunk` > [`batcher`][batcher]                            | (Deprecated) Configures batching requests based on a timeout and a minimum number of items. | no       |
| `splunk` > [`heartbeat`][heartbeat]                        | Configures the exporters heartbeat settings.                                   | no       |
| `splunk` > [`otel_to_hec_fields`][otel_to_hec_fields]      | Configures mapping of OpenTelemetry to HEC Fields.                             | no       |
| `splunk` > [`telemetry`][telemetry]                        | Configures the exporters telemetry.                                            | no       |
| [`client`][client]                                         | Configures the HTTP client used to send data to Splunk HEC.                    | yes      |
| [`debug_metrics`][debug_metrics]                           | Configures the metrics that this component generates to monitor its state.     | no       |
| [`otel_attrs_to_hec_metadata`][otel_attrs_to_hec_metadata] | Configures mapping of resource attributes to HEC metadata fields.              | no       |
| [`sending_queue`][sending_queue]                           | Configures batching of data before sending.                                    | no       |
| `sending_queue` > [`batch`][batch]                         | Configures batching requests based on a timeout and a minimum number of items. | no       |
| [`retry_on_failure`][retry_on_failure]                     | Configures retry mechanism for failed requests.                                | no       |

The > symbol indicates deeper levels of nesting.
For example, `splunk` > `batcher` refers to a `batcher` block defined inside a `splunk` block.

[splunk]: #splunk
[otel_to_hec_fields]: #otel_to_hec_fields
[telemetry]: #telemetry
[heartbeat]: #heartbeat
[batcher]: #batcher
[client]: #client
[otel_attrs_to_hec_metadata]: #otel_attrs_to_hec_metadata
[retry_on_failure]: #retry_on_failure
[sending_queue]: #sending_queue
[batch]: #batch
[debug_metrics]: #debug_metrics

### `splunk`

{{< badge text="Required" >}}

The `splunk` block configures Splunk HEC specific settings.

The following arguments are supported:

| Name                         | Type     | Description                                                                                                            | Default                        | Required |
|------------------------------|----------|------------------------------------------------------------------------------------------------------------------------|--------------------------------|----------|
| `token`                      | `secret` | Splunk HEC Token.                                                                                                      |                                | yes      |
| `disable_compression`        | `bool`   | Disable Gzip compression.                                                                                              | `false`                        | no       |
| `export_raw`                 | `bool`   | Send only the logs body when targeting HEC raw endpoint.                                                               | `false`                        | no       |
| `health_check_enabled`       | `bool`   | Used to verify Splunk HEC health on exporter startup.                                                                  | `true`                         | no       |
| `health_path`                | `string` | Path for the health API.                                                                                               | `"/services/collector/health"` | no       |
| `index`                      | `string` | Splunk index name.                                                                                                     | `""`                           | no       |
| `log_data_enabled`           | `bool`   | Enable sending logs from the exporter. One of `log_data_enabled` or `profiling_data_enabled` must be `true`.           | `true`                         | no       |
| `max_content_length_logs`    | `uint`   | Maximum log payload size in bytes. Must be less than 838860800 (~800MB).                                               | `2097152`                      | no       |
| `max_content_length_metrics` | `uint`   | Maximum metric payload size in bytes. Must be less than 838860800 (~800MB).                                            | `2097152`                      | no       |
| `max_content_length_traces`  | `uint`   | Maximum trace payload size in bytes. Must be less than 838860800 (~800MB).                                             | `2097152`                      | no       |
| `max_event_size`             | `uint`   | Maximum event payload size in bytes. Must be less than 838860800 (~800MB).                                             | `5242880`                      | no       |
| `profiling_data_enabled`     | `bool`   | Enable sending profiling data from the exporter. One of `log_data_enabled` or `profiling_data_enabled` must be `true`. | `true`                         | no       |
| `sourcetype`                 | `string` | [Splunk source type](https://docs.splunk.com/Splexicon:Sourcetype).                                                    | `""`                           | no       |
| `source`                     | `string` | [Splunk source](https://docs.splunk.com/Splexicon:Source).                                                             | `""`                           | no       |
| `splunk_app_name`            | `string` | Used to track telemetry for Splunk Apps by name.                                                                       | `"Alloy"`                      | no       |
| `splunk_app_version`         | `string` | Used to track telemetry by App version.                                                                                | `""`                           | no       |
| `use_multi_metrics_format`   | `bool`   | Use multi-metrics format to save space during ingestion.                                                               | `false`                        | no       |

#### `batcher`

{{< admonition type="warning" >}}
The `batcher` block is deprecated and will be removed in a future release. Use the `sending_queue` > `batch` block instead.
{{< /admonition >}}

| Name            | Type       | Description                                                                                                                                                                                               | Default   | Required |
|-----------------|------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------|----------|
| `enabled`       | `bool`     | Whether to not enqueue batches before sending to the consumerSender.                                                                                                                                      | `false`   | no       |
| `flush_timeout` | `duration` | The time after which a batch will be sent regardless of its size.                                                                                                                                         | `"200ms"` | no       |
| `max_size`      | `uint`     | The maximum size of a batch. If the batch exceeds this value, it's broken up into smaller batches. Must be greater than or equal to `min_size`. Set this value to zero to disable the maximum size limit. | `0`       | no       |
| `min_size`      | `uint`     | The minimum size of a batch.                                                                                                                                                                              | `8192`    | no       |
| `sizer`         | `string`   | The unit of measure for the batch size. Must be one of `items`, `bytes`, or `requests`.                                                                                                                   | `"items"` | no       |

#### `heartbeat`

| Name       | Type       | Description                                           | Default | Required |
|------------|------------|-------------------------------------------------------|---------|----------|
| `interval` | `duration` | Time interval for the heartbeat interval, in seconds. | `"0s"`  | no       |
| `startup`  | `bool`     | Send heartbeat events on exporter startup.            | `false` | no       |

#### `otel_to_hec_fields`

| Name              | Type     | Description                                         | Default | Required |
|-------------------|----------|-----------------------------------------------------|---------|----------|
| `severity_number` | `string` | Maps severity number field to a specific HEC field. | `""`    | no       |
| `severity_text`   | `string` | Maps severity text field to a specific HEC field.   | `""`    | no       |

#### `telemetry`

| Name                     | Type          | Description                                            | Default | Required |
|--------------------------|---------------|--------------------------------------------------------|---------|----------|
| `enabled`                | `bool`        | Enable telemetry inside the exporter.                  | `false` | no       |
| `override_metrics_names` | `map(string)` | Override metrics for internal metrics in the exporter. |         | no       |

### `client`

{{< badge text="Required" >}}

The `client` block configures the HTTP client used by the component.

The following arguments are supported:

| Name                      | Type       | Description                                                                                     | Default | Required |
|---------------------------|------------|-------------------------------------------------------------------------------------------------|---------|----------|
| `endpoint`                | `string`   | The Splunk HEC endpoint to use.                                                                 |         | yes      |
| `disable_keep_alives`     | `bool`     | Disable HTTP keep-alive.                                                                        | `false` | no       |
| `idle_conn_timeout`       | `duration` | Time to wait before an idle connection closes itself.                                           | `"45s"` | no       |
| `insecure_skip_verify`    | `bool`     | Ignores insecure server TLS certificates.                                                       | `false` | no       |
| `max_conns_per_host`      | `int`      | Limits the total (dialing,active, and idle) number of connections per host. Zero means no limit | `0`     | no       |
| `max_idle_conns_per_host` | `int`      | Limits the number of idle HTTP connections the host can keep open.                              | `0`     | no       |
| `max_idle_conns`          | `int`      | Limits the number of idle HTTP connections the client can keep open.                            | `100`   | no       |
| `read_buffer_size`        | `int`      | Size of the read buffer the HTTP client uses for reading server responses.                      | `0`     | no       |
| `timeout`                 | `duration` | Time to wait before marking a request as failed.                                                | `"15s"` | no       |
| `write_buffer_size`       | `int`      | Size of the write buffer the HTTP client uses for writing requests.                             | `0`     | no       |

### `otel_attrs_to_hec_metadata`

The `otel_attrs_to_hec_metadata` block configures the mapping of resource attributes to HEC metadata fields.
This allows resource attributes like `host.name` to be mapped to the top-level `host` field in the HEC JSON payload.

The following arguments are supported:

| Name         | Type     | Description                                                                                                        | Default                   | Required |
|--------------|----------|--------------------------------------------------------------------------------------------------------------------|---------------------------|----------|
| `host`       | `string` | Specifies the mapping of a specific unified model attribute value to the standard host field of a HEC event.       | `"host.name"`             | no       |
| `index`      | `string` | Specifies the mapping of a specific unified model attribute value to the standard index field of a HEC event.      | `"com.splunk.index"`      | no       |
| `source`     | `string` | Specifies the mapping of a specific unified model attribute value to the standard source field of a HEC event.     | `"com.splunk.source"`     | no       |
| `sourcetype` | `string` | Specifies the mapping of a specific unified model attribute value to the standard sourcetype field of a HEC event. | `"com.splunk.sourcetype"` | no       |

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `sending_queue`

The `sending_queue` block configures an in-memory buffer of batches before data is sent to the HTTP server.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.
By default, the `batch` block is not used.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `retry_on_failure`

The `retry_on_failure` block configures how failed requests to Splunk HEC are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                 |
|---------|--------------------|-------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.splunkhec` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.splunkhec` doesn't expose any component-specific debug information.

## Example

### OpenTelemetry Receiver

This example forwards metrics, logs, and traces send to the `otelcol.receiver.otlp.default` receiver to the Splunk HEC exporter.

```alloy
otelcol.receiver.otlp "default" {
    grpc {
        endpoint = "localhost:4317"
    }

    http {
        endpoint               = "localhost:4318"
        compression_algorithms = ["zlib"]
    }

    output {
        metrics = [otelcol.exporter.splunkhec.default.input]
        logs    = [otelcol.exporter.splunkhec.default.input]
        traces  = [otelcol.exporter.splunkhec.default.input]
    }
}

otelcol.exporter.splunkhec "default" {
    client {
        endpoint                = "https://splunkhec.domain.com:8088/services/collector"
        timeout                 = "10s"
        max_idle_conns          = 200
        max_idle_conns_per_host = 200
        idle_conn_timeout       = "10s"
    }

    splunk {
        token              = "SPLUNK_TOKEN"
        source             = "otel"
        sourcetype         = "otel"
        index              = "metrics"
        splunk_app_name    = "OpenTelemetry-Collector Splunk Exporter"
        splunk_app_version = "v0.0.1"

        otel_to_hec_fields {
            severity_text   = "otel.log.severity.text"
            severity_number = "otel.log.severity.number"
        }

        heartbeat {
            interval = "30s"
        }

        telemetry {
            enabled                = true
            override_metrics_names = {
                otelcol_exporter_splunkhec_heartbeats_failed = "app_heartbeats_failed_total",
                otelcol_exporter_splunkhec_heartbeats_sent   = "app_heartbeats_success_total",
            }
            extra_attributes = {
                custom_key   = "custom_value",
                dataset_name = "SplunkCloudBeaverStack",
            }
        }
    }
}
```

### Forward Prometheus Metrics

This example forwards Prometheus metrics from {{< param "PRODUCT_NAME" >}} through a receiver for conversion to OpenTelemetry format before finally sending them to Splunk HEC.

```alloy
prometheus.exporter.self "default" {
}

prometheus.scrape "metamonitoring" {
  targets    = prometheus.exporter.self.default.targets
  forward_to = [otelcol.receiver.prometheus.default.receiver]
}

otelcol.receiver.prometheus "default" {
  output {
    metrics = [otelcol.exporter.splunkhec.default.input]
  }
}


otelcol.exporter.splunkhec "default" {
    splunk {
        token = "SPLUNK_TOKEN"
    }
    client {
        endpoint = "http://splunkhec.domain.com:8088"
    }
}
```

### Forward Loki logs

This example watches for files ending with `.log` in the path `/var/log`, tails these logs with Loki and forwards the logs to the configured Splunk HEC endpoint.
The Splunk HEC exporter component is setup to send an heartbeat every 5 seconds.

```alloy
local.file_match "local_files" {
    path_targets = [{"__path__" = "/var/log/*.log"}]
    sync_period  = "5s"
}

otelcol.receiver.loki "default" {
    output {
        logs = [otelcol.processor.resourcedetection.default.input]
    }
}

otelcol.processor.resourcedetection "default" {
    detectors = ["system"]

    system {
        hostname_sources = ["os", "dns"]
        resource_attributes {
            host.name {
                enabled = true
            }
        }
    }

    output {
        logs = [otelcol.exporter.splunkhec.default.input]
    }
}

loki.source.file "log_scrape" {
    targets       = local.file_match.local_files.targets
    forward_to    = [otelcol.receiver.loki.default.receiver]
    tail_from_end = false
}

otelcol.exporter.splunkhec "default" {
    retry_on_failure {
        enabled = false
    }

    client {
        endpoint                = "http://splunkhec.domain.com:8088"
        timeout                 = "5s"
        max_idle_conns          = 200
        max_idle_conns_per_host = 200
        idle_conn_timeout       = "10s"
        write_buffer_size       = 8000
    }

    sending_queue {
        enabled = false
    }

    // Configure mapping of resource attributes to HEC metadata fields
    otel_attrs_to_hec_metadata {
        host       = "host.name"           // Maps host.name resource attribute to top-level host field
        source     = "com.splunk.source"   // Maps com.splunk.source attribute to source field
        sourcetype = "com.splunk.sourcetype" // Maps com.splunk.sourcetype attribute to sourcetype field
        index      = "com.splunk.index"    // Maps com.splunk.index attribute to index field
    }

    splunk {
        token            = "SPLUNK_TOKEN"
        source           = "otel"
        sourcetype       = "otel"
        index            = "devnull"
        log_data_enabled = true

        heartbeat {
            interval = "5s"
        }

        batcher {
            flush_timeout = "200ms"
        }

        telemetry {
            enabled                = true
            override_metrics_names = {
                otelcol_exporter_splunkhec_heartbeats_failed = "app_heartbeats_failed_total",
                otelcol_exporter_splunkhec_heartbeats_sent   = "app_heartbeats_success_total",
            }
            extra_attributes = {
                host   = "myhost",
                dataset_name = "SplunkCloudBeaverStack",
            }
        }
    }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.splunkhec` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
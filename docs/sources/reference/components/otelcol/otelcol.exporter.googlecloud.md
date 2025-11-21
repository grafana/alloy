---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.googlecloud/
description: Learn about otelcol.exporter.googlecloud
labels:
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: otelcol.exporter.googlecloud
---

# `otelcol.exporter.googlecloud`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.googlecloud` accepts metrics, traces, and logs from other `otelcol` components and sends it to Google Cloud.

{{< admonition type="note" >}}
`otelcol.exporter.googlecloud` is a wrapper over the upstream OpenTelemetry Collector [`googlecloud`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.

[`googlecloud`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/googlecloudexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.googlecloud` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.googlecloud "<LABEL>" {
}
```

### Authenticating

Refer to the original [Google Cloud Exporter][] document.

[Google Cloud Exporter]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/googlecloudexporter/README.md

## Arguments

You can use the following arguments with `otelcol.exporter.googlecloud`:

| Name                        | Type     | Description                                                                                                                                                                                                                        | Default                                         | Required |
|-----------------------------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------|----------|
| `project`                   | `string` | GCP project identifier.                                                                                                                                                                                                            | Fetch from credentials                          | no       |
| `destination_project_quota` | `bool`   | Counts quota for traces and metrics against the project to which the data is sent as opposed to the project associated with the Collector's service account. For example, when setting `project_id` or using multi-project export. | `false`                                         | no       |
| `user_agent`                | `string` | Override the user agent string sent on requests to Cloud Monitoring (currently only applies to metrics). Specify `{{version}}` to include the application version number.                                                          | `"opentelemetry-collector-contrib {{version}}"` | no       |

## Blocks

You can use the following blocks with `otelcol.exporter.googlecloud`:

| Block                                             | Description                                                                    | Required |
|---------------------------------------------------|--------------------------------------------------------------------------------|----------|
| [`debug_metrics`][debug_metrics]                  | Configures the metrics that this component generates to monitor its state.     | no       |
| [`impersonate`][impersonate]                      | Configuration for service account impersonation                                | no       |
| [`log`][log]                                      | Configuration for sending logs to Cloud Logging.                               | no       |
| [`metric`][metric]                                | Configuration for sending metrics to Cloud Monitoring.                         | no       |
| [`metric` > `experimental_wal`][experimental_wal] | Configuration for write ahead log for time series requests.                    | no       |
| [`sending_queue`][sending_queue]                  | Configures batching of data before sending.                                    | no       |
| `sending_queue` > [`batch`][batch]                | Configures batching requests based on a timeout and a minimum number of items. | no       |
| [`trace`][trace]                                  | Configuration for sending traces to Cloud Trace.                               | no       |

The > symbol indicates deeper levels of nesting.
For example, `metric` > `experimental_wal` refers to a `experimental_wal` block defined inside a `metric` block.

[debug_metrics]: #debug_metrics
[impersonate]: #impersonate
[log]: #log
[metric]: #metric
[experimental_wal]: #experimental_wal
[sending_queue]: #sending_queue
[batch]: #batch
[trace]: #trace

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `impersonate`

The following arguments are supported:

| Name               | Type           | Description                                                                                                                                                                                  | Default | Required |
|--------------------|----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `target_principal` | `string`       | TargetPrincipal is the email address of the service account to impersonate.                                                                                                                  |         | yes      |
| `delegates`        | `list(string)` | Delegates are the service account email addresses in a delegation chain. Each service account must be granted roles/iam.serviceAccountTokenCreator on the next service account in the chain. | `[]`    | no       |
| `subject`          | `string`       | Subject is the sub field of a JWT. This field should only be set if you need to impersonate as a user. This feature is useful when using domain wide delegation.                             | `""`    | no       |

### `log`

The following arguments are supported:

| Name                          | Type           | Description                                                                                                                                                                                                | Default                      | Required |
|-------------------------------|----------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------|----------|
| `compression`                 | `string`       | Compression format for Log gRPC requests. Supported values: [`gzip`].                                                                                                                                      | `""` (no compression)        | no       |
| `default_log_name`            | `string`       | Defines a default name for log entries. If left unset, and a log entry doesn't have the `gcp.log_name` attribute set, the exporter returns an error processing that entry.                                 | `""`                         | no       |
| `endpoint`                    | `string`       | Endpoint where log data is sent.                                                                                                                                                                           | `logging.googleapis.com:443` | no       |
| `error_reporting_type`        | `bool`         | Enables automatically parsing error logs to a JSON payload containing the type value for GCP Error Reporting.                                                                                              | `false`                      | no       |
| `grpc_pool_size`              | `number`       | Sets the size of the connection pool in the GCP client.                                                                                                                                                    | `1`                          | no       |
| `resource_filters`            | `list(object)` | If provided, resource attributes matching any filter is included in log labels. Can be defined by `prefix`, `regex`, or `prefix` AND `regex`. Each object must contain one of `prefix` or `regex` or both. | `[]`                         | no       |
| `resource_filters` > `prefix` | `string`       | Match resource keys by prefix.                                                                                                                                                                             | `""`                         | no       |
| `resource_filters` > `regex`  | `string`       | Match resource keys by regular expression.                                                                                                                                                                 | `""`                         | no       |
| `service_resource_labels`     | `bool`         | If true, the exporter copies the OTel service.name, service.namespace, and service.instance.id resource attributes into the GCM timeseries metric labels.                                                  | `true`                       | no       |
| `use_insecure`                | `bool`         | If true, disables gRPC client transport security. Only has effect if Endpoint isn't `""`.                                                                                                                  | `false`                      | no       |

### `metric`

The following arguments are supported:

| Name                                   | Type           | Description                                                                                                                                                                                                                 | Default                                                          | Required |
|----------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------|----------|
| `compression`                          | `string`       | Compression format for Metrics gRPC requests. Supported values: [`gzip`].                                                                                                                                                   | `""` (no compression)                                            | no       |
| `create_metric_descriptor_buffer_size` | `number`       | Buffer size for the  channel which asynchronously calls CreateMetricDescriptor.                                                                                                                                             | `10`                                                             | no       |
| `create_service_timeseries`            | `bool`         | If true, this sends all timeseries using `CreateServiceTimeSeries`. Implicitly, this sets `skip_create_descriptor` to true.                                                                                                 | `false`                                                          | no       |
| `cumulative_normalization`             | `bool`         | If true, normalizes cumulative metrics without start times or with explicit reset points by subtracting subsequent points from the initial point. Since it caches starting points, it may result in increased memory usage. | `true`                                                           | no       |
| `endpoint`                             | `string`       | Endpoint where metric data is sent to.                                                                                                                                                                                      | `"monitoring.googleapis.com:443"`                                | no       |
| `grpc_pool_size`                       | `number`       | Sets the size of the connection pool in the GCP client.                                                                                                                                                                     | `1`                                                              | no       |
| `instrumentation_library_labels`       | `bool`         | If true, set the `instrumentation_source` and `instrumentation_version` labels.                                                                                                                                             | `true`                                                           | no       |
| `known_domains`                        | `list(string)` | If a metric belongs to one of these domains it doesn't get a prefix.                                                                                                                                                        | `["googleapis.com", "kubernetes.io", "istio.io", "knative.dev"]` | no       |
| `prefix`                               | `string`       | The prefix to add to metrics.                                                                                                                                                                                               | `"workload.googleapis.com"`                                      | no       |
| `resource_filters.prefix`              | `string`       | Match resource keys by prefix.                                                                                                                                                                                              | `""`                                                             | no       |
| `resource_filters.regex`               | `string`       | Match resource keys by regular expression.                                                                                                                                                                                  | `""`                                                             | no       |
| `resource_filters`                     | `list(object)` | If provided, resource attributes matching any filter is included in metric labels. Can be defined by `prefix`, `regex`, or `prefix` AND `regex`. Each object must contain one of `prefix` or `regex` or both.               | `[]`                                                             | no       |
| `service_resource_labels`              | `bool`         | If true, the exporter copies the OTel service.name, service.namespace, and service.instance.id resource attributes into the GCM timeseries metric labels.                                                                   | `true`                                                           | no       |
| `skip_create_descriptor`               | `bool`         | If set to true, don't send metric descriptors to GCM.                                                                                                                                                                       | `false`                                                          | no       |
| `sum_of_squared_deviation`             | `bool`         | If true, enables calculation of an estimated sum of squared deviation. It's an estimate, and isn't exact.                                                                                                                   | `false`                                                          | no       |
| `use_insecure`                         | `bool`         | If true, disables gRPC client transport security. Only has effect if Endpoint isn't `""`.                                                                                                                                   | `false`                                                          | no       |

### `experimental_wal`

The following arguments are supported:

| Name          | Type     | Description                                                                              | Default | Required |
|---------------|----------|------------------------------------------------------------------------------------------|---------|----------|
| `directory`   | `string` | Path to local directory for the WAL file.                                                | `"./"`  | yes      |
| `max_backoff` | `string` | Max duration to retry requests on network errors (`UNAVAILABLE` or `DEADLINE_EXCEEDED`). | `"1h"`  | no       |

### `sending_queue`

The `sending_queue` block configures queueing and batching for the exporter.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `batch`

The `batch` block configures batching requests based on a timeout and a minimum number of items.

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `trace`

| Name                                 | Type           | Description                                                                                                                                                                      | Default                           | Required |
|--------------------------------------|----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------|----------|
| `attribute_mappings`                 | `list(object)` | Determines how to map from OpenTelemetry attribute keys to Google Cloud Trace keys. By default, it changes HTTP and service keys so that they appear more prominently in the UI. | `[]`                              | no       |
| `attribute_mappings` > `key`         | `string`       | The OpenTelemetry attribute key.                                                                                                                                                 | `""`                              | no       |
| `attribute_mappings` > `replacement` | `string`       | The attribute sent to Google Cloud Trace.                                                                                                                                        | `""`                              | no       |
| `endpoint`                           | `string`       | Endpoint where trace data is sent.                                                                                                                                               | `"cloudtrace.googleapis.com:443"` | no       |
| `grpc_pool_size`                     | `int`          | Sets the size of the connection pool in the GCP client. Defaults to a single connection.                                                                                         | `1`                               | no       |
| `use_insecure`                       | `bool`         | If true, disables gRPC client transport security. Only has effect if Endpoint isn't `""`.                                                                                        | `false`                           | no       |

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                 |
|---------|--------------------|-------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.googlecloud` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.googlecloud` doesn't expose any component-specific debug information.

## Example

This example scrapes logs from local files through a receiver for conversion to OpenTelemetry format before finally sending them to Cloud Logging.

This configuration includes the recommended `memory_limiter` and `batch` plugins, which avoid high latency for reporting telemetry, and ensure that the collector itself will stay stable (not run out of memory) by dropping telemetry if needed.

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
    metrics = [otelcol.exporter.googlecloud.default.input]
    logs = [otelcol.exporter.googlecloud.default.input]
    traces = [otelcol.exporter.googlecloud.default.input]
  }
}

otelcol.exporter.googlecloud "default" {
  project = "my-gcp-project"
  log {
    default_log_name = "opentelemetry.io/collector-exported-log"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.googlecloud` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.googlecloud/
description: Learn about otelcol.exporter.googlecloud
aliases:
  - ../otelcol.exporter.googlecloud/ # /docs/alloy/latest/reference/components/otelcol.exporter.googlecloud/
title: otelcol.exporter.googlecloud
---


<span class="badge docs-labels__stage docs-labels__item">Community</span>

# otelcol.exporter.googlecloud

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.googlecloud` accepts metrics, traces, and logs from other `otelcol` components and sends it to Google Cloud.

{{< admonition type="note" >}}
`otelcol.exporter.googlecloud` is a wrapper over the upstream OpenTelemetry Collector `googlecloud` exporter from the `otelcol-contrib`  distribution.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

You can specify multiple `otelcol.exporter.googlecloud` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.googlecloud "LABEL" {
}
```

### Authenticating

Refer to the original [Google Cloud Exporter][] document.

[Google Cloud Exporter]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/googlecloudexporter/README.md

## Arguments

If there are any discrepancies between the argument descriptions here and those in the original [Google Cloud Exporter][] documentation,
the original documentation takes precedence.
Argument descriptions are excerpted directly from the original documentation.

The following arguments are supported:

| Name                        | Type     | Description                                                                                                                                                                                                                       | Default                                       | Required |
|-----------------------------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------|----------|
| `project`                   | `string` | GCP project identifier.                                                                                                                                                                                                           | Fetch from credentials                        | no       |
| `destination_project_quota` | `bool`   | Counts quota for traces and metrics against the project to which the data is sent (as opposed to the project associated with the Collector's service account. For example, when setting project_id or using multi-project export. | `false`                                       | no       |
| `user_agent`                | `string` | Override the user agent string sent on requests to Cloud Monitoring (currently only applies to metrics). Specify `{{version}}` to include the application version number.                                                         | `opentelemetry-collector-contrib {{version}}` | no       |

## Blocks

The following blocks are supported inside the definition of `otelcol.exporter.googlecloud`:

| Hierarchy     | Block             | Description                                                                | Required |
|---------------|-------------------|----------------------------------------------------------------------------|----------|
| impersonate   | [impersonate][]   | Configuration for service account impersonation                            | no       |
| metric        | [metric][]        | Configuration for sending metrics to Cloud Monitoring.                     | no       |
| trace         | [trace][]         | Configuration for sending traces to Cloud Trace.                           | no       |
| log           | [log][]           | Configuration for sending metrics to Cloud Logging.                        | no       |
| sending_queue | [sending_queue][] | Configures batching of data before sending.                                | no       |
| debug_metrics | [debug_metrics][] | Configures the metrics that this component generates to monitor its state. | no       |

[impersonate]: #impersonate-block
[metric]: #metric-block
[trace]: #trace-block
[log]: #log-block
[sending_queue]: #sending_queue-block
[debug_metrics]: #debug_metrics-block

### impersonate block

The following arguments are supported:

| Name               | Type           | Description                                                                                                                                                                                  | Default | Required |
|--------------------|----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `target_principal` | `string`       | TargetPrincipal is the email address of the service account to impersonate.                                                                                                                  |         | yes      |
| `subject`          | `string`       | Subject is the sub field of a JWT. This field should only be set if you wish to impersonate as a user. This feature is useful when using domain wide delegation.                             | `""`    | no       |
| `delegates`        | `list(string)` | Delegates are the service account email addresses in a delegation chain. Each service account must be granted roles/iam.serviceAccountTokenCreator on the next service account in the chain. | `[]`    | no       |

### metric block

The following arguments are supported:

| Name                                   | Type           | Description                                                                                                                                                                                                                                           | Default                                                  | Required |
|----------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------|----------|
| `prefix`                               | `string`       | The prefix to add to metrics.                                                                                                                                                                                                                         | `workload.googleapis.com`                                | no       |
| `endpoint`                             | `string`       | Endpoint where metric data is going to be sent to.                                                                                                                                                                                                    | `monitoring.googleapis.com:443`                          | no       |
| `compression`                          | `string`       | Compression format for Metrics gRPC requests. Supported values: [`gzip`].                                                                                                                                                                             | `""` (no compression)                                    | no       |
| `grpc_pool_size`                       | `number`       | Sets the size of the connection pool in the GCP client.                                                                                                                                                                                               | `1`                                                      | no       |
| `use_insecure`                         | `bool`         | If true, disables gRPC client transport security. Only has effect if Endpoint is not "".                                                                                                                                                              | `false`                                                  | no       |
| `known_domains`                        | `list(string)` | If a metric belongs to one of these domains it does not get a prefix.                                                                                                                                                                                 | `[googleapis.com, kubernetes.io, istio.io, knative.dev]` | no       |
| `skip_create_descriptor`               | `bool`         | If set to true, do not send metric descriptors to GCM.                                                                                                                                                                                                | `false`                                                  | no       |
| `instrumentation_library_labels`       | `bool`         | If true, set the instrumentation_source and instrumentation_version labels.                                                                                                                                                                           | `true`                                                   | no       |
| `create_service_timeseries`            | `bool`         | If true, this will send all timeseries using `CreateServiceTimeSeries`. Implicitly, this sets `skip_create_descriptor` to true.                                                                                                                       | `false`                                                  | no       |
| `create_metric_descriptor_buffer_size` | `number`       | Buffer size for the  channel which asynchronously calls CreateMetricDescriptor.                                                                                                                                                                       | `10`                                                     | no       |
| `service_resource_labels`              | `bool`         | If true, the exporter will copy OTel's service.name, service.namespace, and service.instance.id resource attributes into the GCM timeseries metric labels.                                                                                            | `true`                                                   | no       |
| `resource_filters`                     | `list(object)` | If provided, resource attributes matching any filter will be included in metric labels. Can be defined by `prefix`, `regex`, or `prefix` AND `regex`. Each object must contain one of `prefix` or `regex` or both.                                    | `[]`                                                     | no       |
| `resource_filters.prefix`              | `string`       | Match resource keys by prefix.                                                                                                                                                                                                                        | `""`                                                     | no       |
| `resource_filters.regex`               | `string`       | Match resource keys by regex.                                                                                                                                                                                                                         | `""`                                                     | no       |
| `cumulative_normalization`             | `bool`         | If true, normalizes cumulative metrics without start times or with explicit reset points by subtracting subsequent points from the initial point. It is enabled by default. Since it caches starting points, it may result in increased memory usage. | `true`                                                   | no       |
| `sum_of_squared_deviation`             | `bool`         | If true, enables calculation of an estimated sum of squared deviation. It is an estimate, and is not exact.                                                                                                                                           | `false`                                                  | no       |
| `experimental_wal`                     | `list(object)` | If provided, enables use of a write ahead log for time series requests. Each object must contain `directory`                                                                                                                                          | `[]`                                                     | no       |
| `experimental_wal` > `directory`       | `string`       | Path to local directory for WAL file.                                                                                                                                                                                                                 | `./`                                                     | no       |
| `experimental_wal` > `max_backoff`     | `string`       | Max duration to retry requests on network errors (`UNAVAILABLE` or `DEADLINE_EXCEEDED`).                                                                                                                                                              | `1h`                                                     | no       |

The `>` symbol indicates deeper levels of nesting. For example, `experimental_wal > directory` refers to a directory argument defined inside a experimental_wal block.

### trace block

| Name                                 | Type           | Description                                                                                                                                                                                         | Default                         | Required |
|--------------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------|----------|
| `endpoint`                           | `string`       | Endpoint where trace data is going to be sent to.                                                                                                                                                   | `cloudtrace.googleapis.com:443` | no       |
| `grpc_pool_size`                     | `int`          | Sets the size of the connection pool in the GCP client. Defaults to a single connection.                                                                                                            | `1`                             | no       |
| `use_insecure`                       | `bool`         | If true, disables gRPC client transport security. Only has effect if Endpoint is not "".                                                                                                            | `false`                         | no       |
| `attribute_mappings`                 | `list(object)` | AttributeMappings determines how to map from OpenTelemetry attribute keys to Google Cloud Trace keys.  By default, it changes http and service keys so that they appear more prominently in the UI. | `[]`                            | no       |
| `attribute_mappings` > `key`         | `string`       | Key is the OpenTelemetry attribute key                                                                                                                                                              | `""`                            | no       |
| `attribute_mappings` > `replacement` | `string`       | Replacement is the attribute sent to Google Cloud Trace                                                                                                                                             | `""`                            | no       |

### log block

The following arguments are supported:

| Name                          | Type           | Description                                                                                                                                                                                                     | Default                      | Required |
|-------------------------------|----------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------|----------|
| `endpoint`                    | `string`       | Endpoint where log data is going to be sent to.                                                                                                                                                                 | `logging.googleapis.com:443` | no       |
| `compression`                 | `string`       | Compression format for Log gRPC requests. Supported values: [`gzip`].                                                                                                                                           | `""` (no compression)        | no       |
| `grpc_pool_size`              | `number`       | Sets the size of the connection pool in the GCP client.                                                                                                                                                         | `1`                          | no       |
| `use_insecure`                | `bool`         | If true, disables gRPC client transport security. Only has effect if Endpoint is not "".                                                                                                                        | `false`                      | no       |
| `default_log_name`            | `string`       | Defines a default name for log entries. If left unset, and a log entry does not have the `gcp.log_name` attribute set, the exporter will return an error processing that entry.                                 | `""`                         | no       |
| `resource_filters`            | `list(object)` | If provided, resource attributes matching any filter will be included in log labels. Can be defined by `prefix`, `regex`, or `prefix` AND `regex`. Each object must contain one of `prefix` or `regex` or both. | `[]`                         | no       |
| `resource_filters` > `prefix` | `string`       | Match resource keys by prefix.                                                                                                                                                                                  | `""`                         | no       |
| `resource_filters` > `regex`  | `string`       | Match resource keys by regex.                                                                                                                                                                                   | `""`                         | no       |
| `service_resource_labels`     | `bool`         | If true, the exporter will copy OTel's service.name, service.namespace, and service.instance.id resource attributes into the GCM timeseries metric labels.                                                      | `true`                       | no       |
| `error_reporting_type`        | `bool`         | ErrorReportingType enables automatically parsing error logs to a json payload containing the type value for GCP Error Reporting.                                                                                | `false`                      | no       |

### sending_queue block

The `sending_queue` block configures an in-memory buffer of batches before data is sent to the HTTP server.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                 |
|---------|--------------------|-------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.googlecloud` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.exporter.googlecloud` does not expose any component-specific debug
information.

## Example

### Logging

This example scrapes logs from local files through a receiver for conversion to OpenTelemetry format before finally sending them to Cloud Logging.

Note that this configuration includes the recommended `memory_limiter` and `batch` plugins, which avoid high latency for reporting telemetry,
and ensure that the collector itself will stay stable (not run out of memory) by dropping telemetry if needed.

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

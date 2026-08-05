---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.azureblob/
description: Learn about otelcol.exporter.azureblob
labels:
  stage: experimental
  products:
    - oss
  tags:
    - text: Community
      tooltip: This component is developed, maintained, and supported by the Alloy user community.
title: otelcol.exporter.azureblob
---

# `otelcol.exporter.azureblob`

{{< docs/shared lookup="stability/community.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.azureblob` receives telemetry data from other `otelcol` components and writes it to Azure Blob Storage.

{{< admonition type="note" >}}
`otelcol.exporter.azureblob` is a wrapper over the upstream OpenTelemetry Collector [`azureblob`][] exporter.
Bug reports or feature requests will be redirected to the upstream repository if necessary.

[`azureblob`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/exporter/azureblobexporter
{{< /admonition >}}

You can specify multiple `otelcol.exporter.azureblob` components by giving them different labels.

## Usage

```alloy
otelcol.exporter.azureblob "<LABEL>" {
  url = "https://<ACCOUNT>.blob.core.windows.net"

  auth {
    type = "system_managed_identity"
  }

  container {
    logs    = "otel-logs"
    metrics = "otel-metrics"
    traces  = "otel-traces"
  }
}
```

## Arguments

You can use the following arguments with `otelcol.exporter.azureblob`:

| Name      | Type       | Description                                      | Default  | Required |
| --------- | ---------- | ------------------------------------------------ | -------- | -------- |
| `format`  | `string`   | Format used to encode telemetry data.            | `"json"` | no       |
| `timeout` | `duration` | Timeout for each attempt to send data to Azure.  | `"30s"`  | no       |
| `url`     | `string`   | Azure Storage account URL.                       |          | no       |

The `format` argument must be either `json` or `proto`.
The `url` argument is required unless `auth.type` is `connection_string`.

## Blocks

You can use the following blocks with `otelcol.exporter.azureblob`:

{{< docs/alloy-config >}}

| Block                                  | Description                                                                    | Required |
| -------------------------------------- | ------------------------------------------------------------------------------ | -------- |
| [`auth`][auth]                         | Configures Azure authentication.                                               | yes      |
| [`container`][container]               | Configures the container name for metrics, logs, and traces.                   | no       |
| [`blob_name_format`][blob_name_format] | Configures the blob name format.                                               | no       |
| [`append_blob`][append_blob]           | Enables append blob mode and separator.                                        | no       |
| [`debug_metrics`][debug_metrics]       | Configures the metrics that this component generates to monitor its state.     | no       |
| [`retry_on_failure`][retry_on_failure] | Configures retry backoff for failed requests.                                  | no       |
| [`sending_queue`][sending_queue]       | Configures batching of data before sending.                                    | no       |
| `sending_queue` > [`batch`][batch]     | Configures batching requests based on a timeout and a minimum number of items. | no       |

[auth]: #auth
[container]: #container
[append_blob]: #append_blob
[retry_on_failure]: #retry_on_failure
[debug_metrics]: #debug_metrics
[sending_queue]: #sending_queue
[batch]: #batch
[blob_name_format]: #blob_name_format

The `>` symbol indicates deeper levels of nesting.
For example, `sending_queue` > `batch` refers to a `batch` block defined inside a `sending_queue` block.

{{< /docs/alloy-config >}}

### `auth`

{{< badge text="Required" >}}

The `auth` block configures authentication with Azure Blob Storage.

The following arguments are supported:

| Name                   | Type     | Description                                                                                                                             | Default               | Required |
| ---------------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------- | --------------------- | -------- |
| `type`                 | `string` | Authentication type: `connection_string`, `service_principal`, `system_managed_identity`, `user_managed_identity`, `workload_identity`. | `"connection_string"` | no       |
| `tenant_id`            | `string` | Microsoft Entra tenant ID for service principal.                                                                                        |                       | no       |
| `client_id`            | `string` | Microsoft Entra client ID for service principal or user-managed identity.                                                               |                       | no       |
| `client_secret`        | `string` | Microsoft Entra client secret for service principal.                                                                                    |                       | no       |
| `connection_string`    | `string` | Azure Storage connection string.                                                                                                        |                       | no       |
| `federated_token_file` | `string` | Path to federated token for workload identity.                                                                                          |                       | no       |

### `container`

The `container` block configures the container names for each telemetry signal.

The following arguments are supported:

| Name      | Type     | Description                 | Default     | Required |
| --------- | -------- | --------------------------- | ----------- | -------- |
| `metrics` | `string` | Container name for metrics. | `"metrics"` | no       |
| `logs`    | `string` | Container name for logs.    | `"logs"`    | no       |
| `traces`  | `string` | Container name for traces.  | `"traces"`  | no       |

### `blob_name_format`

The `blob_name_format` block configures blob names.

The following arguments are supported:

| Name                          | Type                | Description                                       | Default                              | Required |
| ----------------------------- | ------------------- | ------------------------------------------------- | ------------------------------------ | -------- |
| `metrics_format`              | `string`            | Blob name format for metrics.                     | `"2006/01/02/metrics_15_04_05.json"` | no       |
| `logs_format`                 | `string`            | Blob name format for logs.                        | `"2006/01/02/logs_15_04_05.json"`    | no       |
| `traces_format`               | `string`            | Blob name format for traces.                      | `"2006/01/02/traces_15_04_05.json"`  | no       |
| `serial_num_enabled`          | `boolean`           | Whether to append a random serial number.         | `true`                               | no       |
| `serial_num_range`            | `int`               | Upper limit for the random serial suffix.         | `10000`                              | no       |
| `serial_num_before_extension` | `boolean`           | Place serial before file extension.               | `false`                              | no       |
| `timezone`                    | `string`            | Timezone used when formatting blob names.         |                                      | no       |
| `template_enabled`            | `boolean`           | Enable Go template expansion in blob name format. | `false`                              | no       |
| `time_parser_enabled`         | `boolean`           | Enable time parsing in blob name format.          | `true`                               | no       |
| `time_parser_ranges`          | `list(string)`      | Time ranges used by the time parser.              |                                      | no       |
| `params`                      | `map(string)`       | Additional template parameters.                   | `{}`                                 | no       |

### `append_blob`

The `append_blob` block configures append blob behavior.

The following arguments are supported:

| Name        | Type      | Description                            | Default | Required |
| ----------- | --------- | -------------------------------------- | ------- | -------- |
| `enabled`   | `boolean` | Enable append blob mode.               | `false` | no       |
| `separator` | `string`  | Separator used when appending content. | `"\n"`  | no       |

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `retry_on_failure`

The `retry_on_failure` block configures retry backoff for failed requests.

The following arguments are supported:

| Name                   | Type       | Description                              | Default | Required |
| ---------------------- | ---------- | ---------------------------------------- | ------- | -------- |
| `enabled`              | `boolean`  | Enable retries.                          | `true`  | no       |
| `initial_interval`     | `duration` | Initial backoff interval.                | `5s`    | no       |
| `randomization_factor` | `float`    | Randomization factor for backoff jitter. | `0.5`   | no       |
| `multiplier`           | `float`    | Exponential backoff multiplier.          | `1.5`   | no       |
| `max_interval`         | `duration` | Maximum backoff interval.                | `30s`   | no       |
| `max_elapsed_time`     | `duration` | Maximum total retry time.                | `5m`    | no       |

### `sending_queue`

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `batch`

{{< docs/shared lookup="reference/components/otelcol-queue-batch-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name    | Type               | Description                                                      |
|---------|--------------------|------------------------------------------------------------------|
| `input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to. |

`input` accepts `otelcol.Consumer` data for any telemetry signal.

## Example

This example sends logs to Azure Blob Storage using a system-managed identity:

```alloy
otelcol.receiver.loki "default" {
  output {
    logs = [otelcol.exporter.azureblob.logs.input]
  }
}

otelcol.exporter.azureblob "logs" {
  url = "https://myaccount.blob.core.windows.net"

  auth {
    type = "system_managed_identity"
  }

  container {
    logs = "logs"
  }
}
```

## Component health

`otelcol.exporter.azureblob` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.exporter.azureblob` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.exporter.azureblob` doesn't expose any component-specific debug metrics.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.azureblob` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

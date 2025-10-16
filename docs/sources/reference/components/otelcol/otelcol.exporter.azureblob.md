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
  blob_uploader {
    url = "https://<ACCOUNT>.blob.core.windows.net"
    container {
      logs    = "otel-logs"
      metrics = "otel-metrics"
      traces  = "otel-traces"
    }
  }
}
```

## Arguments

The `otelcol.exporter.azureblob` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `otelcol.exporter.azureblob`:

| Block                                                    | Description                                                                    | Required |
| -------------------------------------------------------- | ------------------------------------------------------------------------------ | -------- |
| [`blob_uploader`][blob_uploader]                         | Configures destination and blob naming.                                        | yes      |
| `blob_uploader` > [`auth`][auth]                         | Configures Azure authentication.                                               | no       |
| `blob_uploader` > [`container`][container]               | Configures the container name for logs, metrics, and traces.                   | no       |
| `blob_uploader` > [`blob_name_format`][blob_name_format] | Configures the blob name format.                                               | no       |
| [`append_blob`][append_blob]                             | Enables append blob mode and separator.                                        | no       |
| [`debug_metrics`][debug_metrics]                         | Configures the metrics that this component generates to monitor its state.     | no       |
| [`encodings`][encodings]                                 | Overrides marshaler via extension encodings per-signal.                        | no       |
| [`marshaler`][marshaler]                                 | Marshaler used to produce output data.                                         | no       |
| [`retry_on_failure`][retry_on_failure]                   | Configures retry backoff for failed requests.                                  | no       |
| [`sending_queue`][sending_queue]                         | Configures batching of data before sending.                                    | no       |
| `sending_queue` > [`batch`][batch]                       | Configures batching requests based on a timeout and a minimum number of items. | no       |

The > symbol indicates deeper levels of nesting.
For example, `blob_uploader` > `auth` refers to an `auth` block defined inside a `blob_uploader` block.

[blob_uploader]: #blob_uploader
[auth]: #auth
[container]: #container
[marshaler]: #marshaler
[append_blob]: #append_blob
[encodings]: #encodings
[retry_on_failure]: #retry_on_failure
[debug_metrics]: #debug_metrics
[sending_queue]: #sending_queue
[batch]: #batch

### `blob_uploader`

{{< badge text="Required" >}}

The `blob_uploader` block configures the Azure Blob Storage destination and naming.

The following arguments are supported:

| Name  | Type     | Description                | Default | Required |
| ----- | -------- | -------------------------- | ------- | -------- |
| `url` | `string` | Azure Storage account URL. |         | no       |

### `auth`

| Name                   | Type     | Description                                                                                                                             | Default               | Required |
| ---------------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------- | --------------------- | -------- |
| `type`                 | `string` | Authentication type: `connection_string`, `service_principal`, `system_managed_identity`, `user_managed_identity`, `workload_identity`. | `"connection_string"` | no       |
| `tenant_id`            | `string` | Azure AD tenant ID for service principal.                                                                                               |                       | no       |
| `client_id`            | `string` | Azure AD client ID for service principal or user-managed identity.                                                                      |                       | no       |
| `client_secret`        | `string` | Azure AD client secret for service principal.                                                                                           |                       | no       |
| `connection_string`    | `string` | Azure Storage connection string.                                                                                                        |                       | no       |
| `federated_token_file` | `string` | Path to federated token for workload identity.                                                                                          |                       | no       |

### `container`

| Name      | Type     | Description                 | Default     | Required |
| --------- | -------- | --------------------------- | ----------- | -------- |
| `logs`    | `string` | Container name for logs.    | `"logs"`    | no       |
| `metrics` | `string` | Container name for metrics. | `"metrics"` | no       |
| `traces`  | `string` | Container name for traces.  | `"traces"`  | no       |

### `blob_name_format`

| Name                          | Type                | Description                               | Default                              | Required |
| ----------------------------- | ------------------- | ----------------------------------------- | ------------------------------------ | -------- |
| `metrics_format`              | `string`            | Blob name format for metrics.             | `"2006/01/02/metrics_15_04_05.json"` | no       |
| `logs_format`                 | `string`            | Blob name format for logs.                | `"2006/01/02/logs_15_04_05.json"`    | no       |
| `traces_format`               | `string`            | Blob name format for traces.              | `"2006/01/02/traces_15_04_05.json"`  | no       |
| `serial_num_range`            | `int`               | Upper limit for the random serial suffix. | `10000`                              | no       |
| `serial_num_before_extension` | `boolean`           | Place serial before file extension.       | `false`                              | no       |
| `params`                      | `map[string]string` | Additional template parameters.           |                                      | no       |

### `append_blob`

| Name        | Type      | Description                            | Default |
| ----------- | --------- | -------------------------------------- | ------- |
| `enabled`   | `boolean` | Enable append blob mode.               | `false` |
| `separator` | `string`  | Separator used when appending content. | `"\n"`  |

### `debug_metrics`

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `encodings`

Overrides `marshaler` via extension encodings per signal. Values should be OTel component IDs like `text_encoding` or `text_encoding/custom`.

| Name      | Type     | Description                                 | Required |
|-----------|----------|---------------------------------------------|----------|
| `logs`    | `string` | Encoding extension component ID for logs.   | no       |
| `metrics` | `string` | Encoding extension component ID for metrics.| no       |
| `traces`  | `string` | Encoding extension component ID for traces. | no       |

### `marshaler`

Marshaler determines the format of data written to Azure Blob Storage. 

| Name   | Type     | Description                            | Default       | Required |
|--------|----------|----------------------------------------|---------------|----------|
| `type` | `string` | Marshaler used to produce output data. | `"otlp_json"` | no       |

Supported values for `type`:

* `otlp_json`: The OpenTelemetry protocol format represented as JSON.
* `otlp_proto`: The OpenTelemetry protocol format represented as Protocol Buffers.

### `retry_on_failure`

Configures retry backoff for failed requests.

| Name                   | Type       | Description                              | Default |
| ---------------------- | ---------- | ---------------------------------------- | ------- |
| `enabled`              | `boolean`  | Enable retries.                          | `true`  |
| `initial_interval`     | `duration` | Initial backoff interval.                | `5s`    |
| `randomization_factor` | `float`    | Randomization factor for backoff jitter. | `0.5`   |
| `multiplier`           | `float`    | Exponential backoff multiplier.          | `1.5`   |
| `max_interval`         | `duration` | Maximum backoff interval.                | `30s`   |
| `max_elapsed_time`     | `duration` | Maximum total retry time.                | `5m`    |

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

```alloy
otelcol.receiver.loki "default" {
  output {
    logs = [otelcol.exporter.azureblob.logs.input]
  }
}

otelcol.exporter.azureblob "logs" {
  blob_uploader {
    url = "https://myaccount.blob.core.windows.net"
    container {
      logs = "logs"
    }
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



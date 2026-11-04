---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.encoding.azure/
description: Learn about otelcol.encoding.azure
labels:
  stage: experimental
  products:
    - oss
title: otelcol.encoding.azure
---

# `otelcol.encoding.azure`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.encoding.azure` decodes logs, metrics, and traces exported by [Azure Diagnostic Settings][] or [Azure Data Collection Rules][] into OpenTelemetry data.
It exposes a handler that compatible `otelcol` components can use to unmarshal Azure telemetry.
`otelcol.encoding.azure` only decodes data. It doesn't marshal OpenTelemetry data into Azure formats.

`otelcol.encoding.azure` detects the following input formats for every signal:

- **Records object:** `{"records": [{...}, {...}]}`, which Azure Event Hubs exports and some Azure Blob Storage exports use.
- **JSON array:** `[{...}, {...}]`, which some Azure Blob Storage exports use.
- **Newline-delimited JSON:** one JSON record per line, which some Azure Blob Storage exports use.

{{< admonition type="note" >}}
`otelcol.encoding.azure` is a wrapper over the upstream OpenTelemetry Collector [`azureencodingextension`][] extension.
Grafana Labs redirects bug reports or feature requests to the upstream repository when necessary.

[`azureencodingextension`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/encoding/azureencodingextension
{{< /admonition >}}

You can specify multiple `otelcol.encoding.azure` components by giving them different labels.

[Azure Diagnostic Settings]: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/diagnostic-settings
[Azure Data Collection Rules]: https://learn.microsoft.com/en-us/azure/azure-monitor/data-collection/data-collection-rule-overview

## Usage

```alloy
otelcol.encoding.azure "<LABEL>" {
}
```

## Arguments

The `otelcol.encoding.azure` component doesn't support any arguments. You can configure this component with blocks.

## Blocks

You can use the following blocks with `otelcol.encoding.azure`:

| Block                | Description                                 | Required |
| -------------------- | ------------------------------------------- | -------- |
| [`logs`][logs]       | Configures decoding of Azure resource logs. | no       |
| [`metrics`][metrics] | Configures decoding of Azure metrics.       | no       |
| [`traces`][traces]   | Configures decoding of Azure traces.        | no       |

All blocks are optional.
If you don't specify a block, `otelcol.encoding.azure` decodes that signal with the default settings.

[logs]: #logs
[metrics]: #metrics
[traces]: #traces

### `logs`

The `logs` block configures how `otelcol.encoding.azure` decodes [Azure resource logs][] that you export with Azure Diagnostic Settings or Data Collection Rules.

The following arguments are supported:

| Name                 | Type           | Description                                                    | Default                | Required |
| -------------------- | -------------- | -------------------------------------------------------------- | ---------------------- | -------- |
| `exclude_categories` | `list(string)` | Azure resource log categories to drop.                         | `[]`                   | no       |
| `include_categories` | `list(string)` | Azure resource log categories to decode.                       | `[]`                   | no       |
| `time_formats`       | `list(string)` | Additional time layouts to use when parsing record timestamps. | See the following list | no       |

The `include_categories` argument limits decoding to the listed Azure resource log categories, for example, `FrontDoorAccessLog`.
`otelcol.encoding.azure` silently drops records from other categories.
When `include_categories` is empty, `otelcol.encoding.azure` decodes all categories.

The `exclude_categories` argument drops records from the listed categories.
If a category appears in both `include_categories` and `exclude_categories`, `otelcol.encoding.azure` drops it.

The `time_formats` argument is a list of [Go time layouts][] to use when parsing record timestamps.
`otelcol.encoding.azure` always tries ISO 8601 first, and then tries each layout in the order you list them.
Setting `time_formats` replaces the default list, which contains the following layouts:

- `"01/02/2006 15:04:05"`
- `"1/2/2006 3:04:05.000 PM -07:00"`
- `"1/2/2006 3:04:05 PM -07:00"`

`otelcol.encoding.azure` maps the fields of a subset of Azure resource log categories to OpenTelemetry semantic conventions.
For records from other categories, `otelcol.encoding.azure` maps the common fields and copies the remaining fields to log attributes.
Refer to the upstream [Azure resource logs transformation rules][logs-rules] for the supported categories and field mappings.

`otelcol.encoding.azure` sets the `encoding.format` scope attribute on each decoded log to identify the Azure log family.
You can use this attribute to filter and route logs by Azure service:

| Azure log family    | `encoding.format` value     | Description                                                         |
| ------------------- | --------------------------- | ------------------------------------------------------------------- |
| Activity            | `azure.activity`            | Activity logs, for example, administrative, alert, and policy.      |
| Application Gateway | `azure.application_gateway` | Application Gateway access, performance, and firewall logs.         |
| App Service         | `azure.appservice`          | App Service logs, for example, app, audit, HTTP, and platform.      |
| CDN                 | `azure.cdn`                 | Azure CDN access logs.                                              |
| Data Factory        | `azure.datafactory`         | Data Factory activity, pipeline, and trigger run logs.              |
| Front Door          | `azure.frontdoor`           | Front Door access, health probe, and web application firewall logs. |
| Function App        | `azure.function_app`        | Azure Functions application logs.                                   |
| Messaging           | `azure.messaging`           | Event Hubs and Service Bus logs.                                    |
| Storage             | `azure.storage`             | Storage read, write, and delete operation logs.                     |
| Generic             | `azure.generic`             | Logs from categories without a dedicated mapping.                   |

Log attributes use the stable OpenTelemetry semantic convention names.

[Azure resource logs]: https://learn.microsoft.com/en-us/azure/azure-monitor/platform/resource-logs
[logs-rules]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/encoding/azureencodingextension/internal/unmarshaler/logs/README.md
[Go time layouts]: https://pkg.go.dev/time#Layout

### `metrics`

The `metrics` block configures how `otelcol.encoding.azure` decodes Azure platform metrics.
`otelcol.encoding.azure` supports metrics that you export with Azure Diagnostic Settings or Data Collection Rules.
Diagnostic Settings exports don't include [multi-dimensional metrics][].

The following arguments are supported:

| Name           | Type           | Description                                                    | Default | Required |
| -------------- | -------------- | -------------------------------------------------------------- | ------- | -------- |
| `aggregations` | `list(string)` | Azure metric aggregations to emit as OpenTelemetry metrics.    | `[]`    | no       |
| `time_formats` | `list(string)` | Additional time layouts to use when parsing record timestamps. | `[]`    | no       |

`otelcol.encoding.azure` converts each aggregation of an Azure metric into a separate OpenTelemetry gauge.
Refer to the upstream [Azure metrics transformation rules][metrics-rules] for metric names and attributes.

The `aggregations` argument limits the emitted metrics to the listed aggregations.
The supported values are `average`, `count`, `maximum`, `minimum`, and `total`.
When `aggregations` is empty, `otelcol.encoding.azure` emits all supported aggregations.
`otelcol.encoding.azure` reports an invalid configuration if `aggregations` contains any other value.

The `time_formats` argument is a list of [Go time layouts][] to use when parsing record timestamps.
`otelcol.encoding.azure` always tries ISO 8601 first, and then tries each layout in the order you list them.
When `time_formats` is empty, `otelcol.encoding.azure` only parses ISO 8601 timestamps.

[multi-dimensional metrics]: https://learn.microsoft.com/en-us/azure/azure-monitor/platform/diagnostic-settings#metrics-limitations
[metrics-rules]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/encoding/azureencodingextension/internal/unmarshaler/metrics/README.md

### `traces`

The `traces` block configures how `otelcol.encoding.azure` decodes traces from Application Insights in Azure Monitor.
Export the data with Azure Diagnostic Settings or Data Collection Rules, and use Application Insights as the source instead of a specific Azure resource.

`otelcol.encoding.azure` decodes the following Application Insights data as traces:

- [Availability results](https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appavailabilityresults)
- [Dependencies](https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appdependencies)
- [Requests](https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/apprequests)

Refer to the upstream [Azure traces transformation rules][traces-rules] for field mappings.

The following arguments are supported:

| Name           | Type           | Description                                                    | Default | Required |
| -------------- | -------------- | -------------------------------------------------------------- | ------- | -------- |
| `time_formats` | `list(string)` | Additional time layouts to use when parsing record timestamps. | `[]`    | no       |

The `time_formats` argument is a list of [Go time layouts][] to use when parsing record timestamps.
`otelcol.encoding.azure` always tries ISO 8601 first, and then tries each layout in the order you list them.
When `time_formats` is empty, `otelcol.encoding.azure` only parses ISO 8601 timestamps.

[traces-rules]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/extension/encoding/azureencodingextension/internal/unmarshaler/traces/README.md

## Exported fields

The following fields are exported and can be referenced by other components:

| Name      | Type                       | Description                                                                                       |
| --------- | -------------------------- | ------------------------------------------------------------------------------------------------- |
| `handler` | `capsule(otelcol.Handler)` | A handler that compatible `otelcol` components can use to decode Azure logs, metrics, and traces. |

## Component health

`otelcol.encoding.azure` is only reported as unhealthy if given an invalid configuration.

## Debug information

`otelcol.encoding.azure` doesn't expose any component-specific debug information.

## Debug metrics

`otelcol.encoding.azure` doesn't expose any component-specific debug metrics.

## Example

This example decodes Azure resource logs that Azure Diagnostic Settings stream to an Azure Event Hubs namespace.
`otelcol.receiver.kafka` reads the event hub through the Kafka-compatible endpoint of the namespace and uses `otelcol.encoding.azure` to decode each message.
Event Hubs doesn't fully support leader epochs, so the example sets `use_leader_epoch` to `false`.
The receiver forwards the decoded logs to `otelcol.exporter.debug`:

```alloy
otelcol.encoding.azure "default" { }

otelcol.receiver.kafka "default" {
	brokers          = ["<NAMESPACE>.servicebus.windows.net:9093"]
	protocol_version = "2.0.0"
	use_leader_epoch = false

	authentication {
		sasl {
			username  = "$ConnectionString"
			password  = sys.env("EVENTHUBS_CONNECTION_STRING")
			mechanism = "PLAIN"
		}
	}

	tls { }

	logs {
		topics             = ["<EVENT_HUB>"]
		encoding_extension = otelcol.encoding.azure.default.handler
	}

	output {
		logs = [otelcol.exporter.debug.default.input]
	}
}

otelcol.exporter.debug "default" {}
```

Replace the following:

- _`<NAMESPACE>`_: The name of your Event Hubs namespace.
- _`<EVENT_HUB>`_: The name of the event hub that receives the resource logs.

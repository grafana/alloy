---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.azure/
aliases:
  - ../prometheus.exporter.azure/ # /docs/alloy/latest/reference/components/prometheus.exporter.azure/
description: Learn about prometheus.exporter.azure
title: prometheus.exporter.azure
---

# prometheus.exporter.azure

The `prometheus.exporter.azure` component embeds [`azure-metrics-exporter`](https://github.com/webdevops/azure-metrics-exporter) to collect metrics from [Azure Monitor](https://azure.microsoft.com/en-us/products/monitor).

The exporter supports all metrics defined by Azure Monitor. You can find the complete list of available metrics in the [Azure Monitor documentation](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-supported).
Metrics for this integration are exposed with the template `azure_{type}_{metric}_{aggregation}_{unit}` by default. As an example,
the Egress metric for BlobService would be exported as `azure_microsoft_storage_storageaccounts_blobservices_egress_total_bytes`.

The exporter offers the following two options for gathering metrics.

1. (Default) Use an [Azure Resource Graph](https://azure.microsoft.com/en-us/get-started/azure-portal/resource-graph/#overview) query to identify resources for gathering metrics.
   1. This query will make one API call per resource identified.
   1. Subscriptions with a reasonable amount of resources can hit the [12000 requests per hour rate limit](https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/request-limits-and-throttling#subscription-and-tenant-limits) Azure enforces.
1. Set the regions to gather metrics from and get metrics for all resources across those regions.
   1. This option will make one API call per subscription, dramatically reducing the number of API calls.
   1. This approach does not work with all resource types, and Azure does not document which resource types do or do not work.
   1. A resource type that is not supported produces errors that look like `Resource type: microsoft.containerservice/managedclusters not enabled for Cross Resource metrics`.
   1. If you encounter one of these errors you must use the default Azure Resource Graph based option to gather metrics.

## Authentication

{{< param "PRODUCT_NAME" >}} must be running in an environment with access to Azure. The exporter uses the Azure SDK for go and supports [authentication](https://learn.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication?tabs=bash#2-authenticate-with-azure).

The account used by {{< param "PRODUCT_NAME" >}} needs:

- When using an Azure Resource Graph query, [read access to the resources that will be queried by Resource Graph](https://learn.microsoft.com/en-us/azure/governance/resource-graph/overview#permissions-in-azure-resource-graph).
<!-- vale Grafana.GoogleSpacing = NO -->
- Permissions to call the [Microsoft.Insights Metrics API](https://learn.microsoft.com/en-us/rest/api/monitor/metrics/list) which should be the `Microsoft.Insights/Metrics/Read` permission.
<!-- vale Grafana.GoogleSpacing = YES -->

## Usage

```alloy
prometheus.exporter.azure LABEL {
        subscriptions = [
                SUB_ID_1,
                SUB_ID_2,
                ...
        ]

        resource_type = RESOURCE_TYPE

        metrics = [
                METRIC_1,
                METRIC_2,
                ...
        ]
}
```

## Arguments

You can use the following arguments to configure the exporter's behavior.
Omitted fields take their default values.

| Name                          | Type           | Description                                                                                          | Default                                                                       | Required |
|-------------------------------|----------------|------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------|----------|
| `subscriptions`               | `list(string)` | List of subscriptions to scrape metrics from.                                                        |                                                                               | yes      |
| `resource_type`               | `string`       | The Azure Resource Type to scrape metrics for.                                                       |                                                                               | yes      |
| `metrics`                     | `list(string)` | The metrics to scrape from resources.                                                                |                                                                               | yes      |
| `resource_graph_query_filter` | `string`       | The [Kusto query][] filter to apply when searching for resources. Can't be used if `regions` is set. |                                                                               | no       |
| `regions`                     | `list(string)` | The list of regions for gathering metrics and enables gathering metrics for all resources in the subscription. Can't be used if `resource_graph_query_filter` is set. |              | no       |
| `metric_aggregations`         | `list(string)` | Aggregations to apply for the metrics produced.                                                      |                                                                               | no       |
| `timespan`                    | `string`       | [ISO8601 Duration][] over which the metrics are being queried.                                       | `"PT1M"` (1 minute)                                                           | no       |
| `included_dimensions`         | `list(string)` | List of dimensions to include on the final metrics.                                                  |                                                                               | no       |
| `included_resource_tags`      | `list(string)` | List of resource tags to include on the final metrics.                                               | `["owner"]`                                                                   | no       |
| `metric_namespace`            | `string`       | Namespace for `resource_type` which have multiple levels of metrics.                                 |                                                                               | no       |
| `azure_cloud_environment`     | `string`       | Name of the cloud environment to connect to.                                                         | `"azurecloud"`                                                                | no       |
| `metric_name_template`        | `string`       | Metric template used to expose the metrics.                                                          | `"azure_{type}_{metric}_{aggregation}_{unit}"`                                | no       |
| `metric_help_template`        | `string`       | Description of the metric.                                                                           | `"Azure metric {metric} for {type} with aggregation {aggregation} as {unit}"` | no       |
| `validate_dimensions`         | `bool`         | Enable dimension validation in the azure sdk                                                         | `false`                                                                       | no       |

The list of available `resource_type` values and their corresponding `metrics` can be found in [Azure Monitor essentials][].

The list of available `regions` to your subscription can be found by running the azure CLI command `az account list-locations --query '[].name'`.

The `resource_graph_query_filter` can be embedded into a template query of the form `Resources | where type =~ "<resource_type>" <resource_graph_query_filter> | project id, tags`.

Valid values for `metric_aggregations` are `minimum`, `maximum`, `average`, `total`, and `count`.
If no aggregation is specified, the value is retrieved from the metric.
<!-- vale Grafana.GoogleSpacing = NO -->
For example, the aggregation value of the metric `Availability` in [Microsoft.ClassicStorage/storageAccounts](https://learn.microsoft.com/en-us/azure/azure-monitor/reference/supported-metrics/microsoft-classicstorage-storageaccounts-metrics) is `average`.
<!-- vale Grafana.GoogleSpacing = YES -->
Every metric has its own set of dimensions.
<!-- vale Grafana.GoogleSpacing = NO -->
For example, the dimensions for the metric `Availability` in [Microsoft.ClassicStorage/storageAccounts](https://learn.microsoft.com/en-us/azure/azure-monitor/reference/supported-metrics/microsoft-classicstorage-storageaccounts-metrics) are `GeoType`, `ApiName`, and `Authentication`.
<!-- vale Grafana.GoogleSpacing = YES -->
If a single dimension is requested, it will have the name `dimension`.
If multiple dimensions are requested, they will have the name `dimension<dimension_name>`.

Tags in `included_resource_tags` will be added as labels with the name `tag_<tag_name>`.

Valid values for `azure_cloud_environment` are `azurecloud`, `azurechinacloud`, `azuregovernmentcloud` and `azurepprivatecloud`.

`validate_dimensions` is disabled by default to reduce the number of Azure exporter instances requires when a `resource_type` has metrics with varying dimensions.
When `validate_dimensions` is enabled you will need one exporter instance per metric + dimension combination which is more tedious to maintain.

[Kusto query]: https://learn.microsoft.com/en-us/azure/data-explorer/kusto/query/
[Azure Monitor essentials]: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-supported
[ISO8601 Duration]: https://en.wikipedia.org/wiki/ISO_8601#Durations

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.azure` is only reported as unhealthy if given
an invalid configuration. In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.azure` does not expose any component-specific
debug information.

## Debug metrics

`prometheus.exporter.azure` does not expose any component-specific
debug metrics.

## Examples

```alloy
prometheus.exporter.azure "example" {
	subscriptions    = SUBSCRIPTIONS
	resource_type    = "Microsoft.Storage/storageAccounts"
	regions          = [
	    "westeurope",
	]
	metric_namespace = "Microsoft.Storage/storageAccounts/blobServices"
	metrics          = [
		"Availability",
		"BlobCapacity",
		"BlobCount",
		"ContainerCount",
		"Egress",
		"IndexCapacity",
		"Ingress",
		"SuccessE2ELatency",
		"SuccessServerLatency",
		"Transactions",
	]
	included_dimensions = [
        "ApiName",
        "TransactionType",
	]
	timespan                    = "PT1H"
}

// Configure a prometheus.scrape component to send metrics to.
prometheus.scrape "demo" {
	targets    = prometheus.exporter.azure.example.targets
	forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
	endpoint {
		url = PROMETHEUS_REMOTE_WRITE_URL

		basic_auth {
			username = USERNAME
			password = PASSWORD
		}
	}
}
```

Replace the following:

- `SUBSCRIPTIONS`: The Azure subscription IDs holding the resources you are interested in.
- `PROMETHEUS_REMOTE_WRITE_URL`: The URL of the Prometheus remote_write-compatible server to send metrics to.
- `USERNAME`: The username to use for authentication to the remote_write API.
- `PASSWORD`: The password to use for authentication to the remote_write API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.azure` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.azure/
aliases:
  - ../prometheus.exporter.azure/ # /docs/alloy/latest/reference/components/prometheus.exporter.azure/
description: Learn about prometheus.exporter.azure
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-09-14
title: prometheus.exporter.azure
---

# `prometheus.exporter.azure`

The `prometheus.exporter.azure` component embeds [`azure-metrics-exporter`][] to collect metrics from [Azure Monitor][].

The exporter supports all metrics defined by Azure Monitor.
You can find the complete list of available metrics in the [Azure Monitor documentation][].
Metrics for this integration use the template `azure_{type}_{metric}_{aggregation}_{unit}` by default.
As an example, the exporter exports the Egress metric for BlobService as `azure_microsoft_storage_storageaccounts_blobservices_egress_total_bytes`.

The exporter offers the following two options for gathering metrics.

1. Use an [Azure Resource Graph][] query to identify resources for gathering metrics. This is the default option.
   1. This query makes one API call per resource identified.
   1. Subscriptions with a reasonable amount of resources can hit the [12000 requests per hour rate limit][] Azure enforces.
1. Set the regions to gather metrics from and get metrics for all resources across those regions.
   1. This option makes one API call per subscription, dramatically reducing the number of API calls.
   1. This approach doesn't work with all resource types, and Azure doesn't document which resource types do or don't work.
   1. A resource type that's not supported produces errors that look like `Resource type: microsoft.containerservice/managedclusters not enabled for Cross Resource metrics`.
   1. If you encounter one of these errors you must use the default Azure Resource Graph based option to gather metrics.

[Azure Resource Graph]: https://azure.microsoft.com/en-us/get-started/azure-portal/resource-graph/#overview
[12000 requests per hour rate limit]: https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/request-limits-and-throttling#subscription-and-tenant-limits
[`azure-metrics-exporter`]: https://github.com/webdevops/azure-metrics-exporter
[Azure Monitor]: https://azure.microsoft.com/en-us/products/monitor
[Azure Monitor documentation]: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-supported

## Authentication

{{< param "PRODUCT_NAME" >}} must be running in an environment with access to Azure.
The exporter uses the Azure SDK for Go and supports [authentication][].

The account used by {{< param "PRODUCT_NAME" >}} needs:

- When using an Azure Resource Graph query, [read access to the resources that Resource Graph queries][].
- Permissions to call the [`Microsoft.Insights` Metrics API][Microsoft.Insights Metrics API] which should be the `Microsoft.Insights/Metrics/Read` permission.

[authentication]: https://learn.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication?tabs=bash#2-authenticate-with-azure
[read access to the resources that Resource Graph queries]: https://learn.microsoft.com/en-us/azure/governance/resource-graph/overview#permissions-in-azure-resource-graph
[Microsoft.Insights Metrics API]: https://learn.microsoft.com/en-us/rest/api/monitor/metrics/list

## Usage

```alloy
prometheus.exporter.azure "<LABEL>" {
        subscriptions = [
                <SUB_ID_1>,
                <SUB_ID_2>,
                ...
        ]

        resource_type = "<RESOURCE_TYPE>"

        metrics = [
                "<METRIC_1>",
                "<METRIC_2>",
                ...
        ]
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.azure`:

| Name                          | Type           | Description                                                                                                        | Default                                                                       | Required |
| ----------------------------- | -------------- | ------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------- | -------- |
| `metrics`                     | `list(string)` | The metrics to scrape from resources.                                                                              |                                                                               | yes      |
| `resource_type`               | `string`       | The Azure Resource Type to scrape metrics for.                                                                     |                                                                               | yes      |
| `subscriptions`               | `list(string)` | List of subscriptions to scrape metrics from.                                                                      |                                                                               | yes      |
| `azure_cloud_environment`     | `string`       | Name of the cloud environment to connect to.                                                                       | `"azurecloud"`                                                                | no       |
| `included_dimensions`         | `list(string)` | List of dimensions to include on the final metrics.                                                                |                                                                               | no       |
| `included_resource_tags`      | `list(string)` | List of resource tags to include on the final metrics.                                                             | `["owner"]`                                                                   | no       |
| `interval`                    | `string`       | [ISO8601 Duration][] used to generate individual data points in Azure Monitor. Must be no greater than `timespan`. | `"PT1M"`                                                                      | no       |
| `metric_aggregations`         | `list(string)` | Aggregations to apply for the metrics produced.                                                                    |                                                                               | no       |
| `metric_help_template`        | `string`       | Description of the metric.                                                                                         | `"Azure metric {metric} for {type} with aggregation {aggregation} as {unit}"` | no       |
| `metric_name_template`        | `string`       | Metric template used to expose the metrics.                                                                        | `"azure_{type}_{metric}_{aggregation}_{unit}"`                                | no       |
| `metric_namespace`            | `string`       | Namespace for resource types that have multiple levels of metrics.                                                 |                                                                               | no       |
| `regions`                     | `list(string)` | The list of regions to gather metrics from. Mutually exclusive with `resource_graph_query_filter`.                 |                                                                               | no       |
| `resource_graph_query_filter` | `string`       | The [Kusto query][] filter to apply when searching for resources. Mutually exclusive with `regions`.               |                                                                               | no       |
| `timespan`                    | `string`       | [ISO8601 Duration][] over which the exporter queries metrics. Defaults to 5 minutes.                               | `"PT5M"`                                                                      | no       |
| `validate_dimensions`         | `bool`         | Enable dimension validation in the Azure SDK.                                                                      | `false`                                                                       | no       |

The list of available `resource_type` values and their corresponding `metrics` is in [Azure Monitor essentials][].

You can find the list of available `regions` for your subscription by running the Azure CLI command `az account list-locations --query '[].name'`.

You can embed `resource_graph_query_filter` into a template query of the form `Resources | where type =~ "<resource_type>" | <resource_graph_query_filter> | project id, tags`.

Valid values for `metric_aggregations` are `minimum`, `maximum`, `average`, `total`, and `count`.
If you don't specify an aggregation, the exporter retrieves the value from the metric.
For example, the aggregation value of the metric `Availability` in [`Microsoft.ClassicStorage/storageAccounts`][] is `average`.
Every metric has its own set of dimensions.
For example, the dimensions for the metric `Availability` in [`Microsoft.ClassicStorage/storageAccounts`][] are `GeoType`, `ApiName`, and `Authentication`.
If you request a single dimension, it has the name `dimension`.
If you request multiple dimensions, they have the name `dimension<dimension_name>`.

The exporter adds tags in `included_resource_tags` as labels with the name `tag_<tag_name>`.

Valid values for `azure_cloud_environment` are `azurecloud`, `azurechinacloud`, `azuregovernmentcloud` and `azurepprivatecloud`.

`validate_dimensions` defaults to `false` to reduce the number of Azure exporter instances required when a `resource_type` has metrics with varying dimensions.
When you enable `validate_dimensions`, you need one exporter instance per metric + dimension combination, which is more tedious to maintain.

You use `timespan` and `interval` to control how the exporter queries metrics from Azure Monitor. 
The exporter queries metrics over the `timespan` and returns the most recent data point at the specified `interval`. 
If you are having issues with missing metrics, try increasing the `timespan` to a larger value, such as `PT10M` for 10 minutes, or `PT15M` for 15 minutes.

[Kusto query]: https://learn.microsoft.com/en-us/azure/data-explorer/kusto/query/
[Azure Monitor essentials]: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-supported
[ISO8601 Duration]: https://en.wikipedia.org/wiki/ISO_8601#Durations
[`Microsoft.ClassicStorage/storageAccounts`]: https://learn.microsoft.com/en-us/azure/azure-monitor/reference/supported-metrics/microsoft-classicstorage-storageaccounts-metrics

## Blocks

The `prometheus.exporter.azure` component doesn't support any blocks.
You can configure this component with arguments.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.azure` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.azure` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.azure` doesn't expose any component-specific debug metrics.

## Examples

```alloy
prometheus.exporter.azure "example" {
    subscriptions    = ["<SUBSCRIPTION_ID>"]
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
    timespan         = "PT1H"
}

// Configure a prometheus.scrape component to send metrics to.
prometheus.scrape "demo" {
    targets    = prometheus.exporter.azure.example.targets
    forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
    endpoint {
        url = "<PROMETHEUS_REMOTE_WRITE_URL>"

        basic_auth {
            username = "<USERNAME>"
            password = "<PASSWORD>"
        }
    }
}
```

Replace the following:

- _`<SUBSCRIPTION_ID>`_: An Azure subscription ID holding the resources you want to monitor. Add one array element per subscription ID.
- _`<PROMETHEUS_REMOTE_WRITE_URL>`_: The URL of the Prometheus `remote_write` compatible server to send metrics to.
- _`<USERNAME>`_: The username to use for authentication to the `remote_write` API.
- _`<PASSWORD>`_: The password to use for authentication to the `remote_write` API.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.azure` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

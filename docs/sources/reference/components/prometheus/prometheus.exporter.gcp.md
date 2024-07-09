---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.gcp/
aliases:
  - ../prometheus.exporter.gcp/ # /docs/alloy/latest/reference/components/prometheus.exporter.gcp/
description: Learn about prometheus.exporter.gcp
title: prometheus.exporter.gcp
---

# prometheus.exporter.gcp

The `prometheus.exporter.gcp` component embeds [`stackdriver_exporter`](https://github.com/prometheus-community/stackdriver_exporter).
It lets you collect [GCP Cloud Monitoring (formerly stackdriver)](https://cloud.google.com/monitoring/docs), translate them to prometheus-compatible format and remote write.
The component supports all metrics available via the [GCP monitoring API](https://cloud.google.com/monitoring/api/metrics_gcp).

Metric names follow the template `stackdriver_<monitored_resource>_<metric_type_prefix>_<metric_type>`.

The following example shows a load balancing metric:

{{< figure src="/media/docs/alloy/gcp-exporter-config-metric-example.png" alt="Example GCP exporter configuration metric" >}}

The following list shows its attributes: \
monitored_resource = `https_lb_rule`\
metric_type_prefix = `loadbalancing.googleapis.com/`\
metric_type = `https/backend_latencies`

These attributes result in a final metric name of `stackdriver_https_lb_rule_loadbalancing_googleapis_com_https_backend_latencies`

## Authentication

{{< param "PRODUCT_NAME" >}} must be running in an environment with access to the GCP project it's scraping. The exporter
uses the Google Golang Client Library, which offers a variety of ways to [provide credentials](https://developers.google.com/identity/protocols/application-default-credentials). Choose the option that works best for you.

After deciding how {{< param "PRODUCT_NAME" >}} will obtain credentials, ensure the account is set up with the IAM role `roles/monitoring.viewer`.
Since the exporter gathers all of its data from [GCP monitoring APIs](https://cloud.google.com/monitoring/api/v3), this is the only permission needed.

## Usage

```alloy
prometheus.exporter.gcp "pubsub" {
        project_ids = [
                "foo",
                "bar",
        ]

        metrics_prefixes = [
                "pubsub.googleapis.com/snapshot",
                "pubsub.googleapis.com/subscription/num_undelivered_messages",
                "pubsub.googleapis.com/subscription/oldest_unacked_message_age",
        ]
}
```

## Arguments

You can use the following arguments to configure the exporter's behavior.
Omitted fields take their default values.

{{< admonition type="note" >}}
If you are supplying a list of strings for the `extra_filters` argument, any string values within a particular filter string must be enclosed in escaped double quotes.
For example, `loadbalancing.googleapis.com:resource.labels.backend_target_name="sample-value"` must be encoded as `"loadbalancing.googleapis.com:resource.labels.backend_target_name=\"sample-value\""` in the {{< param "PRODUCT_NAME" >}} configuration.
{{< /admonition >}}

| Name                      | Type           | Description                                                                                                                                                                                                                                              | Default | Required |
|---------------------------|----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `project_ids`             | `list(string)` | Configure the GCP Projects to scrape for metrics.                                                                                                                                                                                                        |         | yes      |
| `metrics_prefixes`        | `list(string)` | One or more values from the supported [GCP Metrics](https://cloud.google.com/monitoring/api/metrics_gcp). These can be as targeted or loose as needed.                                                                                                   |         | yes      |
| `extra_filters`           | `list(string)` | Used to further refine the resources you would like to collect metrics from. Any string value within a particular filter string must be enclosed in escaped double-quotes. The structure for these filters is `<targeted_metric_prefix>:<filter_query>`. | `[]`    | no       |
| `request_interval`        | `duration`     | The time range used when querying for metrics.                                                                                                                                                                                                           | `5m`    | no       |
| `ingest_delay`            | `boolean`      | When enabled, this automatically adjusts the time range used when querying for metrics backwards based on the metadata GCP has published for how long the data can take to be ingested.                                                                  | `false` | no       |
| `request_offset`          | `duration`     | When enabled this offsets the time range used when querying for metrics by a set amount.                                                                                                                                                                 | `0s`    | no       |
| `drop_delegated_projects` | `boolean`      | When enabled drops metrics from attached projects and only fetches metrics from the explicitly configured `project_ids`.                                                                                                                                 | `false` | no       |
| `gcp_client_timeout`      | `duration`     | Sets a timeout on the client used to make API calls to GCP. A single scrape can initiate numerous calls to GCP, so be mindful if you choose to override this value.                                                                                      | `15s`   | no       |

For `extra_filters`, the `targeted_metric_prefix` is used to ensure the filter is only applied to the metric_prefix(es) where it makes sense.
It doesn't explicitly have to match a value from `metric_prefixes`, but the `targeted_metric_prefix` must be at least a prefix to one or more `metric_prefixes`.
The `filter_query` is applied to a final metrics API query when querying for metric data.
The final query sent to the metrics API already includes filters for project and metric type.
Each applicable `filter_query` is appended to the query with an AND.
You can read more about the metric API filter options in the [GCP documentation](https://cloud.google.com/monitoring/api/v3/filters).

For `request_interval`, most of the time the default works perfectly fine.
Most documented metrics include a comments of the form `Sampled every X seconds. After sampling, data is not visible for up to Y seconds.`
As long as your `request_interval` is >= `Y` you should have no issues.
Consider using `ingest_delay` if you would like this to be done programmatically or are gathering slower moving metrics.

For `ingest_delay`, you can find the values for this in documented metrics as `After sampling, data is not visible for up to Y seconds.`
Since the GCP ingestion delay is an "at worst", this is off by default to ensure data is gathered as soon as it's available.

## Exported fields

{{< docs/shared lookup="reference/components/exporter-component-exports.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Component health

`prometheus.exporter.gcp` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.gcp` doesn't expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.gcp` doesn't expose any component-specific debug metrics.

## Examples

```alloy
prometheus.exporter.gcp "pubsub_full_config" {
        project_ids = [
                "foo",
                "bar",
        ]

        // Using pubsub metrics (https://cloud.google.com/monitoring/api/metrics_gcp/gcp-pubsub) as an example
        // all metrics.
        //   [
        //     "pubsub.googleapis.com/"
        //   ]
        // all snapshot specific metrics
        //   [
        //     "pubsub.googleapis.com/snapshot"
        //   ]
        // all snapshot specific metrics and a few subscription metrics
        metrics_prefixes = [
                "pubsub.googleapis.com/snapshot",
                "pubsub.googleapis.com/subscription/num_undelivered_messages",
                "pubsub.googleapis.com/subscription/oldest_unacked_message_age",
        ]

        // Given the above metrics_prefixes list, some examples of
        // targeted_metric_prefix option behavior with respect to the filter string
        // format <targeted_metric_prefix>:<filter_query> would be:
        //   pubsub.googleapis.com (apply to all defined prefixes)
        //   pubsub.googleapis.com/snapshot (apply to only snapshot metrics)
        //   pubsub.googleapis.com/subscription (apply to only subscription metrics)
        //   pubsub.googleapis.com/subscription/num_undelivered_messages (apply to only the specific subscription metric)
        extra_filters = [
                "pubsub.googleapis.com/subscription:resource.labels.subscription_id=monitoring.regex.full_match(\"my-subs-prefix.*\")",
        ]

        request_interval        = "5m"
        request_offset          = "0s"
        ingest_delay            = false
        drop_delegated_projects = false
        gcp_client_timeout      = "15s"
}
```

```alloy
prometheus.exporter.gcp "lb_with_filter" {
        project_ids = [
                "foo",
                "bar",
        ]
        metrics_prefixes = [
                "loadbalancing.googleapis.com",
        ]
        extra_filters = [
                "loadbalancing.googleapis.com:resource.labels.backend_target_name=\"sample-value\"",
        ]
}
```

```alloy
prometheus.exporter.gcp "lb_subset_with_filter" {
        project_ids = [
                "foo",
                "bar",
        ]
        metrics_prefixes = [
                "loadbalancing.googleapis.com/https/request_bytes_count",
                "loadbalancing.googleapis.com/https/total_latencies",
        ]
        extra_filters = [
                "loadbalancing.googleapis.com:resource.labels.backend_target_name=\"sample-value\"",
        ]
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.exporter.gcp` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.exporter.gcp/
aliases:
  - ../prometheus.exporter.gcp/ # /docs/alloy/latest/reference/components/prometheus.exporter.gcp/
description: Learn about prometheus.exporter.gcp
labels:
  stage: general-availability
  products:
    - oss
review_date: 2026-10-01
title: prometheus.exporter.gcp
---

# `prometheus.exporter.gcp`

The `prometheus.exporter.gcp` component embeds the [`stackdriver_exporter`][stackdriver-exporter].
You can use this component to collect [GCP Cloud Monitoring][cloud-monitoring] metrics, translate them to Prometheus-compatible format, and remote write.
The component supports all metrics available through the [GCP monitoring API][gcp-metrics].

Metric names follow the template `stackdriver_<monitored_resource>_<metric_type_prefix>_<metric_type>`.

The following example shows a load balancing metric:

{{< figure src="/media/docs/alloy/gcp-exporter-config-metric-example.png" alt="Example GCP exporter configuration metric" >}}

The metric has the following attributes:

- `monitored_resource` = `https_lb_rule`
- `metric_type_prefix` = `loadbalancing.googleapis.com/`
- `metric_type` = `https/backend_latencies`

These attributes result in a final metric name of `stackdriver_https_lb_rule_loadbalancing_googleapis_com_https_backend_latencies`

You can specify multiple `prometheus.exporter.gcp` components by giving them different labels.

[stackdriver-exporter]: https://github.com/prometheus-community/stackdriver_exporter
[cloud-monitoring]: https://cloud.google.com/monitoring/docs

## Authentication

{{< param "PRODUCT_NAME" >}} must be running in an environment with access to the GCP project it's scraping.
The exporter uses the Google Golang Client Library, which offers a variety of ways to [provide credentials][credentials].
Choose the option that works best for you.

After you decide how {{< param "PRODUCT_NAME" >}} obtains credentials, give the account the IAM role `roles/monitoring.viewer`.
Since the exporter gathers all of its data from [GCP monitoring APIs][monitoring-api], this is the only permission needed.

[credentials]: https://developers.google.com/identity/protocols/application-default-credentials
[monitoring-api]: https://cloud.google.com/monitoring/api/v3

## Usage

```alloy
prometheus.exporter.gcp "<LABEL>" {
  project_ids = [
    "<PROJECT_ID_1>",
    "<PROJECT_ID_2>",
  ]

  metrics_prefixes = [
    "pubsub.googleapis.com/snapshot",
    "pubsub.googleapis.com/subscription/num_undelivered_messages",
    "pubsub.googleapis.com/subscription/oldest_unacked_message_age",
  ]
}
```

## Arguments

You can use the following arguments with `prometheus.exporter.gcp`:

| Name                      | Type           | Description                                                                                               | Default | Required |
| ------------------------- | -------------- | --------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `metrics_prefixes`        | `list(string)` | One or more supported [GCP metrics][gcp-metrics]. These can be as targeted or broad as needed.            |         | yes      |
| `project_ids`             | `list(string)` | Configure the GCP Projects to scrape for metrics.                                                         |         | yes      |
| `drop_delegated_projects` | `bool`         | When enabled, drops metrics from projects attached to the configured `project_ids`.                       | `false` | no       |
| `extra_filters`           | `list(string)` | Refine the resources to collect metrics from. The structure is `<targeted_metric_prefix>:<filter_query>`. | `[]`    | no       |
| `gcp_client_timeout`      | `duration`     | Timeout for the client that makes API calls to GCP.                                                       | `"15s"` | no       |
| `ingest_delay`            | `bool`         | When enabled, adjusts the query time range backwards to account for the GCP ingestion delay.              | `false` | no       |
| `request_interval`        | `duration`     | The time range used when querying for metrics.                                                            | `"5m"`  | no       |
| `request_offset`          | `duration`     | Offsets the time range used when querying for metrics by a set amount.                                    | `"0s"`  | no       |

{{< admonition type="note" >}}
If you supply a list of strings for the `extra_filters` argument, you must enclose any string values within a filter string in escaped double quotes.
For example, encode `loadbalancing.googleapis.com:resource.labels.backend_target_name="sample-value"` as `"loadbalancing.googleapis.com:resource.labels.backend_target_name=\"sample-value\""` in the {{< param "PRODUCT_NAME" >}} configuration.
{{< /admonition >}}

For `extra_filters`, the `targeted_metric_prefix` ensures the component applies the filter only to the `metrics_prefixes` values where it makes sense.
It doesn't explicitly have to match a value from `metrics_prefixes`, but the `targeted_metric_prefix` must be at least a prefix to one or more `metrics_prefixes` values.
The component fails to start if an `extra_filters` entry omits the `:` separator, or if its `targeted_metric_prefix` doesn't match any `metrics_prefixes` value.
The component applies the `filter_query` to the final metrics API query when it queries for metric data.
The final query sent to the metrics API already includes filters for project and metric type.
The component appends each applicable `filter_query` to the query with an AND.
You can read more about the metric API filter options in the [GCP documentation][filters].

For `gcp_client_timeout`, be mindful when you override the default.
A single scrape can initiate numerous calls to GCP.

For `request_interval`, most of the time the default works perfectly fine.
Most documented metrics include a comments of the form `Sampled every X seconds. After sampling, data is not visible for up to Y seconds.`
As long as your `request_interval` is greater than or equal to `Y` you should have no issues.
Use `ingest_delay` if you want the component to make this adjustment automatically, or if you gather slower moving metrics.

For `ingest_delay`, you can find the values for this in documented metrics as `After sampling, data is not visible for up to Y seconds.`
Since the GCP ingestion delay is an "at worst", this is off by default so the component gathers data without waiting for that delay to elapse.

The component sets the `instance` label on its exported targets to a hash of its own configuration, because no single argument identifies a GCP scrape.
The label changes if you change any argument.

[gcp-metrics]: https://cloud.google.com/monitoring/api/metrics_gcp
[filters]: https://cloud.google.com/monitoring/api/v3/filters

## Blocks

The `prometheus.exporter.gcp` component doesn't support any blocks. You can configure this component with arguments.

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

The following example sets every argument, and uses comments to show how the granularity of `metrics_prefixes` and `targeted_metric_prefix` affects what the component collects:

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

The following example collects every load balancing metric, and filters the results to a single backend target:

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

The following example collects two specific load balancing metrics with the same backend target filter.
The `targeted_metric_prefix` is shorter than either value in `metrics_prefixes`, which is enough for the filter to apply to both:

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

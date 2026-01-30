---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.scrape/
aliases:
  - ../prometheus.scrape/ # /docs/alloy/latest/reference/components/prometheus.scrape/
description: Learn about prometheus.scrape
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.scrape
---

# `prometheus.scrape`

`prometheus.scrape` configures a Prometheus scraping job for a given set of `targets`.
The scraped metrics are forwarded to the list of receivers passed in `forward_to`.

You can specify multiple `prometheus.scrape` components by giving them different labels.

## Usage

```alloy
prometheus.scrape "<LABEL>" {
  targets    = <TARGET_LIST>
  forward_to = <RECEIVER_LIST>
}
```

## Arguments

The component configures and starts a new scrape job to scrape all the input targets.
The list of arguments that can be used to configure the block is presented below.

The scrape job name defaults to the component's unique identifier.

One of the following can be provided:

* [`authorization`][authorization] block
* [`basic_auth`][basic_auth] block
* [`bearer_token_file`](#arguments) argument
* [`bearer_token`](#arguments) argument
* [`oauth2`][oauth2] block

If conflicting attributes are passed, for example, defining both a `bearer_token` and `bearer_token_file` or configuring both `basic_auth` and `oauth2` at the same time, the component reports an error.

You can use the following arguments with `prometheus.scrape`:

| Name                                 | Type                    | Description                                                                                                              | Default                                                                                          | Required |
|--------------------------------------|-------------------------|--------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------|----------|
| `forward_to`                         | `list(MetricsReceiver)` | List of receivers to send scraped metrics to.                                                                            |                                                                                                  | yes      |
| `targets`                            | `list(map(string))`     | List of targets to scrape.                                                                                               |                                                                                                  | yes      |
| `bearer_token_file`                  | `string`                | File containing a bearer token to authenticate with.                                                                     |                                                                                                  | no       |
| `bearer_token`                       | `secret`                | Bearer token to authenticate with.                                                                                       |                                                                                                  | no       |
| `body_size_limit`                    | `int`                   | An uncompressed response body larger than this many bytes causes the scrape to fail. 0 means no limit.                   |                                                                                                  | no       |
| `convert_classic_histograms_to_nhcb` | `bool`                  | Whether to convert classic histograms to native histograms with custom buckets (NHCB).                                   | `false`                                                                                          | no       |
| `enable_compression`                 | `bool`                  | Whether compression is enabled for the scrape.                                                                           | `true`                                                                                           | no       |
| `enable_http2`                       | `bool`                  | Whether HTTP2 is supported for requests.                                                                                 | `true`                                                                                           | no       |
| `enable_protobuf_negotiation`        | `bool`                  | Deprecated: use `scrape_protocols` instead.                                                                              | `false`                                                                                          | no       |
| `enable_type_and_unit_labels`        | `bool`                  | (Experimental) Whether the metric type and unit should be added as labels to scraped metrics.                            | `false`                                                                                          | no       |
| `extra_metrics`                      | `bool`                  | Whether extra metrics should be generated for scrape targets. Currently, cannot be updated at runtime.                   | `false`                                                                                          | no       |
| `follow_redirects`                   | `bool`                  | Whether redirects returned by the server should be followed.                                                             | `true`                                                                                           | no       |
| `http_headers`                       | `map(list(secret))`     | Custom HTTP headers to be sent along with each request. The map key is the header name.                                  |                                                                                                  | no       |
| `honor_labels`                       | `bool`                  | Indicator whether the scraped metrics should remain unmodified.                                                          | `false`                                                                                          | no       |
| `honor_timestamps`                   | `bool`                  | Indicator whether the scraped timestamps should be respected.                                                            | `true`                                                                                           | no       |
| `honor_metadata`                     | `bool`                  | (Experimental) Indicates whether to send metric metadata to downstream components.                                       | `false`                                                                                          | no       |
| `job_name`                           | `string`                | The value to use for the job label if not already set.                                                                   | component name                                                                                   | no       |
| `label_limit`                        | `uint`                  | More than this many labels post metric-relabeling causes the scrape to fail.                                             |                                                                                                  | no       |
| `label_name_length_limit`            | `uint`                  | More than this label name length post metric-relabeling causes the scrape to fail.                                       |                                                                                                  | no       |
| `label_value_length_limit`           | `uint`                  | More than this label value length post metric-relabeling causes the scrape to fail.                                      |                                                                                                  | no       |
| `metric_name_escaping_scheme`        | `string`                | The escaping scheme to use for metric names. See below for available values.                                             | `"underscores"`                                                                                  | no       |
| `metric_name_validation_scheme`      | `string`                | The validation scheme to use for metric names. See below for available values.                                           | `"legacy"`                                                                                       | no       |
| `metrics_path`                       | `string`                | The HTTP resource path on which to fetch metrics from targets.                                                           | `"/metrics"`                                                                                     | no       |
| `native_histogram_bucket_limit`      | `uint`                  | Native histogram buckets will be merged to stay within this limit. Disabled when set to zero.                            | `0`                                                                                              | no       |
| `native_histogram_min_bucket_factor` | `float64`               | If the growth from one bucket to the next is smaller than this, buckets will be merged. Disabled when set to zero.       | `0`                                                                                              | no       |
| `no_proxy`                           | `string`                | Comma-separated list of IP addresses, CIDR notations, and domain names to exclude from proxying.                         |                                                                                                  | no       |
| `params`                             | `map(list(string))`     | A set of query parameters with which the target is scraped.                                                              |                                                                                                  | no       |
| `proxy_connect_header`               | `map(list(secret))`     | Specifies headers to send to proxies during CONNECT requests.                                                            |                                                                                                  | no       |
| `proxy_from_environment`             | `bool`                  | Use the proxy URL indicated by environment variables.                                                                    | `false`                                                                                          | no       |
| `proxy_url`                          | `string`                | HTTP proxy to send requests through.                                                                                     |                                                                                                  | no       |
| `sample_limit`                       | `uint`                  | More than this many samples post metric-relabeling causes the scrape to fail                                             |                                                                                                  | no       |
| `scheme`                             | `string`                | The URL protocol scheme used to fetch metrics from targets.                                                        |                                                                                                  | no       |
| `scrape_classic_histograms`          | `bool`                  | Whether to scrape a classic histogram that's also exposed as a native histogram.                                         | `false`                                                                                          | no       |
| `scrape_failure_log_file`            | `string`                | File to which scrape failures are logged.                                                                                | `""`                                                                                             | no       |
| `scrape_fallback_protocol`           | `string`                | The fallback protocol to use if the target does not provide a valid Content-Type header. See below for available values. | `PrometheusText0_0_4`                                                                            | no       |
| `scrape_interval`                    | `duration`              | How frequently to scrape the targets of this scrape configuration.                                                       | `"60s"`                                                                                          | no       |
| `scrape_native_histograms`           | `bool`                  | Whether to scrape native histograms. Currently, cannot be updated at runtime.                                            | `false`                                                                                          | no       |
| `scrape_protocols`                   | `list(string)`          | The protocols to negotiate during a scrape, in order of preference. See below for available values.                      | `["OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText1.0.0", "PrometheusText0.0.4"]` | no       |
| `scrape_timeout`                     | `duration`              | The timeout for scraping targets of this configuration.                                                                  | `"10s"`                                                                                          | no       |
| `target_limit`                       | `uint`                  | More than this many targets after the target relabeling causes the scrapes to fail.                                      |                                                                                                  | no       |
| `track_timestamps_staleness`         | `bool`                  | Indicator whether to track the staleness of the scraped timestamps.                                                      | `false`                                                                                          | no       |

> **EXPERIMENTAL**: The `honor_metadata` argument is an [experimental][] feature.
> If you enable this argument, resource consumption may increase, particularly if you ingest many metrics with different names.
> Some downstream components aren't compatible with Prometheus metadata.
> The following components are compatible:
>
> * `otelcol.receiver.prometheus`
> * `prometheus.remote_write` only when configured for Remote Write v2.
> * `prometheus.write_queue`
>
> **EXPERIMENTAL**: The `enable_type_and_unit_labels` argument is an [experimental][] feature.
> When enabled and available from the scrape, the metric type and unit are added as labels to each scraped sample.
> This provides additional schema information about metrics directly in the label set.
> This feature doesn't require downstream components to support Remote Write v2.
>
> Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.
> To enable and use an experimental feature, you must set the `stability.level` [flag][] to `experimental`.

[experimental]: https://grafana.com/docs/release-life-cycle/
[flag]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/cli/run/

The `scrape_protocols` controls the preferred order of protocols to negotiate during a scrape.
The following values are supported:

* `OpenMetricsText0.0.1`
* `OpenMetricsText1.0.0`
* `PrometheusProto`
* `PrometheusText0.0.4`
* `PrometheusText1.0.0`

You can also use the `scrape_fallback_protocol` argument to specify a fallback protocol to use if the target does not provide a valid Content-Type header.

If you were using the deprecated `enable_protobuf_negotiation` argument, switch to using `scrape_protocols = ["PrometheusProto", "OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"]` instead.

For now, native histograms are only available through the Prometheus Protobuf exposition format.
To scrape native histograms, `scrape_native_histograms` must be set to `true` and the first item in `scrape_protocols` must be `PrometheusProto`.

The default value for `scrape_protocols` changes to `["PrometheusProto", "OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText1.0.0", "PrometheusText0.0.4"]` when
`scrape_native_histograms` is set to `true`.

The `metric_name_validation_scheme` controls how metric names are validated. The following values are supported:

* `"utf8"` - Uses UTF-8 validation scheme.
* `"legacy"` - Uses legacy validation scheme which was default in Prometheus v2 (default).

The `metric_name_escaping_scheme` controls how metric names are escaped. The following values are supported:

* `"allow-utf-8"` - Allows UTF-8 characters in metric names. No escaping is required. (default when validation scheme is "utf8")
* `"underscores"` - Replaces all legacy-invalid characters with underscores (default when validation scheme is "legacy")
* `"dots"` - Replaces all legacy-invalid characters with dots except that dots are converted to `_dot_` and pre-existing underscores are converted to `__`.
* `"values"` - Prepends the name with `U__` and replaces all invalid characters with the unicode value, surrounded by underscores. Single underscores are replaced with double underscores.

Note: `metric_name_escaping_scheme` cannot be set to `"allow-utf-8"` while `metric_name_validation_scheme` is not set to `"utf8"`.

{{< docs/shared lookup="reference/components/http-client-proxy-config-description.md" source="alloy" version="<ALLOY_VERSION>" >}}

`track_timestamps_staleness` controls whether Prometheus tracks [staleness][prom-staleness] of metrics with an explicit timestamp present in scraped data.

* An "explicit timestamp" is an optional timestamp in the [Prometheus metrics exposition format][prom-text-exposition-format]. For example, this sample has a timestamp of `1395066363000`:

  ```text
  http_requests_total{method="post",code="200"} 1027 1395066363000
  ```

* If `track_timestamps_staleness` is set to `true`, a staleness marker will be inserted when a metric is no longer present or the target is down.
* A "staleness marker" is just a {{< term "sample" >}}sample{{< /term >}} with a specific NaN value which is reserved for internal use by Prometheus.
* We recommend you set `track_timestamps_staleness` to `true` if the database where metrics are written to has enabled [out of order ingestion][mimir-ooo].
* If `track_timestamps_staleness` is set to `false`, samples with explicit timestamps will only be labeled as stale after a certain time period, which in Prometheus is 5 minutes by default.

[prom-text-exposition-format]: https://prometheus.io/docs/instrumenting/exposition_formats/#text-based-format
[prom-staleness]: https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness
[mimir-ooo]: https://grafana.com/docs/mimir/latest/configure/configure-out-of-order-samples-ingestion/

## Blocks

You can use the following blocks with `prometheus.scrape`:

| Block                                 | Description                                                                                 | Required |
| ------------------------------------- | ------------------------------------------------------------------------------------------- | -------- |
| [`authorization`][authorization]      | Configure generic authorization to targets.                                                 | no       |
| [`basic_auth`][basic_auth]            | Configure `basic_auth` for authenticating to targets.                                       | no       |
| [`clustering`][clustering]            | Configure the component for when {{< param "PRODUCT_NAME" >}} is running in clustered mode. | no       |
| [`oauth2`][oauth2]                    | Configure OAuth 2.0 for authenticating to targets.                                          | no       |
| `oauth2` > [`tls_config`][tls_config] | Configure TLS settings for connecting to targets via OAuth 2.0                              | no       |
| [`tls_config`][tls_config]            | Configure TLS settings for connecting to targets.                                           | no       |

The > symbol indicates deeper levels of nesting.
For example, `oauth2` > `tls_config` refers to a `tls_config` block defined inside an `oauth2` block.

[authorization]: #authorization
[basic_auth]: #basic_auth
[clustering]: #clustering
[oauth2]: #oauth2
[tls_config]: #tls_config

### `authorization`

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `basic_auth`

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `clustering`

| Name      | Type   | Description                                       | Default | Required |
| --------- | ------ | ------------------------------------------------- | ------- | -------- |
| `enabled` | `bool` | Enables sharing targets with other cluster nodes. | `false` | yes      |

When {{< param "PRODUCT_NAME" >}} is [using clustering][], and `enabled` is set to true, then this `prometheus.scrape` component instance opts-in to participating in the cluster to distribute scrape load between all cluster nodes.

Clustering assumes that all cluster nodes are running with the same configuration file, have access to the same service discovery APIs, and that all `prometheus.scrape` components that have opted-in to using clustering, over the course of a scrape interval, are converging on the same target set from
upstream components in their `targets` argument.

All `prometheus.scrape` components instances opting in to clustering use target labels and a consistent hashing algorithm to determine ownership for each of the targets between the cluster peers.
Then, each peer only scrapes the subset of targets that it's responsible for, so that the scrape load is distributed.
When a node joins or leaves the cluster, every peer recalculates ownership and continues scraping with the new target set.
This performs better than hashmod sharding where _all_ nodes have to be re-distributed, as only 1/N of the targets ownership is transferred, but is eventually consistent (rather than fully consistent like hashmod sharding is).

If {{< param "PRODUCT_NAME" >}} is _not_ running in clustered mode, then the block is a no-op and `prometheus.scrape` scrapes every target it receives in its arguments.

[using clustering]: ../../../../get-started/clustering/

### `oauth2`

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### `tls_config`

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

`prometheus.scrape` doesn't export any fields that can be referenced by other components.

## Component health

`prometheus.scrape` is only reported as unhealthy if given an invalid configuration.

## Debug information

`prometheus.scrape` reports the status of the last scrape for each configured scrape job on the component's debug endpoint.

## Debug metrics

* `prometheus_fanout_latency` (histogram): Write latency for sending to direct and indirect components.
* `prometheus_forwarded_samples_total` (counter): Total number of samples sent to downstream components.
* `prometheus_scrape_targets_gauge` (gauge): Number of targets this component is configured to scrape.

## Scraping behavior

The `prometheus.scrape` component borrows the scraping behavior of Prometheus.
Prometheus, and by extent this component, uses a pull model for scraping metrics from a given set of _targets_.
Each scrape target is defined as a set of key-value pairs called _labels_.
The set of targets can either be _static_, or dynamically provided periodically by a service discovery component such as `discovery.kubernetes`.
The special label `__address__` _must always_ be present and corresponds to the `<host>:<port>` that's used for the scrape request.

By default, the scrape job tries to scrape all available targets' `/metrics` endpoints using HTTP, with a scrape interval of 1 minute and scrape timeout of 10 seconds.
The metrics path, protocol scheme, scrape interval and timeout, query parameters, as well as any other settings can be configured using the component's arguments.

If a target is hosted at the [in-memory traffic][] address specified by the [run command][], `prometheus.scrape` scrapes the metrics in-memory, bypassing the network.

The scrape job expects the metrics exposed by the endpoint to follow the [OpenMetrics](https://openmetrics.io/) format.
All metrics are then propagated to each receiver listed in the component's `forward_to` argument.

Labels coming from targets, that start with a double underscore `__` are treated as _internal_, and are removed prior to scraping.

The `prometheus.scrape` component regards a scrape as successful if it responded with an HTTP `200 OK` status code and returned a body of valid metrics.

If the scrape request fails, the component's debug UI section contains more detailed information about the failure, the last successful scrape, as well as the labels last used for scraping.

The following labels are automatically injected to the scraped time series and can help pin down a scrape target.

| Label    | Description                                                                                      |
| -------- | ------------------------------------------------------------------------------------------------ |
| job      | The configured job name that the target belongs to. Defaults to the fully formed component name. |
| instance | The `__address__` or `<host>:<port>` of the scrape target's URL.                                 |

Similarly, these metrics that record the behavior of the scrape targets are also automatically available.

| Metric Name                             | Description                                                                                                                                                                                  |
| --------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `scrape_body_size_bytes`                | The uncompressed size of the most recent scrape response, if successful. Scrapes failing because the `body_size_limit` is exceeded report -1, other scrape failures report 0.                |
| `scrape_duration_seconds`               | Duration of the scrape in seconds.                                                                                                                                                           |
| `scrape_sample_limit`                   | The configured sample limit for a target. Useful for measuring how close a target was to reaching the sample limit using `scrape_samples_post_metric_relabeling / (scrape_sample_limit > 0)` |
| `scrape_samples_post_metric_relabeling` | The number of samples remaining after metric relabeling was applied.                                                                                                                         |
| `scrape_samples_scraped`                | The number of samples the target exposed.                                                                                                                                                    |
| `scrape_series_added`                   | The approximate number of new series in this scrape.                                                                                                                                         |
| `scrape_timeout_seconds`                | The configured scrape timeout for a target. Useful for measuring how close a target was to timing out using `scrape_duration_seconds / scrape_timeout_seconds`                               |
| `up`                                    | 1 if the instance is healthy and reachable, or 0 if the scrape failed.                                                                                                                       |

The `up` metric is particularly useful for monitoring and alerting on the health of a scrape job.
It's set to `0` in case anything goes wrong with the scrape target, either because it's not reachable, because the connection times out while scraping, or because the samples from the target couldn't be processed.
When the target is behaving normally, the `up` metric is set to `1`.

To enable scraping of Prometheus' native histograms over gRPC, the `scrape_protocols` should specify `PrometheusProto` as the first protocol to negotiate, for example:

```alloy
prometheus.scrape "prometheus" {
  ...
  scrape_native_histograms = true
  scrape_protocols = ["PrometheusProto", "OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"]
}
```

The`scrape_classic_histograms` argument controls whether the component should also scrape the 'classic' histogram equivalent of a native histogram, if it's present. It's an equivalent to the `always_scrape_classic_histograms` argument in Prometheus v3.

[in-memory traffic]: ../../../../get-started/component_controller/#in-memory-traffic
[run command]: ../../../cli/run/

## Example

### Set up scrape jobs for `blackbox exporter` targets  

The following example sets up the scrape job with certain attributes (scrape endpoint, scrape interval, query parameters) and lets it scrape two instances of the [blackbox exporter](https://github.com/prometheus/blackbox_exporter/).
The exposed metrics are sent over to the provided list of receivers, as defined by other components.

```alloy
prometheus.scrape "blackbox_scraper" {
  targets = [
    {"__address__" = "blackbox-exporter:9115", "instance" = "one"},
    {"__address__" = "blackbox-exporter:9116", "instance" = "two"},
  ]

  forward_to = [prometheus.remote_write.grafanacloud.receiver, prometheus.remote_write.onprem.receiver]

  scrape_interval = "10s"
  params          = { "target" = ["grafana.com"], "module" = ["http_2xx"] }
  metrics_path    = "/probe"
}
```

The endpoints that are being scraped every 10 seconds are:

```text
http://blackbox-exporter:9115/probe?target=grafana.com&module=http_2xx
http://blackbox-exporter:9116/probe?target=grafana.com&module=http_2xx
```

### Authentication with the Kubernetes API server

The following example shows you how to authenticate with the Kubernetes API server.

```alloy
prometheus.scrape "kubelet" {
        scheme = "https"
        tls_config {
            server_name = "kubernetes"
            ca_file = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
            insecure_skip_verify = false
        }
        bearer_token_file = "/var/run/secrets/kubernetes.io/serviceaccount/token"
}
```

### Technical details

`prometheus.scrape` supports [gzip](https://en.wikipedia.org/wiki/Gzip) compression.

The following special labels can change the behavior of `prometheus.scrape`:

* `__address__`: The name of the label that holds the `<host>:<port>` address of a scrape target.
* `__metrics_path__`: The name of the label that holds the path on which to scrape a target.
* `__param_<name>`: A prefix for labels that provide URL parameters `<name>` used to scrape a target.
* `__scheme__`: the name of the label that holds the protocol scheme (`http`, `https`) on which to scrape a target.
* `__scrape_interval__`: The name of the label that holds the scrape interval used to scrape a target.
* `__scrape_timeout__`: The name of the label that holds the scrape timeout used to scrape a target.

Special labels added after a scrape

* `__name__`: The label name indicating the metric name of a timeseries.
* `instance`: The label name used for the instance label.
* `job`: The label name indicating the job from which a timeseries was scraped.

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.scrape` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

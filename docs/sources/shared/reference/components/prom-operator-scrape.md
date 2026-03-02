---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/prom-operator-scrape/
description: Shared content, prom operator scrape
headless: true
---

| Name                          | Type       | Description                                                                                                                  | Default | Required |
|-------------------------------|------------|------------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `default_sample_limit`        | `int`      | The default maximum samples per scrape. Used as the default if the target resource doesn't provide a sample limit.           |         | no       |
| `default_scrape_interval`     | `duration` | The default interval between scraping targets. Used as the default if the target resource doesn't provide a scrape interval. | `1m`    | no       |
| `default_scrape_timeout`      | `duration` | The default timeout for scrape requests. Used as the default if the target resource doesn't provide a scrape timeout.        | `10s`   | no       |
| `enable_type_and_unit_labels` | `bool`     | (Experimental) Whether the metric type and unit should be added as labels to scraped metrics.                                | `false` | no       |
| `honor_metadata`              | `bool`     | (Experimental) Indicates whether to send metric metadata to downstream components.                                           | `false` | no       |
| `scrape_native_histograms`    | `bool`     | Whether to scrape native histograms from targets.                                                                            | `false` | no       |

> **EXPERIMENTAL**: The `honor_metadata` and `enable_type_and_unit_labels` arguments are [experimental][] features.
> 
> If you enable the `honor_metadata` argument, resource consumption may increase, particularly if you ingest many metrics with different names.
> Some downstream components aren't compatible with Prometheus metadata.
> The following components are compatible:
>
> * `otelcol.receiver.prometheus`
> * `prometheus.remote_write` only when configured for Remote Write v2.
> * `prometheus.write_queue`
> 
> When `enable_type_and_unit_labels` argument is enabled and available from the scrape, the metric type and unit are added as labels to each scraped sample.
> This provides additional schema information about metrics directly in the label set.
> This feature doesn't require downstream components to support Remote Write v2.
> 
> Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.
> To enable and use an experimental feature, you must set the `stability.level` [flag][] to `experimental`.

[experimental]: https://grafana.com/docs/release-life-cycle/
[flag]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/cli/run/

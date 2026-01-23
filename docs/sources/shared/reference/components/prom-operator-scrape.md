---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/prom-operator-scrape/
description: Shared content, prom operator scrape
headless: true
---

| Name                       | Type       | Description                                                                                                                  | Default | Required |
|----------------------------|------------|------------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `default_sample_limit`     | `int`      | The default maximum samples per scrape. Used as the default if the target resource doesn't provide a sample limit.           |         | no       |
| `default_scrape_interval`  | `duration` | The default interval between scraping targets. Used as the default if the target resource doesn't provide a scrape interval. | `1m`    | no       |
| `default_scrape_timeout`   | `duration` | The default timeout for scrape requests. Used as the default if the target resource doesn't provide a scrape timeout.        | `10s`   | no       |
| `scrape_native_histograms` | `bool`     | Whether to scrape native histograms from targets.                                                                            | `false` | no       |
| `honor_metadata`           | `bool`     | (Experimental) Indicator whether metric metadata should be sent to downstream components.                                    | `false` | no       |

> **EXPERIMENTAL**: The `honor_metadata` argument is an [experimental][] feature.
> Enabling it may increase resource consumption, particularly if a lot of metrics with different names are ingested.
> Not all downstream components may be compatible with Prometheus metadata yet.
> Currently, the compatible components are:
>
> * `otelcol.receiver.prometheus`
> * `prometheus.remote_write` only when configured for Remote Write v2.
> * `prometheus.write_queue`
> 
> Metadata support for Remote Write v1 in `prometheus.remote_write` will be added soon.
> 
> Experimental features are subject to frequent breaking changes, and may be removed with no equivalent replacement.
> To enable and use an experimental feature, you must set the `stability.level` [flag][] to `experimental`.

[experimental]: https://grafana.com/docs/release-life-cycle/
[flag]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/cli/run/

---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/prom-operator-scrape/
description: Shared content, prom operator scrape
headless: true
---

| Name                            | Type       | Description                                                                                                                  | Default    | Required |
|---------------------------------|------------|------------------------------------------------------------------------------------------------------------------------------|------------|----------|
| `default_scrape_interval`       | `duration` | The default interval between scraping targets. Used as the default if the target resource doesn't provide a scrape interval. | `1m`       | no       |
| `default_scrape_timeout`        | `duration` | The default timeout for scrape requests. Used as the default if the target resource doesn't provide a scrape timeout.        | `10s`      | no       |
| `scrape_native_histograms`      | `bool`     | Whether to scrape native histograms from targets.                                                                            | `false`    | no       |
| `metric_name_validation_scheme` | `string`   | The validation scheme to use for metric names. Supported values: `"legacy"`, `"utf8"`.                                       | `"legacy"` | no       |

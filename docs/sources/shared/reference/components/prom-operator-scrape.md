---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/prom-operator-scrape/
description: Shared content, prom operator scrape
headless: true
---

| Name                       | Type       | Description                                                                                                                  | Default | Required |
|----------------------------|------------|------------------------------------------------------------------------------------------------------------------------------|---------|----------|
| `default_scrape_interval`  | `duration` | The default interval between scraping targets. Used as the default if the target resource doesn't provide a scrape interval. | `1m`    | no       |
| `default_scrape_timeout`   | `duration` | The default timeout for scrape requests. Used as the default if the target resource doesn't provide a scrape timeout.        | `10s`   | no       |
| `scrape_native_histograms` | `bool`     | Allow the scrape manager to ingest native histograms.                                                                        | `false` | no       |

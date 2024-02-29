---
aliases:
- /docs/agent/shared/flow/reference/components/prom-operator-scrape/
- /docs/grafana-cloud/monitor-infrastructure/agent/shared/flow/reference/components/prom-operator-scrape/
- /docs/grafana-cloud/send-data/agent/shared/flow/reference/components/prom-operator-scrape/
canonical: https://grafana.com/docs/agent/latest/shared/flow/reference/components/prom-operator-scrape/
description: Shared content, prom operator scrape
headless: true
---

Name                      | Type       | Description                                                                                                                  | Default | Required
--------------------------|------------|------------------------------------------------------------------------------------------------------------------------------|---------|---------
`default_scrape_interval` | `duration` | The default interval between scraping targets. Used as the default if the target resource doesn't provide a scrape interval. | `1m`    | no
`default_scrape_timeout`  | `duration` | The default timeout for scrape requests. Used as the default if the target resource doesn't provide a scrape timeout.        | `10s`   | no

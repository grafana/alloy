---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/prom-operator-scrape-class-metric-relabel-rule/
description: Shared content, prom operator scrape class metric relabel rule
headless: true
---

The `metric_relabel_rule` block has the same arguments as the `rule` block.
Rules defined here are prepended to the metric relabeling rules of resources that reference the scrape class.

{{< docs/shared lookup="reference/components/rule-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

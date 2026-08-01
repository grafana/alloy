---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/prom-operator-scrape-class/
description: Shared content, prom operator scrape class
headless: true
---

The `scrape_class` block defines a named set of scrape settings that discovered resources can reference through their `scrapeClass` field.
This mirrors the [Prometheus Operator ScrapeClass](https://prometheus-operator.dev/docs/developer/scrapeclass/) feature.
You can define multiple `scrape_class` blocks.

| Name      | Type     | Description                                                               | Default | Required |
| --------- | -------- | ------------------------------------------------------------------------- | ------- | -------- |
| `name`    | `string` | Name of the scrape class, referenced by a resource's `scrapeClass` field. |         | yes      |
| `default` | `bool`   | Apply this class to resources that don't reference a scrape class.        | `false` | no       |

At most one `scrape_class` block can set `default` to `true`.
A resource's own settings take precedence over the scrape class.
Scrape class relabeling rules are prepended to the resource's relabeling rules, and metric relabeling rules are appended.
Referencing a scrape class that isn't defined is an error.

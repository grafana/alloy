---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/prom-operator-scrape-class-attach-metadata/
description: Shared content, prom operator scrape class attach metadata
headless: true
---

The `attach_metadata` block configures metadata attached to discovered targets when the resource doesn't set its own.

| Name   | Type   | Description                                 | Default | Required |
| ------ | ------ | ------------------------------------------- | ------- | -------- |
| `node` | `bool` | Attach node metadata to discovered targets. | `false` | no       |

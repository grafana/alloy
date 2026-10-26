---
description: Shared content, otelcol Kafka metadata
headless: true
---

The `metadata` block configures how to retrieve and store metadata from the Kafka broker.

The following arguments are supported:

| Name               | Type       | Description                                           | Default | Required |
| ------------------ | ---------- | ----------------------------------------------------- | ------- | -------- |
| `full`             | `bool`     | Deprecated and has no effect.                         | `true`  | no       |
| `refresh_interval` | `duration` | The frequency at which cluster metadata is refreshed. | `"10m"` | no       |

The `full` argument is deprecated and has no effect because the underlying Kafka client no longer supports it.

Retrieving metadata may fail if the Kafka broker is starting up at the same time as the {{< param "PRODUCT_NAME" >}} component.
The `retry` child block can be provided to customize retry behavior.
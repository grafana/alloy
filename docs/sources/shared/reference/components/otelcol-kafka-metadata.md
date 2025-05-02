---
description: Shared content, otelcol Kafka metadata
headless: true
---

The `metadata` block configures how to retrieve and store metadata from the Kafka broker.

The following arguments are supported:

Name               | Type       | Description                                           | Default  | Required
-------------------|------------|-------------------------------------------------------|----------|---------
`full`             | `bool`     | Whether to maintain a full set of metadata.           | `true`   | no
`refresh_interval` | `duration` | The frequency at which cluster metadata is refreshed. | `"10m"`  | no

When `full` is set to `false`, the client does not make the initial request to broker at the startup.


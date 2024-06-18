---
description: Shared content, otelcol Kafka metadata retry
headless: true
---

The `retry` block configures how to retry retrieving metadata when retrieval
fails.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`max_retries` | `number` | How many times to reattempt retrieving metadata. | `3` | no
`backoff` | `duration` | Time to wait between retries. | `"250ms"` | no

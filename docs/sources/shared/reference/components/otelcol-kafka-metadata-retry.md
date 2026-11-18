---
description: Shared content, otelcol Kafka metadata retry
headless: true
---

The `retry` block configures how to retry retriable Kafka requests, such as when the cluster is in the middle of a leader election or the broker is still starting up.
It applies to metadata, consumer group, offset commit and fetch, and administrator requests.

The following arguments are supported:

| Name          | Type       | Description                                      | Default   | Required |
| ------------- | ---------- | ------------------------------------------------ | --------- | -------- |
| `backoff`     | `duration` | Minimum time to wait before retrying a request.  | `"250ms"` | no       |
| `max_retries` | `number`   | How many times to reattempt a retriable request. | `20`      | no       |

Each successive retry doubles the wait time, with jitter applied, up to a maximum of the greater of `5s` and `backoff`.

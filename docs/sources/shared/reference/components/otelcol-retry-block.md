---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-retry-block/
description: Shared content, otelcol retry block
headless: true
---

The following arguments are supported:

| Name                   | Type       | Description                                            | Default | Required |
| ---------------------- | ---------- | ------------------------------------------------------ | ------- | -------- |
| `enabled`              | `boolean`  | Enables retrying failed requests.                      | `true`  | no       |
| `initial_interval`     | `duration` | Initial time to wait before retrying a failed request. | `"5s"`  | no       |
| `max_elapsed_time`     | `duration` | Maximum time to wait before discarding a failed batch. | `"5m"`  | no       |
| `max_interval`         | `duration` | Maximum time to wait between retries.                  | `"30s"` | no       |
| `multiplier`           | `number`   | Factor to grow wait time before retrying.              | `1.5`   | no       |
| `randomization_factor` | `number`   | Factor to randomize wait time before retrying.         | `0.5`   | no       |

When `enabled` is `true`, failed batches are retried after a given interval.
The `initial_interval` argument specifies how long to wait before the first retry attempt.
If requests continue to fail, the time to wait before retrying increases by the factor specified by the `multiplier` argument, which must be greater than `1.0`.
The `max_interval` argument specifies the upper bound of how long to wait between retries.

The `randomization_factor` argument is useful for adding jitter between retrying Alloy instances.
If `randomization_factor` is greater than `0`, the wait time before retries is multiplied by a random factor in the range `[ I - randomization_factor * I, I + randomization_factor * I]`, where `I` is the current interval.

If a batch hasn't been sent successfully, it's discarded after the time specified by `max_elapsed_time` elapses.
If `max_elapsed_time` is set to `"0s"`, failed requests are retried forever until they succeed.

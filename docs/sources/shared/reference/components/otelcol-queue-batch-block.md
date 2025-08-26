---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-queue-batch-block/
description: Shared content, otelcol queue batch block
headless: true
---

The following arguments are supported:

| Name            | Type        | Description                                                                                                | Default      | Required |
| --------------- | ----------- | ---------------------------------------------------------------------------------------------------------- | ------------ | -------- |
| `flush_timeout` | `duration`  | Time after which a batch will be sent regardless of its size. Must be a non-zero value.                    |              | yes      |
| `min_size`      | `number`    | The minimum size of a batch.                                                                               |              | yes      |
| `max_size`      | `number`    | The maximum size of a batch, enables batch splitting.                                                      |              | yes      |
| `sizer`         | `string`    | How the queue and batching is measured. Overrides the sizer set at the `sending_queue` level for batching. |              | yes      |

`max_size` should be greater than or equal to `min_size`.

The `sizer` argument could be set to:

* `items`: number of the smallest parts of each signal (spans, metric data points, log records).
* `bytes`: the size of serialized data in bytes (the least performant option).

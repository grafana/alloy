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

`max_size` must be greater than or equal to `min_size`.

The `sizer` argument can be set to:

* `items`: The number of the smallest parts of each span, metric data point, or log record.
* `bytes`: the size of serialized data in bytes (the least performant option).

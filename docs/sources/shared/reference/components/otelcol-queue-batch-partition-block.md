---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-queue-batch-partition-block/
description: Shared content, otelcol queue batch partition block
headless: true
---

Without `metadata_keys`, the exporter uses a single batcher for all data.
Set `metadata_keys` to batch each distinct combination of client metadata values separately, for example one for each tenant.

The following arguments are supported:

| Name            | Type           | Description                                                                        | Default  | Required |
| --------------- | -------------- | ---------------------------------------------------------------------------------- | -------- | -------- |
| `cache_size`    | `number`       | The maximum number of active partitions. Must be a positive value.                 | `10000`  | no       |
| `idle_timeout`  | `duration`     | How long a partition can stay empty before it's removed. Must be a positive value. | `"90s"`  | no       |
| `metadata_keys` | `list(string)` | The client metadata keys to partition batches by.                                  |          | no       |

The `metadata_keys` argument has the following behavior:

* Client metadata is available only if the receiver that gets the data includes it, for example by setting `include_metadata` to `true`.
* Keys are case-insensitive, and duplicate keys aren't allowed.
* A missing metadata value and an empty metadata value create different partitions.
* If you leave it empty, the `cache_size` and `idle_timeout` arguments have no effect.

When the number of active partitions reaches `cache_size`, the least recently used partition is sent and removed.
Keep `idle_timeout` longer than the interval between incoming data, to avoid removing and re-creating partitions.

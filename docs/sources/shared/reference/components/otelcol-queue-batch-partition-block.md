---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-queue-batch-partition-block/
description: Shared content, otelcol queue batch partition block
headless: true
---

The `partition` block configures separate batchers based on client metadata.

The following arguments are supported:

| Name            | Type           | Description                                                                                                              | Default | Required |
| --------------- | -------------- | ------------------------------------------------------------------------------------------------------------------------ | ------- | -------- |
| `metadata_keys` | `list(string)` | Client metadata keys used to partition data into separate batches. Entries are case-insensitive and must be unique. | `[]`    | no       |

When `metadata_keys` is empty, a single batcher is used. Otherwise, one batcher is used for each distinct combination of values for the listed metadata keys. Empty and unset metadata values are treated as distinct cases.

The receiver must be configured to propagate client metadata for partitioning to take effect.

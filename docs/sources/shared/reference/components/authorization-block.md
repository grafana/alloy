---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/authorization-block/
description: Shared content, authorization block
headless: true
---

| Name               | Type     | Description                                | Default | Required |
| ------------------ | -------- | ------------------------------------------ | ------- | -------- |
| `credentials_file` | `string` | File containing the secret value.          |         | no       |
| `credentials`      | `secret` | Secret value.                              |         | no       |
| `type`             | `string` | Authorization type, for example, "Bearer". |         | no       |

`credential` and `credentials_file` are mutually exclusive, and only one can be provided inside an `authorization` block.

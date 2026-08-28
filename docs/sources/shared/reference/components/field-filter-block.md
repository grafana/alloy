---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/filter-field-block/
description: Shared content, filter field block
headless: true
---

The following attributes are supported:

| Name    | Type     | Description                                                   | Default  | Required |
| ------- | -------- | ------------------------------------------------------------- | -------- | -------- |
| `key`   | `string` | The key or name of the field or labels that a filter can use. |          | yes      |
| `value` | `string` | The value associated with the key that a filter can use.      |          | yes      |
| `op`    | `string` | The filter operation to apply on the given key: value pair.   | `equals` | no       |

You can use the following values for `op`:

* `equals`: The field value must equal the provided value.
* `not-equals`: The field value must not be equal to the provided value.
* `exists`: The field value must exist. Only applicable to `annotation` fields.
* `does-not-exist`: The field value must not exist. Only applicable to `annotation` fields.

---
description: Shared content, otelcol filter resource block
headless: true
---

This block specifies items to match the resources against:

* More than one `resource` block can be defined.
* A match occurs if the input data resources match at least one `resource` block.

The following arguments are supported:

| Name    | Type     | Description                          | Default | Required |
| ------- | -------- | ------------------------------------ | ------- | -------- |
| `key`   | `string` | The resource key.                    |         | yes      |
| `value` | `any`    | The resource value to match against. |         | no       |

If `value` isn't set, any value matches.
The type of `value` could be a number, a string, or a boolean.

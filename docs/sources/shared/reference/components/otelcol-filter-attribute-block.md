---
description: Shared content, otelcol filter attribute block
headless: true
---

This block specifies an attribute to match against:

* You can define more than one `attribute` block.
* Only `match_type = "strict"` is allowed if `attribute` is specified.
* All `attribute` blocks must match exactly for a match to occur.

The following arguments are supported:

| Name    | Type     | Description                           | Default | Required |
| ------- | -------- | ------------------------------------- | ------- | -------- |
| `key`   | `string` | The attribute key.                    |         | yes      |
| `value` | `any`    | The attribute value to match against. |         | no       |

If `value` isn't set, any value matches.
The type of `value` could be a number, a string, or a boolean.

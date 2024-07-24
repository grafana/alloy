---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/nonsensitive/
description: Learn about nonsensitive
title: nonsensitive
---

# nonsensitive

`nonsensitive` converts a [secret][] value back into a string.

{{< admonition type="warning" >}}
Only use `nonsensitive` when you are positive that the value converted back to a string isn't a sensitive value.

Strings resulting from calls to `nonsensitive` are displayed in plain text in the UI and internal API calls.
{{< /admonition >}}

## Examples

```
// Assuming `sensitive_value` is a secret:

> sensitive_value
(secret)
> nonsensitive(sensitive_value)
"Hello, world!"
```

[secret]: ../../../get-started/configuration-syntax/expressions/types_and_values/#secrets

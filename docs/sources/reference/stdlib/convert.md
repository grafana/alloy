---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/convert/
description: Learn about convert functions
aliases:
  - ./nonsensitive/ # /docs/alloy/latest/reference/stdlib/nonsensitive/
title: convert
---

# convert

## nonsensitive

`convert.nonsensitive` converts a [secret][] value back into a string.

{{< admonition type="warning" >}}
Only use `convert.nonsensitive` when you are positive that the value converted back to a string isn't a sensitive value.

Strings resulting from calls to `convert.nonsensitive` are displayed in plain text in the UI and internal API calls.
{{< /admonition >}}

### Examples

```
// Assuming `sensitive_value` is a secret:

> sensitive_value
(secret)
> convert.nonsensitive(sensitive_value)
"Hello, world!"
```

[secret]: ../../../get-started/configuration-syntax/expressions/types_and_values/#secrets

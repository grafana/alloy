---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/trim_prefix/
description: Learn about trim_prefix
title: trim_prefix
---

# trim_prefix

`trim_prefix` removes the prefix from the start of a string.
If the string doesn't start with the prefix, the string is returned unchanged.

## Examples

```river
> trim_prefix("helloworld", "hello")
"world"
```

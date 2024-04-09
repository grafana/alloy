---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/replace/
description: Learn about replace
title: replace
---

# replace

`replace` searches a string for a substring, and replaces each occurrence of the substring with a replacement string.

```alloy
replace(string, substring, replacement)
```

## Examples

```alloy
> replace("1 + 2 + 3", "+", "-")
"1 - 2 - 3"
```

---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/coalesce/
description: Learn about coalesce
title: coalesce
---

# coalesce

`coalesce` takes any number of arguments and returns the first one that isn't null, an empty string, empty list, or an empty object.
It's useful for obtaining a default value, such as if an environment variable isn't defined.
If no argument is non-empty or non-zero, the last argument is returned.

## Examples

```alloy
> coalesce("a", "b")
a
> coalesce("", "b")
b
> coalesce(sys.env("DOES_NOT_EXIST"), "c")
c
```

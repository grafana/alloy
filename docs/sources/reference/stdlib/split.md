---
aliases:
- ./reference/stdlib/split/
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/split/
description: Learn about split
title: split
---

# split

`split` produces a list by dividing a string at all occurrences of a separator.

```river
split(list, separator)
```

## Examples

```river
> split("foo,bar,baz", "," )
["foo", "bar", "baz"]

> split("foo", ",")
["foo"]

> split("", ",")
[""]
```

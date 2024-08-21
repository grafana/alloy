---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/array/
description: Learn about array functions
title: array
---

# array.concat (previously `concat`)

The `array.concat` function concatenates one or more lists of values into a single list.
Each argument to `array.concat` must be a list value.
Elements within the list can be any type.

## Examples

```
> array.concat([])
[]

> array.concat([1, 2], [3, 4])
[1, 2, 3, 4]

> array.concat([1, 2], [], [bool, null])
[1, 2, bool, null]

> array.concat([[1, 2], [3, 4]], [[5, 6]])
[[1, 2], [3, 4], [5, 6]]
```

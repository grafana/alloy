---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/join/
description: Learn about join
title: join
---

# join

`join` all items in an array into a string, using a character as separator.

```alloy
join(list, separator)
```

## Examples

```alloy
> join(["foo", "bar", "baz"], "-")
"foo-bar-baz"
> join(["foo", "bar", "baz"], ", ")
"foo, bar, baz"
> join(["foo"], ", ")
"foo"
```

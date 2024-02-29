---
aliases:
- ./reference/stdlib/trim/
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/trim/
description: Learn about trim
title: trim
---

# trim

`trim` removes the specified set of characters from the start and end of a string.

```river
trim(string, str_character_set)
```

## Examples

```river
> trim("?!hello?!", "!?")
"hello"

> trim("foobar", "far")
"oob"

> trim("   hello! world.!  ", "! ")
"hello! world."
```

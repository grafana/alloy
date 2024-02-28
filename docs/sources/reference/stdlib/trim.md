---
aliases:
- ../../configuration-language/standard-library/trim/
- /docs/grafana-cloud/agent/flow/reference/stdlib/trim/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/trim/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/trim/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/trim/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/trim/
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

---
aliases:
- ../../configuration-language/standard-library/join/
- /docs/grafana-cloud/agent/flow/reference/stdlib/join/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/join/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/join/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/join/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/join/
description: Learn about join
title: join
---

# join

`join` all items in an array into a string, using a character as separator.

```river
join(list, separator)
```

## Examples

```river
> join(["foo", "bar", "baz"], "-")
"foo-bar-baz"
> join(["foo", "bar", "baz"], ", ")
"foo, bar, baz"
> join(["foo"], ", ")
"foo"
```

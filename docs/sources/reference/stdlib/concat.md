---
aliases:
- ../../configuration-language/standard-library/concat/
- /docs/grafana-cloud/agent/flow/reference/stdlib/concat/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/concat/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/concat/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/concat/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/concat/
description: Learn about concat
title: concat
---

# concat

The `concat` function concatenates one or more lists of values into a single
list. Each argument to `concat` must be a list value. Elements within the list
can be any type.

## Examples

```
> concat([])
[]

> concat([1, 2], [3, 4])
[1, 2, 3, 4]

> concat([1, 2], [], [bool, null])
[1, 2, bool, null]

> concat([[1, 2], [3, 4]], [[5, 6]])
[[1, 2], [3, 4], [5, 6]]
```

---
aliases:
- ../../configuration-language/standard-library/split/
- /docs/grafana-cloud/agent/flow/reference/stdlib/split/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/split/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/split/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/split/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/split/
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

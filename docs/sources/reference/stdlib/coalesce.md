---
aliases:
- ../../configuration-language/standard-library/coalesce/
- /docs/grafana-cloud/agent/flow/reference/stdlib/coalesce/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/coalesce/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/coalesce/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/coalesce/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/coalesce/
description: Learn about coalesce
title: coalesce
---

# coalesce

`coalesce` takes any number of arguments and returns the first one that isn't null, an empty string, empty list, or an empty object.
It is useful for obtaining a default value, such as if an environment variable isn't defined.
If no argument is non-empty or non-zero, the last argument is returned.

## Examples

```
> coalesce("a", "b")
a
> coalesce("", "b")
b
> coalesce(env("DOES_NOT_EXIST"), "c")
c
```

---
aliases:
- ../../configuration-language/standard-library/trim_prefix/
- /docs/grafana-cloud/agent/flow/reference/stdlib/trim_prefix/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/trim_prefix/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/trim_prefix/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/trim_prefix/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/trim_prefix/
description: Learn about trim_prefix
title: trim_prefix
---

# trim_prefix

`trim_prefix` removes the prefix from the start of a string. If the string does not start with the prefix, the string is returned unchanged.

## Examples

```river
> trim_prefix("helloworld", "hello")
"world"
```

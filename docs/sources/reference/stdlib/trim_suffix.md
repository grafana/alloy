---
aliases:
- ../../configuration-language/standard-library/trim_suffix/
- /docs/grafana-cloud/agent/flow/reference/stdlib/trim_suffix/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/trim_suffix/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/trim_suffix/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/trim_suffix/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/trim_suffix/
description: Learn about trim_suffix
title: trim_suffix
---

# trim_suffix

`trim_suffix` removes the suffix from the end of a string.

## Examples

```river
> trim_suffix("helloworld", "world")
"hello"
```

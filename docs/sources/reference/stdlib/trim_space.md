---
aliases:
- ../../configuration-language/standard-library/trim_space/
- /docs/grafana-cloud/agent/flow/reference/stdlib/trim_space/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/trim_space/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/trim_space/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/trim_space/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/trim_space/
description: Learn about trim_space
title: trim_space
---

# trim_space

`trim_space` removes any whitespace characters from the start and end of a string.

## Examples

```river
> trim_space("  hello\n\n")
"hello"
```

---
aliases:
- ../../configuration-language/standard-library/env/
- /docs/grafana-cloud/agent/flow/reference/stdlib/env/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/env/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/env/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/env/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/env/
description: Learn about env
title: env
---

# env

The `env` function gets the value of an environment variable from the system {{< param "PRODUCT_NAME" >}} is running on.
If the environment variable does not exist, `env` returns an empty string.

## Examples

```
> env("HOME")
"/home/grafana-agent"

> env("DOES_NOT_EXIST")
""
```

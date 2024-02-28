---
aliases:
- ../../configuration-language/expressions/function-calls/ # /docs/agent/latest/flow/concepts/configuration-language/expressions/function-calls/
- /docs/grafana-cloud/agent/flow/concepts/config-language/expressions/function_calls/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/config-language/expressions/function_calls/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/config-language/expressions/function_calls/
- /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/expressions/function_calls/
# Previous page aliases for backwards compatibility:
- ../../../configuration-language/expressions/function-calls/ # /docs/agent/latest/flow/configuration-language/expressions/function-calls/
- /docs/grafana-cloud/agent/flow/config-language/expressions/function_calls/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/config-language/expressions/function_calls/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/config-language/expressions/function_calls/
- /docs/grafana-cloud/send-data/agent/flow/config-language/expressions/function_calls/
canonical: https://grafana.com/docs/agent/latest/flow/concepts/config-language/expressions/function_calls/
description: Learn about function calls
title: Function calls
weight: 400
---

# Function calls

You can use River function calls to build richer expressions.

Functions take zero or more arguments as their input and always return a single value as their output.
You can't construct functions. You can call functions from River's standard library or export them from a component.

If a function fails, the expression isn't evaluated, and an error is reported.

## Standard library functions

River contains a [standard library][] of functions.
Some functions enable interaction with the host system, for example, reading from an environment variable.
Some functions allow for more complex expressions, for example, concatenating arrays or decoding JSON strings into objects.

```river
env("HOME")
json_decode(local.file.cfg.content)["namespace"]
```

{{% docs/reference %}}
[standard library]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/stdlib"
[standard library]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/stdlib"
{{% /docs/reference %}}
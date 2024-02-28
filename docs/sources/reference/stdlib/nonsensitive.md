---
aliases:
- ../../configuration-language/standard-library/nonsensitive/
- /docs/grafana-cloud/agent/flow/reference/stdlib/nonsensitive/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/nonsensitive/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/nonsensitive/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/nonsensitive/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/nonsensitive/
description: Learn about nonsensitive
title: nonsensitive
---

# nonsensitive

`nonsensitive` converts a [secret][] value back into a string.

> **WARNING**: Only use `nonsensitive` when you are positive that the value
> being converted back to a string is not a sensitive value.
>
> Strings resulting from calls to `nonsensitive` will be displayed in plaintext
> in the UI and internal API calls.

[secret]: {{< relref "../../concepts/config-language/expressions/types_and_values.md#secrets" >}}

## Examples

```
// Assuming `sensitive_value` is a secret:

> sensitive_value
(secret)
> nonsensitive(sensitive_value)
"Hello, world!"
```

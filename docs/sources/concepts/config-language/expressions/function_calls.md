---
canonical: https://grafana.com/docs/alloy/latest/concepts/config-language/expressions/function_calls/
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

[standard library]:../../../../reference/stdlib/

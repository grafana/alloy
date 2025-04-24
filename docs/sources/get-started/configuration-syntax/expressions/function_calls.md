---
canonical: https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/expressions/function_calls/
aliases:
  - ../../../concepts/configuration-syntax/expressions/function_calls/ # /docs/alloy/latest/concepts/configuration-syntax/expressions/function_calls/
description: Learn about function calls
title: Function calls
weight: 400
---

# Function calls

You can use {{< param "PRODUCT_NAME" >}} function calls to create richer expressions.

Functions take zero or more arguments as input and always return a single value as output.
You can't construct functions.
You can call functions from the standard library or export them from a component.

If a function fails, the expression isn't evaluated, and the system reports an error.

## Standard library functions

The {{< param "PRODUCT_NAME" >}} configuration syntax includes a [standard library][] of functions.
Some functions interact with the host system, for example, reading from an environment variable.
Other functions enable more complex expressions, for example, concatenating arrays or decoding JSON strings into objects.

```alloy
sys.env("HOME")
encoding.from_json(local.file.cfg.content)["namespace"]
```

[standard library]:../../../../reference/stdlib/

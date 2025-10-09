---
canonical: https://grafana.com/docs/alloy/latest/get-started/expressions/function_calls/
aliases:
  - ./configuration-syntax/expressions/function_calls/ # /docs/alloy/latest/get-started/configuration-syntax/expressions/function_calls/
description: Learn about function calls
title: Function calls
weight: 40
---

# Function calls

You can use {{< param "PRODUCT_NAME" >}} function calls to create richer expressions and transform data in your configuration.

Functions are reusable pieces of code that take input values and return a computed result.
They help you process data, interact with the system, and create dynamic configurations.

## How functions work

Functions take zero or more arguments as input and always return a single value as output.
{{< param "PRODUCT_NAME" >}} provides functions through two sources:

- **Standard library functions**: Built-in functions available in any configuration
- **Component exports**: Functions exported by components in your configuration

You can't create your own functions in {{< param "PRODUCT_NAME" >}} configuration files.

If a function fails during evaluation, {{< param "PRODUCT_NAME" >}} stops processing the expression and reports an error.
This prevents invalid data from propagating through your configuration.

## Standard library functions

The {{< param "PRODUCT_NAME" >}} configuration syntax includes a [standard library][] of functions.
These functions fall into several categories:

- **System functions**: Interact with the host system, like reading environment variables
- **Encoding functions**: Transform data between different formats like JSON and YAML
- **String functions**: Manipulate text values
- **Array functions**: Work with lists of values

### Common examples

The following examples show frequently used standard library functions:

```alloy
// Get environment variables
log_level = sys.env("LOG_LEVEL")

// Parse JSON from a file
config = encoding.from_json(local.file.cfg.content)

// Extract specific values from parsed data
namespace = encoding.from_json(local.file.cfg.content)["namespace"]

// Format strings
message = string.format("Hello, %s!", username)
```

## Component export functions

Components can export functions that other components can call.
This lets you create reusable logic and share computed values across your configuration.

For example, a custom component might export a function that validates input data or transforms metrics.

## Next steps

Learn more about:

- [Standard library reference][standard library] for complete function documentation
- [Types and values][] to understand function inputs and outputs
- [Referencing exports][] to use functions from components

[standard library]: ../../../../reference/stdlib/
[Types and values]: ./types_and_values/
[Referencing exports]: ./referencing_exports/

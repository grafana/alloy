---
canonical: https://Grafana.com/docs/alloy/latest/get-started/expressions/
aliases:
  - ./configuration-syntax/expressions/ # /docs/alloy/latest/get-started/configuration-syntax/expressions/
description: Learn about expressions and functions in Alloy
title: Expressions and functions
weight: 30
---

# Expressions and functions

Expressions compute values you can assign to component attributes in an {{< param "PRODUCT_NAME" >}} configuration.
You can use them to make your configuration dynamic and connect different components together.

This section covers how to write expressions, from basic literal values to complex computations that reference other components.

## What are expressions

Basic expressions are literal values, like `"Hello, world!"` or `true`.
Expressions can also [refer to values][] exported by components, perform arithmetic, or [call functions][].

You use expressions to configure any component.
All component arguments have an underlying [type][].
{{< param "PRODUCT_NAME" >}} checks the expression type before assigning the result to an attribute.

## Common expression types

The most common expressions you'll use are:

- **Literal values**: Simple constants like `"debug"`, `8080`, or `true`
- **Component references**: Values from other components like `local.file.config.content`
- **Function calls**: Built-in functions like `sys.env("HOME")` or `json_decode(local.file.data.content)`
- **Arithmetic**: Mathematical operations like `1 + 2` or `port_base + offset`

## Next steps

Learn how to write different types of expressions:

- [Reference component exports][refer to values] to connect components together
- [Call functions][call functions] to transform and compute values
- [Understand types and values][type] to work with different data types

[refer to values]: ./referencing_exports/
[call functions]: ./function_calls/
[type]: ./types_and_values/

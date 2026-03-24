---
canonical: https://grafana.com/docs/alloy/latest/get-started/expressions/
aliases:
  - ./configuration-syntax/expressions/ # /docs/alloy/latest/get-started/configuration-syntax/expressions/
  - ./concepts/configuration-syntax/expressions/ # /docs/alloy/latest/concepts/configuration-syntax/expressions/
description: Learn about expressions and functions in Alloy
title: Expressions
weight: 40
---

# Expressions

You learned about components and how they work together in pipelines in the previous sections.
Now you'll learn about _expressions_. These are the mechanism that makes your configurations dynamic by computing values and connecting components together.

Expressions are the key to {{< param "PRODUCT_NAME" >}}'s flexibility.
They let you reference data from other components, transform values using functions, and create dynamic configurations that respond to changing conditions.

## How expressions work

Expressions compute values that you assign to component arguments.
The component controller evaluates expressions when components start and whenever their dependencies change.

Basic expressions are literal values like `"Hello, world!"` or `true`.
More powerful expressions can reference component exports, call functions, or perform calculations.

All component arguments have an underlying [type][].
{{< param "PRODUCT_NAME" >}} checks the expression type during evaluation to ensure compatibility before assigning the result to an attribute.
This type checking prevents configuration errors and ensures your pipelines work correctly.

## Expression types

The four main types of expressions you'll use are:

1. **Literal values**: Constants like `"debug"`, `8080`, or `true`.
1. **Component references**: Values from other components like `local.file.config.content`.
1. **Function calls**: Built-in functions like `sys.env("HOME")` or `encoding.from_json(local.file.data.content)`.
1. **Arithmetic operations**: Mathematical calculations like `1 + 2` or `port_base + offset`.

You can combine these types to create complex expressions that transform data and connect components in sophisticated ways.

## Next steps

Now that you understand how expressions work, learn to write different types of expressions:

- [Component exports][refer to values] - Reference data from other components to connect your pipeline
- [Types and values][type] - Understand data types and how they work with expressions

For advanced expression features:

- [Function calls][call functions] - Transform and compute values using built-in functions
- [Operators][] - Combine and manipulate values using mathematical and logical operators

[refer to values]: ./referencing_exports/
[call functions]: ./function_calls/
[type]: ./types_and_values/
[Operators]: ./operators/

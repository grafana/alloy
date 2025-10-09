---
canonical: https://Grafana.com/docs/alloy/latest/get-started/expressions/
aliases:
  - ./configuration-syntax/expressions/ # /docs/alloy/latest/get-started/configuration-syntax/expressions/
description: Learn about expressions and functions in Alloy
title: Expressions and functions
weight: 30
---

# Expressions and functions

Expressions represent or compute values you can assign to attributes in a configuration.

Basic expressions are literal values, like `"Hello, world!"` or `true`.
Expressions can also [refer to values][] exported by components, perform arithmetic, or [call functions][].

You use expressions to configure any component.
All component arguments have an underlying [type][].
{{< param "PRODUCT_NAME" >}} checks the expression type before assigning the result to an attribute.

[refer to values]: ./referencing_exports/
[call functions]: ./function_calls/
[type]: ./types_and_values/

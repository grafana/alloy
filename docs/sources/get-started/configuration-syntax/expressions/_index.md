---
canonical: https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/expressions/
aliases:
  - ../../concepts/configuration-syntax/expressions/ # /docs/alloy/latest/concepts/configuration-syntax/expressions/
description: Learn about expressions
title: Expressions
weight: 400
---

# Expressions

Expressions represent or compute values you can assign to attributes within a configuration.

Basic expressions are literal values, like `"Hello, world!"` or `true`.
Expressions may also do things like [refer to values][] exported by components, perform arithmetic, or [call functions][].

You use expressions when you configure any component.
All component arguments have an underlying [type][].
{{< param "PRODUCT_NAME" >}} checks the expression type before assigning the result to an attribute.

[refer to values]: ./referencing_exports/
[call functions]: ./function_calls/
[type]: ./types_and_values/

---
aliases:
- ../../configuration-language/expressions/operators/ # /docs/agent/latest/flow/concepts/configuration-language/expressions/operators/
- /docs/grafana-cloud/agent/flow/concepts/config-language/expressions/operators/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/config-language/expressions/operators/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/config-language/expressions/operators/
- /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/expressions/operators/
# Previous page aliases for backwards compatibility:
- ../../../configuration-language/expressions/operators/ # /docs/agent/latest/flow/configuration-language/expressions/operators/
- /docs/grafana-cloud/agent/flow/config-language/expressions/operators/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/config-language/expressions/operators/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/config-language/expressions/operators/
- /docs/grafana-cloud/send-data/agent/flow/config-language/expressions/operators/
canonical: https://grafana.com/docs/agent/latest/flow/concepts/config-language/expressions/operators/
description: Learn about operators
title: Operators
weight: 300
---

# Operators

River uses a common set of operators.
All operations follow the standard [PEMDAS][] order of mathematical operations.

## Arithmetic operators

Operator | Description
---------|---------------------------------------------------
`+`      | Adds two numbers.
`-`      | Subtracts two numbers.
`*`      | Multiplies two numbers.
`/`      | Divides two numbers.
`%`      | Computes the remainder after dividing two numbers.
`^`      | Raises the number to the specified power.

## String operators

Operator | Description
---------|-------------------------
`+`      | Concatenate two strings.

## Comparison operators

Operator | Description
---------|---------------------------------------------------------------------
`==`     | `true` when two values are equal.
`!=`     | `true` when two values aren't equal.
`<`      | `true` when the left value is less than the right value.
`<=`     | `true` when the left value is less than or equal to the right value.
`>`      | `true` when the left value is greater than the right value.
`>=`     | `true` when the left value is greater or equal to the right value.

You can apply the equality operators `==` and `!=` to any operands.

The two operands in ordering operators `<` `<=` `>` and `>=`  must both be _orderable_ and of the same type.
The results of the comparisons are:

* Boolean values are equal if they're either both true or both false.
* Numerical (integer and floating-point) values are orderable in the usual way.
* String values are orderable lexically byte-wise.
* Objects are equal if all their fields are equal.
* Array values are equal if their corresponding elements are equal.

## Logical operators

Operator | Description
---------|---------------------------------------------------------
`&&`     | `true` when the both left _and_ right value are `true`.
`\|\|`   | `true` when the either left _or_ right value are `true`.
`!`      | Negates a boolean value.

Logical operators apply to boolean values and yield a boolean result.

## Assignment operator

River uses `=` as its assignment operator.

An assignment statement may only assign a single value.
Each value must be _assignable_ to the attribute or object key.

* You can assign `null` to any attribute.
* You can assign numerical, string, boolean, array, function, capsule, and object types to attributes of the corresponding type.
* You can assign numbers to string attributes with an implicit conversion.
* You can assign strings to numerical attributes if they represent a number.
* You can't assign blocks.

## Brackets

Brackets | Description
---------|------------------------------------
`{ }`    | Defines blocks and objects.
`( )`    | Groups and prioritizes expressions.
`[ ]`    | Defines arrays.

The following example uses curly braces and square brackets to define an object and an array.

```river
obj = { app = "agent", namespace = "dev" }
arr = [1, true, 7 * (1+1), 3]
```

## Access operators

Operator | Description
---------|------------------------------------------------------------------------
`[ ]`    | Access a member of an array or object.
`.`      | Access a named member of an object or an exported field of a component.

You can access arbitrarily nested values with River's access operators.
You can use square brackets to access zero-indexed array indices and object fields by enclosing the field name in double quotes.
You can use the dot operator to access object fields without double quotes and component exports.

```river
obj["app"]
arr[1]

obj.app
local.file.token.content
```

If you use the `[ ]` operator to access a member of an object where the member doesn't exist, the resulting value is `null`.

If you use the `.` operator to access a named member of an object where the named member doesn't exist, an error is generated.

[PEMDAS]: https://en.wikipedia.org/wiki/Order_of_operations

---
canonical: https://grafana.com/docs/alloy/latest/get-started/expressions/operators/
aliases:
  - ./configuration-syntax/expressions/operators/ # /docs/alloy/latest/get-started/configuration-syntax/expressions/operators/
description: Learn about operators
title: Operators
weight: 30
---

# Operators

The {{< param "PRODUCT_NAME" >}} configuration syntax uses a standard set of operators.
All operations follow the [PEMDAS][] order of mathematical operations.

## Arithmetic operators

| Operator | Description                                        |
| -------- | -------------------------------------------------- |
| `+`      | Adds two numbers.                                  |
| `-`      | Subtracts two numbers.                             |
| `*`      | Multiplies two numbers.                            |
| `/`      | Divides two numbers.                               |
| `%`      | Computes the remainder after dividing two numbers. |
| `^`      | Raises a number to the specified power.            |

## String operators

| Operator | Description                                                       |
| -------- | ----------------------------------------------------------------- |
| `+`      | Concatenate two strings or two secrets, or a string and a secret. |

## Comparison operators

| Operator | Description                                                                     |
| -------- | ------------------------------------------------------------------------------- |
| `==`     | Returns `true` when two values are equal.                                       |
| `!=`     | Returns `true` when two values aren't equal.                                    |
| `<`      | Returns `true` when the left value is less than the right value.                |
| `<=`     | Returns `true` when the left value is less than or equal to the right value.    |
| `>`      | Returns `true` when the left value is greater than the right value.             |
| `>=`     | Returns `true` when the left value is greater than or equal to the right value. |

You can use the equality operators `==` and `!=` with any operands.

The operands in ordering operators `<`, `<=`, `>`, and `>=` must be _orderable_ and of the same type.
The results of these comparisons are:

- Boolean values are equal if both are either `true` or `false`.
- Numerical (integer and floating-point) values are ordered in the usual way.
- String values are ordered lexically, byte-wise.
- Objects are equal if all their fields are equal.
- Arrays are equal if their corresponding elements are equal.

## Logical operators

| Operator | Description                                                      |
| -------- | ---------------------------------------------------------------- |
| `&&`     | Returns `true` when both the left _and_ right values are `true`. |
| `\|\|`   | Returns `true` when either the left _or_ right value is `true`.  |
| `!`      | Negates a boolean value.                                         |

Logical operators work with boolean values and return a boolean result.

## Assignment operator

The {{< param "PRODUCT_NAME" >}} configuration syntax uses `=` as the assignment operator.

An assignment statement can only assign a single value.
Each value must be _assignable_ to the attribute or object key.

- You can assign `null` to any attribute.
- You can assign numerical, string, boolean, array, function, capsule, and object types to attributes of the corresponding type.
- You can assign numbers to string attributes with an implicit conversion.
- You can assign strings to numerical attributes if they represent a number.
- You can't assign blocks.

## Brackets

| Brackets | Description                       |
| -------- | --------------------------------- |
| `{ }`    | Define blocks and objects.        |
| `( )`    | Group and prioritize expressions. |
| `[ ]`    | Define arrays.                    |

For example, the following code uses curly braces and square brackets to define an object and an array.

```alloy
obj = { app = "alloy", namespace = "dev" }
arr = [1, true, 7 * (1+1), 3]
```

## Access operators

| Operator | Description                                                             |
| -------- | ----------------------------------------------------------------------- |
| `[ ]`    | Access a member of an array or object.                                  |
| `.`      | Access a named member of an object or an exported field of a component. |

You can use the {{< param "PRODUCT_NAME" >}} access operators to retrieve nested values.
Use square brackets to access zero-indexed array elements or object fields by enclosing the field name in double quotes.
Use the dot operator to access object fields without double quotes or component exports.

```alloy
obj["app"]
arr[1]

obj.app
local.file.token.content
```

If you use the `[ ]` operator to access a non-existent object member, the result is `null`.

If you use the `.` operator to access a non-existent named member of an object, an error occurs.

## Next steps

Learn how to apply operators in expressions:

- [Types and values][] to understand what data types work with different operators
- [Function calls][] to use operators in function arguments and calculations  
- [Reference component exports][] to build expressions that connect components

[Types and values]: ./types_and_values/
[Function calls]: ./function_calls/
[Reference component exports]: ./referencing_exports/
[PEMDAS]: https://en.wikipedia.org/wiki/Order_of_operations

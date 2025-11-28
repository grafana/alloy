---
canonical: https://grafana.com/docs/alloy/latest/get-started/expressions/operators/
aliases:
  - ./configuration-syntax/expressions/operators/ # /docs/alloy/latest/get-started/configuration-syntax/expressions/operators/
  - ./concepts/configuration-syntax/expressions/operators/ # /docs/alloy/latest/concepts/configuration-syntax/expressions/operators/
description: Learn about operators
title: Operators
weight: 30
---

# Operators

You learned how to reference component exports to connect your pipelines in the previous topic.
Now you'll learn how to use operators to manipulate, transform, and combine values in your expressions.

Operators are essential for creating dynamic configurations that can:

1. **Calculate values**: Perform arithmetic operations on component exports and literals.
1. **Compare data**: Make decisions based on component states or configuration values.
1. **Combine strings**: Build dynamic identifiers, paths, or configuration strings.
1. **Access nested data**: Extract values from complex objects and arrays.

The component controller evaluates all operators following the standard [PEMDAS][] order of mathematical operations.

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

String concatenation is particularly useful for building dynamic values from component exports:

```alloy
prometheus.scrape "app" {
  targets = [{ "__address__" = local.file.host.content + ":" + local.file.port.content }]
  job_name = "app-" + sys.env("ENVIRONMENT")
}
```

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

1. Boolean values are equal if both are either `true` or `false`.
1. Numerical (integer and floating-point) values follow the usual ordering.
1. String values follow lexical ordering, byte-wise.
1. Objects are equal if all their fields are equal.
1. Arrays are equal if their corresponding elements are equal.

Comparison operators are commonly used for conditional configuration:

```alloy
// Check if environment matches
remote_write_enabled = sys.env("ENVIRONMENT") == "production"
```

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
Each value must be _assignable_ to the attribute or object key:

1. You can assign `null` to any attribute.
1. You can assign numerical, string, boolean, array, function, capsule, and object types to attributes of the corresponding type.
1. You can assign numbers to string attributes with an implicit conversion.
1. You can assign strings to numerical attributes if they represent a number.
1. You can't assign blocks.

The component controller performs type checking during evaluation to ensure assignments are valid before configuring components.

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
// Access array elements and object fields
obj["app"]
arr[1]

// Access object fields and component exports
obj.app
local.file.token.content
discovery.kubernetes.endpoints.targets[0]["__address__"]

// Build complex expressions with nested access
prometheus.scrape "dynamic" {
  targets = discovery.kubernetes.endpoints.targets
  job_name = discovery.kubernetes.endpoints.targets[0]["job"] + "-scraper"
}
```

If you use the `[ ]` operator to access a non-existent object member, the result is `null`.

If you use the `.` operator to access a non-existent named member of an object, an error occurs.

## Next steps

Now that you understand operators, continue learning about expressions:

- [Types and values][type] - Learn how the type system works with operators to ensure expressions evaluate correctly
- [Function calls][functions] - Use operators inside function calls and with function return values

For building complete configurations:

- [Component exports][refer to values] - Combine operators with component references to create dynamic pipelines

[type]: ./types_and_values/
[functions]: ./function_calls/
[refer to values]: ./referencing_exports/
[PEMDAS]: https://en.wikipedia.org/wiki/Order_of_operations

---
canonical: https://grafana.com/docs/alloy/latest/get-started/expressions/types_and_values/
aliases:
  - ../configuration-syntax/expressions/types_and_values/ # /docs/alloy/latest/get-started/configuration-syntax/expressions/types_and_values/
  - ./concepts/configuration-syntax/expressions/types_and_values/ # /docs/alloy/latest/concepts/configuration-syntax/expressions/types_and_values/
description: Learn about the Alloy syntax types and values
title: Types and values
weight: 10
---

# Types and values

You learned about the main types of expressions in the previous section. These are literals, component references, functions, and arithmetic operations.
Now you'll learn about the _types and values_ that these expressions work with and how {{< param "PRODUCT_NAME" >}} uses them to ensure your configurations work correctly.

Understanding types helps you write expressions that work reliably and helps you understand why certain combinations of components and values are compatible while others aren't.

## Value types

The {{< param "PRODUCT_NAME" >}} syntax supports the following value types:

- `number`: Any numeric value, such as `3` or `3.14`.
- `string`: A sequence of Unicode characters representing text, such as `"Hello, world!"`.
- `bool`: A boolean value, either `true` or `false`.
- `array`: A sequence of values, such as `[1, 2, 3]`. Index array elements using whole numbers starting from zero.
- `object`: A group of values identified by named labels, such as `{ name = "John" }`.
- `function`: A value representing a routine that runs with arguments to compute another value, such as `sys.env("HOME")`.
  Functions take zero or more arguments as input and always return a single value as output.
- `null`: A type that represents no value.

## Names and naming conventions

In addition to the preceding types, the [component reference][] documentation uses the following conventions for referring to types:

- `any`: A value of any type.
- `map(T)`: An `object` where all values are of type `T`.
  For example, `map(string)` is an object where all values are strings.
  The key type of an object is always a string or an identifier converted into a string.
- `list(T)`: An `array` where all values are of type `T`.
  For example, `list(string)` is an array where all values are strings.
- `duration`: A `string` that denotes a duration of time, such as `"100ms"`, `"1h30m"`, or `"10s"`.
  Valid units include:

  - `h` for hours.
  - `m` for minutes.
  - `s` for seconds.
  - `ms` for milliseconds.
  - `ns` for nanoseconds.

  You can combine values of descending units to add their values together.
  For example, `"1h30m"` is equivalent to `"90m"`.

## Numbers

The {{< param "PRODUCT_NAME" >}} syntax treats integers, unsigned integers, and floating-point values as a single `number` type.
This simplifies writing and reading {{< param "PRODUCT_NAME" >}} configuration files.

```text
3    == 3.00     // true
5.0  == (10 / 2) // true
1e+2 == 100      // true
2e-3 == 0.002    // true
```

## Strings

Strings are sequences of Unicode characters enclosed in double quotes `""`.

```alloy
"Hello, world!"
```

A `\` in a string starts an escape sequence to represent a special character.
The following table lists the supported escape sequences.

| Sequence     | Replacement                                                                             |
| ------------ | --------------------------------------------------------------------------------------- |
| `\\`         | The `\` character `U+005C`                                                              |
| `\a`         | The alert or bell character `U+0007`                                                    |
| `\b`         | The backspace character `U+0008`                                                        |
| `\f`         | The form feed character `U+000C`                                                        |
| `\n`         | The newline character `U+000A`                                                          |
| `\r`         | The carriage return character `U+000D`                                                  |
| `\t`         | The horizontal tab character `U+0009`                                                   |
| `\v`         | The vertical tab character `U+000B`                                                     |
| `\'`         | The `'` character `U+0027`                                                              |
| `\"`         | The `"` character `U+0022`, which prevents terminating the string                       |
| `\NNN`       | A literal byte (NNN is three octal digits)                                              |
| `\xNN`       | A literal byte (NN is two hexadecimal digits)                                           |
| `\uNNNN`     | A Unicode character from the basic multilingual plane (NNNN is four hexadecimal digits) |
| `\UNNNNNNNN` | A Unicode character from supplementary planes (NNNNNNNN is eight hexadecimal digits)    |

## Raw strings

Raw strings are sequences of Unicode characters enclosed in backticks ` `` `.
Raw strings don't support escape sequences.

```alloy
`Hello, "world"!`
```

Within backticks, any character can appear except a backtick.
Include a backtick by concatenating a double-quoted string containing a backtick using `+`.

{{< param "PRODUCT_NAME" >}} interprets a multiline raw string exactly as written.

```alloy
`Hello,
"world"!`
```

{{< param "PRODUCT_NAME" >}} interprets the preceding multiline raw string as a string with the following value.

```string
Hello,
"world"!
```

## Boolean values

The symbols `true` and `false` represent boolean values.

## Arrays

Construct arrays using a sequence of comma-separated values enclosed in square brackets `[]`.

```alloy
[0, 1, 2, 3]
```

You can place values on separate lines for readability.
Include a comma after the final value if the closing bracket `]` is on a different line.

```alloy
[
  0,
  1,
  2,
]
```

## Objects

Construct objects using a sequence of comma-separated key-value pairs enclosed in curly braces `{}`.

```alloy
{
  first_name = "John",
  last_name  = "Doe",
}
```

Include a comma after the final key-value pair if the closing curly brace `}` is on a different line.

```alloy
{ name = "John" }
```

Wrap keys in double quotes if they aren't [valid identifiers][valid].

```alloy
{
  "app.kubernetes.io/name"     = "mysql",
  "app.kubernetes.io/instance" = "mysql-abcxyz",
  namespace                    = "default",
}
```

{{< admonition type="note" >}}
Don't confuse objects with blocks.

- An _object_ is a value assigned to an [Attribute][]. Use commas between key-value pairs on separate lines.
- A [Block][] is a named structural element composed of multiple attributes. Don't use commas between attributes.

[Attribute]: ../../syntax/#attributes
[Block]: ../../syntax/#blocks

{{< /admonition >}}

## Functions

You can't construct function values.
You can call functions from the standard library or export them from a component.

## Null

The symbol `null` represents the null value.

## Special types

### Secrets

A `secret` is a special type of string that's never displayed to the user.
You can assign `string` values to an attribute expecting a `secret`, but not the inverse.
You can use [`convert.nonsensitive`][nonsensitive] to convert a secret to a string.
You can't assign a secret to an attribute expecting a string.

### Capsules

A `capsule` is a special type that represents a category of _internal_ types used by {{< param "PRODUCT_NAME" >}}.
Each capsule type has a unique name and appears as `capsule("<SOME_INTERNAL_NAME>")`.
You can't construct capsule values.
Use capsules in expressions like any other type.
Capsules aren't inter-compatible.
An attribute expecting a capsule can only accept a capsule of the same internal type.
If an attribute expects a `capsule("prometheus.Receiver")`, you can only assign a `capsule("prometheus.Receiver")` type.
Components that use or export capsules document the specific capsule type they expect.

In the following example, the `prometheus.remote_write` component exports a `receiver`, which is a `capsule("prometheus.Receiver")` type.
You can use this capsule in the `forward_to` attribute of `prometheus.scrape`, which expects an array of `capsule("prometheus.Receiver")`.

```alloy
prometheus.remote_write "default" {
  endpoint {
    url = "http://localhost:9090/api/v1/write"
  }
}

prometheus.scrape "default" {
  targets    = [/* ... */]
  forward_to = [prometheus.remote_write.default.receiver]
}
```

## Next steps

Now that you understand types and values, learn how to use them in expressions:

- [Component exports][] - Reference data from other components and understand how types ensure compatibility
- [Operators][] - Learn how operators work with different value types and handle type compatibility

For advanced expression features:

- [Function calls][] - Transform data using standard library functions that work with these types

[component reference]: ../../../reference/components/
[valid]: ../../syntax#identifiers
[nonsensitive]: ../../../reference/stdlib/convert/
[Component exports]: ../referencing_exports/
[Operators]: ../operators/
[Function calls]: ../function_calls/

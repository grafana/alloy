---
aliases:
- ../../configuration-language/expressions/types-and-values/ # /docs/agent/latest/flow/concepts/configuration-language/expressions/types-and-values/
- /docs/grafana-cloud/agent/flow/concepts/config-language/expressions/types_and_values/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/config-language/expressions/types_and_values/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/config-language/expressions/types_and_values/
- /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/expressions/types_and_values/
# Previous page aliases for backwards compatibility:
- ../../../configuration-language/expressions/types-and-values/ # /docs/agent/latest/flow/configuration-language/expressions/types-and-values/
- /docs/grafana-cloud/agent/flow/config-language/expressions/types_and_values/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/config-language/expressions/types_and_values/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/config-language/expressions/types_and_values/
- /docs/grafana-cloud/send-data/agent/flow/config-language/expressions/types_and_values/
canonical: https://grafana.com/docs/agent/latest/flow/concepts/config-language/expressions/types_and_values/
description: Learn about the River types and values
title: Types and values
weight: 100
---

# Types and values

River uses the following types for its values:

* `number`: Any numeric value, like `3` or `3.14`.
* `string`: A sequence of Unicode characters representing text, like `"Hello, world!"`.
* `bool`: A boolean value, either `true` or `false`.
* `array`: A sequence of values, like `[1, 2, 3]`. Elements within the list are indexed by whole numbers, starting with zero.
* `object`: A group of values identified by named labels, like `{ name = "John" }`.
* `function`: A value representing a routine that runs with arguments to compute another value, like `env("HOME")`.
  Functions take zero or more arguments as input and always return a single value as output.
* `null`: A type that has no value.

## Naming convention

In addition to the preceding types, the [component reference][] documentation uses the following conventions for referring to types:

* `any`: A value of any type.
* `map(T)`: an `object` with the value type `T`.
  For example, `map(string)` is an object where all the values are strings.
  The key type of an object is always a string or an identifier converted into a string.
* `list(T)`: an `array` with the value type`T`.
  For example, `list(string)` is an array where all the values are strings.
* `duration`: a `string` denoting a duration of time, such as `"1d"`, `"1h30m"`, `"10s"`.
  Valid units are:

  * `d` for days.
  * `h` for hours.
  * `m` for minutes.
  * `s` for seconds.
  * `ms` for milliseconds.
  * `ns` for nanoseconds.

  You can combine values of descending units to add their values together. For example, `"1h30m"` is the same as `"90m"`.

## Numbers

River handles integers, unsigned integers, and floating-point values as a single 'number' type, simplifying writing and reading River configuration files.

```river
3    == 3.00     // true
5.0  == (10 / 2) // true
1e+2 == 100      // true
2e-3 == 0.002    // true
```

## Strings

Strings are represented by sequences of Unicode characters surrounded by double quotes `""`.

```river
"Hello, world!"
```

A `\` in a string starts an escape sequence to represent a special character.
The following table shows the supported escape sequences.

| Sequence     | Replacement                                                                             |
|--------------|-----------------------------------------------------------------------------------------|
| `\\`         | The `\` character `U+005C`                                                              |
| `\a`         | The alert or bell character `U+0007`                                                    |
| `\b`         | The backspace character `U+0008`                                                        |
| `\f`         | The formfeed character `U+000C`                                                         |
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

Raw strings are represented by sequences of Unicode characters surrounded by backticks ``` `` ```.
Raw strings don't support any escape sequences.

```river
`Hello, "world"!`
```

Within the backticks, any character may appear except a backtick.
You can include a backtick by concatenating a double-quoted string that contains a backtick using `+`.

A multiline raw string is interpreted exactly as written.

```river
`Hello,
"world"!`
```

The preceding multiline raw string is interpreted as a string with the following value.

```string
Hello,
"world"!
```

## Bools

Bools are represented by the symbols `true` and `false`.

## Arrays

You construct arrays with a sequence of comma-separated values surrounded by square brackets `[]`.

```river
[0, 1, 2, 3]
```

You can place values in array elements on separate lines for readability.
A comma after the final value must be present if the closing bracket `]` is on a different line than the final value.

```river
[
  0,
  1,
  2,
]
```

## Objects

You construct objects with a sequence of comma-separated key-value pairs surrounded by curly braces `{}`.

```river
{
  first_name = "John",
  last_name  = "Doe",
}
```

You can omit the comma after the final key-value pair if the closing curly brace `}` is on the same line as the final pair.

```river
{ name = "John" }
```

If the key isn't a valid identifier, you must wrap it in double quotes like a string.

```river
{
  "app.kubernetes.io/name"     = "mysql",
  "app.kubernetes.io/instance" = "mysql-abcxyz",
  namespace                    = "default",
}
```

{{< admonition type="note" >}}
Don't confuse objects with blocks.

* An _object_ is a value assigned to an [Attribute][]. You **must** use commas between key-value pairs on separate lines.
* A [Block][] is a named structural element composed of multiple attributes. You **must not** use commas between attributes.

[Attribute]: {{< relref "../syntax.md#Attributes" >}}
[Block]: {{< relref "../syntax.md#Blocks" >}}
{{< /admonition >}}

## Functions

You can't construct function values. You can call functions from the standard library or export them from a component.

## Null

The null value is represented by the symbol `null`.

## Special types

#### Secrets

A `secret` is a special type of string that's never displayed to the user.
You can assign `string` values to an attribute expecting a `secret`, but never the inverse.
It's impossible to convert a secret to a string or assign a secret to an attribute expecting a string.

#### Capsules

A `capsule` is a special type that represents a category of _internal_ types used by {{< param "PRODUCT_NAME" >}}.
Each capsule type has a unique name and is represented to the user as `capsule("<SOME_INTERNAL_NAME>")`.
You can't construct capsule values. You can use capsules in expressions as any other type.
Capsules aren't inter-compatible, and an attribute expecting a capsule can only be given a capsule of the same internal type.
If an attribute expects a `capsule("prometheus.Receiver")`, you can only assign a `capsule("prometheus.Receiver")` type.
The specific type of capsule expected is explicitly documented for any component that uses or exports them.

In the following example, the `prometheus.remote_write` component exports a `receiver`, which is a `capsule("prometheus.Receiver")` type.
You can use this capsule in the `forward_to` attribute of `prometheus.scrape`, which expects an array of `capsule("prometheus.Receiver")`.

```river
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

{{% docs/reference %}}
[type]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components"
[type]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components"
{{% /docs/reference %}}
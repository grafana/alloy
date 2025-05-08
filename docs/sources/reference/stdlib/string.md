---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/string/
description: Learn about string functions
aliases:
  - ./format/ # /docs/alloy/latest/reference/stdlib/format/
  - ./join/ # /docs/alloy/latest/reference/stdlib/join/
  - ./replace/ # /docs/alloy/latest/reference/stdlib/replace/
  - ./split/ # /docs/alloy/latest/reference/stdlib/split/
  - ./to_lower/ # /docs/alloy/latest/reference/stdlib/to_lower/
  - ./to_upper/ # /docs/alloy/latest/reference/stdlib/to_upper/
  - ./trim/ # /docs/alloy/latest/reference/stdlib/trim/
  - ./trim_prefix/ # /docs/alloy/latest/reference/stdlib/trim_prefix/
  - ./trim_suffix/ # /docs/alloy/latest/reference/stdlib/trim_suffix/
  - ./trim_space/ # /docs/alloy/latest/reference/stdlib/trim_space/
menuTitle: string
title: string
---

# string

The `string` namespace contains functions related to strings.

## string.format

The `string.format` function produces a string by formatting a number of other values according to a specification string.
It's similar to the `printf` function in C, and other similar functions in other programming languages.

```alloy
string.format(spec, values...)
```

### Examples

```alloy
> string.format("Hello, %s!", "Ander")
"Hello, Ander!"
> string.format("There are %d lights", 4)
"There are 4 lights"
```

The `string.format` function is most useful when you use more complex format specifications.

### Specification Syntax

The specification is a string that includes formatting verbs that are introduced with the `%` character.
The function call must then have one additional argument for each verb sequence in the specification.
The verbs are matched with consecutive arguments and formatted as directed, as long as each given argument is convertible to the type required by the format verb.

By default, `%` sequences consume successive arguments starting with the first.
Introducing a `[n]` sequence immediately before the verb letter, where `n` is a decimal integer, explicitly chooses a particular value argument by its one-based index.
Subsequent calls without an explicit index will then proceed with `n`+1, `n`+2, etc.

The function produces an error if the format string requests an impossible conversion or accesses more arguments than are given.
An error is also produced for an unsupported format verb.

#### Verbs

The specification may contain the following verbs.

| Verb | Result                                                                                    |
|------|-------------------------------------------------------------------------------------------|
| `%%` | Literal percent sign, consuming no value.                                                 |
| `%t` | Convert to boolean and produce `true` or `false`.                                         |
| `%b` | Convert to integer number and produce binary representation.                              |
| `%d` | Convert to integer and produce decimal representation.                                    |
| `%o` | Convert to integer and produce octal representation.                                      |
| `%x` | Convert to integer and produce hexadecimal representation with lowercase letters.         |
| `%X` | Like `%x`, but use uppercase letters.                                                     |
| `%e` | Convert to number and produce scientific notation, like `-1.234456e+78`.                  |
| `%E` | Like `%e`, but use an uppercase `E` to introduce the exponent.                            |
| `%f` | Convert to number and produce decimal fraction notation with no exponent, like `123.456`. |
| `%g` | Like `%e` for large exponents or like `%f` otherwise.                                     |
| `%G` | Like `%E` for large exponents or like `%f` otherwise.                                     |
| `%s` | Convert to string and insert the string's characters.                                     |
| `%q` | Convert to string and produce a JSON quoted string representation.                        |

When using the `string.format` function with a [`secret`][] value, you must first convert it to a non-sensitive string using the [`convert.nonsensitive`][] function.
If the resulting value must be a [`secret`][], you can use string concatenation with the `+` operator instead of the `string.format` function.

## string.join

`string.join` all items in an array into a string, using a character as separator.

```alloy
string.join(list, separator)
```

### Examples

```alloy
> string.join(["foo", "bar", "baz"], "-")
"foo-bar-baz"
> string.join(["foo", "bar", "baz"], ", ")
"foo, bar, baz"
> string.join(["foo"], ", ")
"foo"
```

## string.replace

`string.replace` searches a string for a substring, and replaces each occurrence of the substring with a replacement string.

```alloy
string.replace(string, substring, replacement)
```

### Examples

```alloy
> string.replace("1 + 2 + 3", "+", "-")
"1 - 2 - 3"
```

## string.split

`string.split` produces a list by dividing a string at all occurrences of a separator.

```alloy
split(list, separator)
```

### Examples

```alloy
> string.split("foo,bar,baz", "," )
["foo", "bar", "baz"]

> string.split("foo", ",")
["foo"]

> string.split("", ",")
[""]
```

## string.to_lower

`string.to_lower` converts all uppercase letters in a string to lowercase.

### Examples

```alloy
> string.to_lower("HELLO")
"hello"
```

## string.to_upper

`string.to_upper` converts all lowercase letters in a string to uppercase.

### Examples

```alloy
> string.to_upper("hello")
"HELLO"
```

## string.trim

`string.trim` removes the specified set of characters from the start and end of a string.

```alloy
string.trim(string, str_character_set)
```

### Examples

```alloy
> string.trim("?!hello?!", "!?")
"hello"

> string.trim("foobar", "far")
"oob"

> string.trim("   hello! world.!  ", "! ")
"hello! world."
```

## string.trim_prefix

`string.trim_prefix` removes the prefix from the start of a string.
If the string doesn't start with the prefix, the string is returned unchanged.

### Examples

```alloy
> string.trim_prefix("helloworld", "hello")
"world"
```
## string.trim_suffix

`string.trim_suffix` removes the suffix from the end of a string.

### Examples

```alloy
> string.trim_suffix("helloworld", "world")
"hello"
```

## string.trim_space

`string.trim_space` removes any whitespace characters from the start and end of a string.

### Examples

```alloy
> string.trim_space("  hello\n\n")
"hello"
```

[`secret`]: ../../../get-started/configuration-syntax/expressions/types_and_values/#secrets
[`convert.nonsensitive`]: ../convert/#nonsensitive
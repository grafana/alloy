---
aliases:
- ../../configuration-language/standard-library/format/
- /docs/grafana-cloud/agent/flow/reference/stdlib/format/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/stdlib/format/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/stdlib/format/
- /docs/grafana-cloud/send-data/agent/flow/reference/stdlib/format/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/format/
description: Learn about format
title: format
---

# format

The `format` function produces a string by formatting a number of other values according
to a specification string. It is similar to the `printf` function in C, and
other similar functions in other programming languages.

```river
format(spec, values...)
```

## Examples

```river
> format("Hello, %s!", "Ander")
"Hello, Ander!"
> format("There are %d lights", 4)
"There are 4 lights"
```

The `format` function is most useful when you use more complex format specifications.

## Specification Syntax

The specification is a string that includes formatting verbs that are introduced
with the `%` character. The function call must then have one additional argument
for each verb sequence in the specification. The verbs are matched with
consecutive arguments and formatted as directed, as long as each given argument
is convertible to the type required by the format verb.

By default, `%` sequences consume successive arguments starting with the first.
Introducing a `[n]` sequence immediately before the verb letter, where `n` is a
decimal integer, explicitly chooses a particular value argument by its
one-based index. Subsequent calls without an explicit index will then proceed
with `n`+1, `n`+2, etc.

The function produces an error if the format string requests an impossible
conversion or accesses more arguments than are given. An error is also produced
for an unsupported format verb.

### Verbs

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

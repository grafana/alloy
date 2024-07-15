---
canonical: https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/syntax/
aliases:
  - ../../concepts/configuration-syntax/syntax/ # /docs/alloy/latest/concepts/configuration-syntax/syntax/
description: Learn about the Alloy syntax
title: Syntax
weight: 999
---

# Syntax

The {{< param "PRODUCT_NAME" >}} syntax is easy to read and write. It has only two high-level elements, _Attributes_ and _Blocks_.

The {{< param "PRODUCT_NAME" >}} configuration syntax is a _declarative_ language used to build programmable pipelines.
The order of blocks and attributes within the {{< param "PRODUCT_NAME" >}} configuration file isn't important.
The language considers all direct and indirect dependencies between elements to determine their relationships.

## Comments

{{< param "PRODUCT_NAME" >}} configuration files support single-line `//` and block `/* */` comments.

## Identifiers

{{< param "PRODUCT_NAME" >}} syntax considers an identifier as valid if it consists of one or more UTF-8 letters (A through Z, both upper- and lower-case), digits or underscores, but doesn't start with a digit.

## Attributes and Blocks

### Attributes

You use _Attributes_ to configure individual settings.
They always take the form of `ATTRIBUTE_NAME = ATTRIBUTE_VALUE`.
They can appear either as top-level elements or nested within blocks.

The following example sets the `log_level` attribute to `"debug"`.

```alloy
log_level = "debug"
```

The `ATTRIBUTE_NAME` must be a valid {{< param "PRODUCT_NAME" >}} [identifier][].

The `ATTRIBUTE_VALUE` can be either a constant value of a valid {{< param "PRODUCT_NAME" >}} [type][] (for example, a string, boolean, number), or an [_expression_][expression] to represent or compute more complex attribute values.

### Blocks

You use _Blocks_ to configure the {{< param "PRODUCT_NAME" >}}'s behavior as well as {{< param "PRODUCT_NAME" >}} components by grouping any number of attributes or nested blocks using curly braces.
Blocks have a _name_, an optional _label_ and a body that contains any number of arguments and nested unlabeled blocks.

Some blocks can be defined more than once.

#### Examples

You can use the following pattern to create an unlabeled block.

```alloy
BLOCK_NAME {
  // Block body can contain attributes and nested unlabeled blocks
  IDENTIFIER = EXPRESSION // Attribute

  NESTED_BLOCK_NAME {
    // Nested block body
  }
}
```

You can use the following pattern to create a labeled block

```alloy
// Pattern for creating a labeled block:
BLOCK_NAME "BLOCK_LABEL" {
  // Block body can contain attributes and nested unlabeled blocks
  IDENTIFIER = EXPRESSION // Attribute

  NESTED_BLOCK_NAME {
    // Nested block body
  }
}
```

#### Block naming rules

The `BLOCK_NAME` has to be recognized by {{< param "PRODUCT_NAME" >}} as either a valid component name or a special block for configuring global settings.
If the `BLOCK_LABEL` must be set, it must be a valid {{< param "PRODUCT_NAME" >}} [identifier][] wrapped in double quotes.
In these cases, you use the label to disambiguate between multiple top-level blocks of the same name.

The following snippet defines a block named `local.file` with its label set to "token".
The block's body sets `filename` to the content of the `TOKEN_FILE_PATH` environment variable by using an expression, and the `is_secret` attribute is set to the boolean `true`, marking the file content as sensitive.

```alloy
local.file "token" {
  filename  = env("TOKEN_FILE_PATH") // Use an expression to read from an env var.
  is_secret = true
}
```

## Terminators

All block and attribute definitions are followed by a newline, which {{< param "PRODUCT_NAME" >}} calls a _terminator_, as it terminates the current statement.

A newline is treated as a terminator when it follows any expression, `]`, `)`, or `}`.
{{< param "PRODUCT_NAME" >}} ignores other newlines and you can enter as many newlines as you want.

[identifier]: #identifiers
[identifier]: #identifiers
[expression]: ../expressions/
[type]: ../expressions/types_and_values/

---
canonical: https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/syntax/
aliases:
  - ../../concepts/configuration-syntax/syntax/ # /docs/alloy/latest/concepts/configuration-syntax/syntax/
description: Learn about the Alloy syntax
title: Syntax
weight: 999
---

# Syntax

The {{< param "PRODUCT_NAME" >}} syntax is easy to read and write.
It has two main elements: _Attributes_ and _Blocks_.

The {{< param "PRODUCT_NAME" >}} configuration syntax is a _declarative_ language for creating programmable pipelines.
The order of blocks and attributes in the {{< param "PRODUCT_NAME" >}} configuration file doesn't matter.
The language evaluates all dependencies between elements to determine their relationships.

## Comments

{{< param "PRODUCT_NAME" >}} configuration files support single-line `//` comments and block `/* */` comments.

## Identifiers

An identifier in {{< param "PRODUCT_NAME" >}} syntax is valid if it contains one or more UTF-8 letters (A through Z, upper- or lowercase), digits, or underscores.
It can't start with a digit.

## Attributes and Blocks

### Attributes

Use _Attributes_ to configure individual settings.
Attributes follow the format `ATTRIBUTE_NAME = ATTRIBUTE_VALUE`.
They can appear as top-level elements or nested within blocks.

The following example sets the `log_level` attribute to `"debug"`.

```alloy
log_level = "debug"
```

The `ATTRIBUTE_NAME` must be a valid {{< param "PRODUCT_NAME" >}} [identifier][].

The `ATTRIBUTE_VALUE` can be a constant value of a valid {{< param "PRODUCT_NAME" >}} [type][], such as a string, boolean, or number.
It can also be an [_expression_][expression] to compute complex values.

### Blocks

Use _Blocks_ to configure the behavior of {{< param "PRODUCT_NAME" >}} and its components by grouping attributes or nested blocks with curly braces.
Blocks have a _name_, an optional _label_, and a body containing arguments and nested unlabeled blocks.

Some blocks can be defined multiple times.

#### Examples

Use the following pattern to create an unlabeled block.

```alloy
BLOCK_NAME {
  // Block body can contain attributes and nested unlabeled blocks
  IDENTIFIER = EXPRESSION // Attribute

  NESTED_BLOCK_NAME {
    // Nested block body
  }
}
```

Use the following pattern to create a labeled block.

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

The `BLOCK_NAME` must be a valid component name or a special block for configuring global settings in {{< param "PRODUCT_NAME" >}}.
If required, the `BLOCK_LABEL` must be a valid {{< param "PRODUCT_NAME" >}} [identifier][] wrapped in double quotes.
Use the label to distinguish between multiple top-level blocks with the same name.

The following snippet defines a block named `local.file` with the label "token."
The block's body sets `filename` to the content of the `TOKEN_FILE_PATH` environment variable using an expression.
The `is_secret` attribute is set to `true`, marking the file content as sensitive.

```alloy
local.file "token" {
  filename  = sys.env("TOKEN_FILE_PATH") // Use an expression to read from an env var.
  is_secret = true
}
```

## Terminators

All block and attribute definitions end with a newline, which {{< param "PRODUCT_NAME" >}} calls a _terminator_.
A newline acts as a terminator when it follows any expression, `]`, `)`, or `}`.
{{< param "PRODUCT_NAME" >}} ignores other newlines, so you can add as many as you want.

[identifier]: #identifiers
[expression]: ../expressions/
[type]: ../expressions/types_and_values/

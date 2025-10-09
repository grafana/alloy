---
canonical: https://grafana.com/docs/alloy/latest/get-started/syntax/
aliases:
  - ./configuration-syntax/syntax/ # /docs/alloy/latest/get-started/configuration-syntax/syntax/
description: Learn about the Alloy configuration language syntax
title: Configuration language syntax
weight: 20
---

# Configuration language syntax

The {{< param "PRODUCT_NAME" >}} syntax helps you create readable and maintainable configurations.
It has two main elements: _Attributes_ and _Blocks_.

{{< param "PRODUCT_NAME" >}} uses a _declarative_ language.
This means you describe what you want, not how to do it.
{{< param "PRODUCT_NAME" >}} figures out the steps to create your data pipelines.

The order of blocks and attributes in your configuration file doesn't matter.
The language evaluates all dependencies between elements to determine their relationships.

## Comments

{{< param "PRODUCT_NAME" >}} configuration files support single-line `//` comments and block `/* */` comments.

```alloy
// This is a single-line comment

/*
This is a block comment
that can span multiple lines
*/
```

## Identifiers

An identifier in {{< param "PRODUCT_NAME" >}} syntax is valid if it contains one or more UTF-8 letters from A through Z, upper- or lowercase, digits, or underscores.
It can't start with a digit.

Valid identifiers:

- `my_component`
- `Component123`
- `_private`

Invalid identifiers:

- `123component` - starts with digit
- `my-component` - contains hyphen
- `my component` - contains space

## Attributes and Blocks

### Attributes

Attributes configure individual settings.
They follow the format `ATTRIBUTE_NAME = ATTRIBUTE_VALUE`.
You can place them as top-level elements or nested within blocks.

This example sets the `log_level` attribute to `"debug"`:

```alloy
log_level = "debug"
```

The `ATTRIBUTE_NAME` must be a valid {{< param "PRODUCT_NAME" >}} [identifier][].

The `ATTRIBUTE_VALUE` can be:

- A constant value of a valid {{< param "PRODUCT_NAME" >}} [type][], such as a string, boolean, or number
- An [_expression_][expression] to compute complex values

### Blocks

Blocks configure {{< param "PRODUCT_NAME" >}} and its components.
They group attributes or nested blocks using curly braces.
Blocks have a _name_, an optional _label_, and a body containing arguments and nested unlabeled blocks.

Some blocks can appear multiple times in your configuration.

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

The `BLOCK_NAME` must be either:

- A valid component name
- A special block for configuring global settings in {{< param "PRODUCT_NAME" >}}

If required, the `BLOCK_LABEL` must be a valid {{< param "PRODUCT_NAME" >}} [identifier][] wrapped in double quotes.
Labels help distinguish between multiple top-level blocks with the same name.

This snippet defines a block named `local.file` with the label "token":

```alloy
local.file "token" {
  filename  = sys.env("TOKEN_FILE_PATH") // Use an expression to read from an env var.
  is_secret = true
}
```

The block's body sets `filename` to the content of the `TOKEN_FILE_PATH` environment variable using an expression.
The configuration sets the `is_secret` attribute to `true`, marking the file content as sensitive.

## Terminators

All block and attribute definitions end with a newline.
{{< param "PRODUCT_NAME" >}} calls this a _terminator_.
A newline acts as a terminator when it follows any expression, `]`, `)`, or `}`.
{{< param "PRODUCT_NAME" >}} ignores other newlines, so you can add as many as you want for readability.

```alloy
// This is valid - newlines are ignored
local.file "example" {
  filename = "/path/to/file"


  is_secret = true
}
```

## Formatting

{{< param "PRODUCT_NAME" >}} provides a built-in formatter to ensure consistent code style.
Use the [`alloy fmt` command][fmt] to format your configuration files.

The formatter:

- Standardizes indentation
- Removes unnecessary whitespace
- Ensures consistent line endings
- Validates syntax

## Next steps

To continue learning about {{< param "PRODUCT_NAME" >}} configuration syntax:

- Learn about [expressions][expression] to create dynamic configurations and computations
- Explore [components][] to understand how to configure data collection and processing
- Read the [configuration language reference][reference] for comprehensive syntax documentation
- Use the [`alloy fmt` command][fmt] to automatically format your configuration files

[identifier]: #identifiers
[expression]: ../expressions/
[type]: ../expressions/types_and_values/
[fmt]: ../../reference/cli/fmt/
[components]: ../components/
[reference]: ../../reference/config-language/

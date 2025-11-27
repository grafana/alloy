---
canonical: https://grafana.com/docs/alloy/latest/get-started/syntax/
aliases:
  - ../configuration-syntax/syntax/ # /docs/alloy/latest/get-started/configuration-syntax/syntax/
description: Learn about the Alloy configuration language syntax
title: Alloy syntax
menuTitle: Alloy syntax
weight: 50
---

# {{% param "PRODUCT_NAME" %}} syntax

You learned about components, expressions, and basic configuration elements in the previous sections.
Now you'll learn the detailed syntax rules that govern how to write {{< param "PRODUCT_NAME" >}} configuration files.

Understanding the syntax ensures your configurations are:

1. **Correctly parsed**: Follow rules that the {{< param "PRODUCT_NAME" >}} parser expects.
1. **Maintainable**: Use consistent patterns and formatting.
1. **Readable**: Structure code clearly for yourself and your team.
1. **Error-free**: Avoid common syntax mistakes that prevent successful evaluation.

The {{< param "PRODUCT_NAME" >}} configuration syntax is _declarative_, meaning you describe what you want rather than how to achieve it.
The parser evaluates all dependencies between elements automatically, so the order of blocks and attributes doesn't matter.

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

An identifier in {{< param "PRODUCT_NAME" >}} syntax is valid if it contains one or more UTF-8 letters, digits, or underscores.
Identifiers can't start with a digit.

The parser uses identifiers for:

1. **Attribute names**: Keys in key-value pairs.
1. **Component names**: Parts of block names like `prometheus` in `prometheus.scrape`.
1. **Component labels**: User-defined names to distinguish component instances.
1. **Variable references**: Names used in expressions and function calls.

Valid identifiers:

- `my_component`
- `Component123`
- `_private`
- `métrica_café` (Unicode letters are supported)

Invalid identifiers:

- `123component` (starts with digit)
- `my-component` (contains hyphen)
- `my component` (contains space)
- `my.component` (contains dot, which has special meaning)

## Attributes and Blocks

{{< param "PRODUCT_NAME" >}} configurations are built using two main syntax elements: attributes and blocks.
Attributes set individual values, while blocks group related configuration and create component instances.

### Attributes

Attributes configure individual settings using the format `ATTRIBUTE_NAME = ATTRIBUTE_VALUE`.
The parser evaluates attributes during component configuration to determine runtime behavior.

You can place attributes as:

1. **Top-level settings**: Global {{< param "PRODUCT_NAME" >}} configuration.
1. **Component arguments**: Settings within component blocks.
1. **Nested block settings**: Configuration within nested blocks.

Examples of different attribute contexts:

```alloy
// Top-level attribute
log_level = "debug"

// Component block with attributes
prometheus.scrape "app" {
  targets    = [{ "__address__" = "localhost:9090" }]
  scrape_interval = "15s"

  // Nested block with attributes
  basic_auth {
    username = "admin"
    password = sys.env("ADMIN_PASSWORD")
  }
}
```

The `ATTRIBUTE_NAME` must be a valid {{< param "PRODUCT_NAME" >}} [identifier][].

The `ATTRIBUTE_VALUE` can be:

1. **Constant values**: Strings, numbers, booleans, arrays, or objects.
1. **Expressions**: Function calls, component references, or calculations that compute dynamic values.

### Blocks

Blocks configure {{< param "PRODUCT_NAME" >}} components and organize related settings using curly braces.
The parser processes blocks to create and configure component instances.

Block structure:

1. **Block name**: Identifies the component type (required).
1. **Block label**: User-defined identifier to distinguish multiple instances (optional).
1. **Block body**: Contains attributes and nested blocks (required).

Some blocks can appear multiple times in your configuration when they have different labels:

```alloy
// Multiple instances of the same component type
prometheus.scrape "frontend" {
  targets = [{ "__address__" = "frontend:8080" }]
}

prometheus.scrape "backend" {
  targets = [{ "__address__" = "backend:9090" }]
}
```

Nested blocks provide structured configuration:

```alloy
prometheus.remote_write "production" {
  endpoint {
    url = "https://prometheus.example.com/api/v1/write"

    basic_auth {
      username = "metrics"
      password = local.file.credentials.content
    }
  }

  queue {
    capacity = 10000
    max_samples_per_send = 2000
  }
}
```

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

The parser enforces specific rules for block names and labels:

**Block names** must be either:

1. **Valid component names**: Dot-separated identifiers like `prometheus.scrape` or `local.file`.
1. **Special configuration blocks**: Built-in blocks for global settings like `logging` or `tracing`.

**Block labels** (when required) must be:

1. **Valid identifiers**: Follow the same rules as attribute names.
1. **Double-quoted strings**: Wrapped in double quotes, not single quotes.
1. **Unique within scope**: No two blocks of the same type can have identical labels.

Example showing proper block naming:

```alloy
// Component name: "local.file", Label: "api_key"
local.file "api_key" {
  filename  = sys.env("API_KEY_PATH")
  is_secret = true
}

// Component name: "prometheus.scrape", Label: "web_servers"
prometheus.scrape "web_servers" {
  targets = discovery.kubernetes.services.targets
  metrics_path = "/metrics"
}
```

The parser validates that:

1. **Component names exist**: The component type must be available in {{< param "PRODUCT_NAME" >}}.
1. **Labels are unique**: Within the same component type.
1. **Syntax is correct**: Follows the identifier and quoting rules.

## Terminators

The parser requires terminators to separate statements and determine where expressions end.
All block and attribute definitions must end with a newline, which {{< param "PRODUCT_NAME" >}} calls a _terminator_.

A newline acts as a terminator when it follows:

1. **Complete expressions**: After any value, calculation, or function call.
1. **Closing delimiters**: After `]`, `)`, or `}`.
1. **Statement endings**: After attribute assignments or block definitions.

The parser ignores newlines in other contexts, allowing flexible formatting:

```alloy
// This formatting is valid - extra newlines are ignored
local.file "example" {
  filename = "/path/to/file"


  is_secret = true


  // Comments can also have extra spacing
}

// Expressions can span multiple lines
targets = [
  { "__address__" = "server1:9090" },
  { "__address__" = "server2:9090" },
  { "__address__" = "server3:9090" }
]
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

Now that you understand the syntax fundamentals, learn how to use these elements to build working configurations:

- [Components][] - Learn about the building blocks that collect, transform, and send data
- [Expressions][] - Create dynamic configurations using functions and component references

For advanced configuration techniques:

- [Configuration language reference][reference] - Comprehensive syntax documentation and advanced features
- [Formatting][fmt] - Use `alloy fmt` to automatically format your configuration files

[Expressions]: ./expressions/
[Components]: ./components/
[reference]: ../reference/config-language/
[fmt]: ../reference/cli/fmt/
[identifier]: #identifiers

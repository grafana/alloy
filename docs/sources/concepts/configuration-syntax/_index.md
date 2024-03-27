---
canonical: https://grafana.com/docs/alloy/latest/concepts/configuration-syntax/
description: Learn about the Alloy configuration syntax
title: Alloy configuration syntax
menuTitle: Configuration syntax
weight: 10
---

# {{% param "PRODUCT_NAME" %>}} configuration syntax

{{< param "FULL_PRODUCT_NAME" >}} dynamically configures and connects components with the {{< param "PRODUCT_NAME" >}} configuration syntax.

The {{< param "PRODUCT_NAME" >}} syntax aims to reduce errors in configuration files by making configurations easier to read and write.
{{< param "PRODUCT_NAME" >}} configurations use blocks that can be easily copied and pasted from the documentation to help you get started as quickly as possible.

An {{< param "PRODUCT_NAME" >}} configuration file tells {{< param "PRODUCT_NAME" >}} which components to launch and how to bind them together into a pipeline.

The {{< param "PRODUCT_NAME" >}} syntax uses blocks, attributes, and expressions.

```river
// Create a local.file component labeled my_file.
// This can be referenced by other components as local.file.my_file.
local.file "my_file" {
  filename = "/tmp/my-file.txt"
}

// Pattern for creating a labeled block, which the above block follows:
BLOCK_NAME "BLOCK_LABEL" {
  // Block body
  IDENTIFIER = EXPRESSION // Attribute
}

// Pattern for creating an unlabeled block:
BLOCK_NAME {
  // Block body
  IDENTIFIER = EXPRESSION // Attribute
}
```

[{{< param "PRODUCT_NAME" >}} is designed][RFC] with the following requirements in mind:

* _Fast_: The configuration language must be fast so the component controller can quickly evaluate changes.
* _Simple_: The configuration language must be easy to read and write to minimize the learning curve.
* _Debuggable_: The configuration language must give detailed information when there's a mistake in the configuration file.

The {{< param "PRODUCT_NAME" >}} configuration syntax is similar to HCL, the language Terraform and other Hashicorp projects use.
It's a distinct language with custom syntax and features, such as first-class functions.

* Blocks are a group of related settings and usually represent creating a component.
  Blocks have a name that consists of zero or more identifiers separated by `.`, an optional user label, and a body containing attributes and nested blocks.
* Attributes appear within blocks and assign a value to a name.
* Expressions represent a value, either literally or by referencing and combining other values.
  You use expressions to compute a value for an attribute.

The {{< param "PRODUCT_NAME" >}} syntax is declarative, so ordering components, blocks, and attributes within a block isn't significant.
The relationship between components determines the order of operations.

## Attributes

You use _Attributes_ to configure individual settings.
Attributes always take the form of `ATTRIBUTE_NAME = ATTRIBUTE_VALUE`.

The following example shows how to set the `log_level` attribute to `"debug"`.

```river
log_level = "debug"
```

## Expressions

You use expressions to compute the value of an attribute.
The simplest expressions are constant values like `"debug"`, `32`, or `[1, 2, 3, 4]`.
The {{< param "PRODUCT_NAME" >}} syntax supports complex expressions, for example:

* Referencing the exports of components: `local.file.password_file.content`
* Mathematical operations: `1 + 2`, `3 * 4`, `(5 * 6) + (7 + 8)`
* Equality checks: `local.file.file_a.content == local.file.file_b.content`
* Calling functions from {{< param "PRODUCT_NAME" >}}'s standard library: `env("HOME")` retrieves the value of the `HOME` environment variable.

You can use expressions for any attribute inside a component definition.

### Referencing component exports

The most common expression is to reference the exports of a component, for example, `local.file.password_file.content`.
You form a reference to a component's exports by merging the component's name (for example, `local.file`),
label (for example, `password_file`), and export name (for example, `content`), delimited by a period.

## Blocks

You use _Blocks_ to configure components and groups of attributes.
Each block can contain any number of attributes or nested blocks.

```river
prometheus.remote_write "default" {
  endpoint {
    url = "http://localhost:9009/api/prom/push"
  }
}
```

The preceding example has two blocks:

* `prometheus.remote_write "default"`: A labeled block which instantiates a `prometheus.remote_write` component.
  The label is the string `"default"`.
* `endpoint`: An unlabeled block inside the component that configures an endpoint to send metrics to.
  This block sets the `url` attribute to specify the endpoint.


## Tooling

You can use one or all of the following tools to help you write {{< param "PRODUCT_NAME" >}} configuration files.

* Editor support for:
  * [VSCode](https://github.com/grafana/vscode-alloy)
  * [Vim/Neovim](https://github.com/grafana/vim-alloy)
* Code formatting using the [`agent fmt` command][fmt]

[RFC]: https://github.com/grafana/agent/blob/97a55d0d908b26dbb1126cc08b6dcc18f6e30087/docs/rfcs/0005-river.md
[fmt]: ../../reference/cli/fmt/

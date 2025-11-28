---
canonical: https://grafana.com/docs/alloy/latest/get-started/
aliases:
  - ./configuration-syntax/ # /docs/alloy/latest/get-started/configuration-syntax/
  - ./configuration-syntax/files/ # /docs/alloy/latest/get-started/configuration-syntax/files/
  - ./concepts/ # /docs/alloy/latest/concepts/
  - ./concepts/configuration-syntax/ # /docs/alloy/latest/concepts/configuration-syntax/
  - ./concepts/configuration-syntax/files/ # /docs/alloy/latest/concepts/configuration-syntax/files/
description: Get started with Grafana Alloy
title: Get started with Grafana Alloy
menuTitle: Get started
weight: 20
---

# Get started with {{% param "FULL_PRODUCT_NAME" %}}

{{< param "FULL_PRODUCT_NAME" >}} uses a configuration language to define how components collect, transform, and send data.
Components are building blocks that perform specific tasks, such as reading files, collecting metrics, or sending data to external systems.

To write effective configurations, you need to understand three fundamental elements: blocks, attributes, and expressions.
Mastering these building blocks lets you create powerful data collection and processing pipelines.

## Basic configuration elements

All {{< param "PRODUCT_NAME" >}} configurations use three main elements: blocks, attributes, and expressions.

## Blocks

Blocks group related settings and configure different parts of {{< param "PRODUCT_NAME" >}}.
Each block has a name and contains attributes or nested blocks.

```alloy
prometheus.remote_write "production" {
  endpoint {
    url = "http://localhost:9009/api/prom/push"
  }
}
```

This example contains two blocks:

- `prometheus.remote_write "production"`: Creates a component with the label `"production"`
- `endpoint`: A nested block that configures connection settings

## Attributes

Attributes set individual values within blocks.
They follow the format `ATTRIBUTE_NAME = ATTRIBUTE_VALUE`.

```alloy
log_level = "debug"
timeout = 30
enabled = true
```

## Expressions

Expressions compute values for attributes.
You can use simple constants or more complex calculations.

**Constants:**

```alloy
name = "my-service"
port = 9090
tags = ["web", "api"]
```

**Simple calculations:**

You can use arithmetic operations to compute values from other variables.
This lets you build dynamic configurations where values depend on other settings.

```alloy
total_timeout = base_timeout + retry_timeout
```

**Function calls:**

Function calls let you access system information and transform data.
[Built-in][] functions like `sys.env()` retrieve environment variables, while others can manipulate strings, decode JSON, and perform other operations.

```alloy
home_dir = sys.env("HOME")
config_path = home_dir + "/config.yaml"
```

**Component references:**

Component references let you use data from other parts of your configuration.
To reference a component's data, combine three parts with periods:

- Component name: `local.file`
- Label: `secret`
- Export name: `content`
- Result: `local.file.secret.content`

```alloy
password = local.file.secret.content
```

You'll learn about more powerful expressions in the dedicated [Expressions][] section, including how to reference data from other components and use more built-in functions.
You can find the available exports for each component in the [Components][components] documentation.

## Configuration syntax

{{< param "PRODUCT_NAME" >}} uses a declarative configuration language, which means you describe what you want your system to do rather than how to do it.
This design makes configurations flexible and easy to understand.

You can organize blocks and attributes in any order that makes sense for your use case.
{{< param "PRODUCT_NAME" >}} automatically determines the dependencies between components and evaluates them in the correct order.

## Configuration files

{{< param "PRODUCT_NAME" >}} configuration files conventionally use a `.alloy` file extension, though you can name single files anything you want.
If you specify a directory path, {{< param "PRODUCT_NAME" >}} processes only files with the `.alloy` extension.
You must save your configuration files as UTF-8 encoded text - {{< param "PRODUCT_NAME" >}} can't parse files with invalid UTF-8 encoding.

## Tooling

You can use these tools to write {{< param "PRODUCT_NAME" >}} configuration files:

- Editor support:
  - [VS Code](https://github.com/grafana/vscode-alloy)
  - [Vim/Neovim](https://github.com/grafana/vim-alloy)
- Code formatting: [`alloy fmt` command][fmt]

## Next steps

Now that you understand the basic syntax, learn how to use these elements to build working configurations:

- [Components][] - Learn about the building blocks that collect, transform, and send data
- [Expressions][] - Create dynamic configurations using functions and component references
- [Alloy syntax][] - Explore advanced syntax features and patterns

For hands-on learning:

- [Tutorials][] - Build complete data collection pipelines step by step
- [Components][components] - Browse all available components and their options

[fmt]: ../../reference/cli/fmt/
[Built-in]: ../reference/stdlib/
[Alloy syntax]: ./syntax/
[Components]: ./components/
[Expressions]: ./expressions/
[tutorials]: ../tutorials/
[components]: ../reference/components/

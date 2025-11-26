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

This configuration sets up logging for {{< param "PRODUCT_NAME" >}}:

```alloy
logging {
    level  = "info"
    format = "json"
}
```

This example shows the basic elements:

- **Block**: `logging` defines a configuration section
- **Attributes**: `level` and `format` are settings within the block
- **Values**: `"info"` and `"json"` are the assigned values

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
You can use simple constants or complex calculations.

**Constants:**

```alloy
name = "my-service"
port = 9090
tags = ["web", "api"]
```

**Function calls:**

```alloy
home_dir = sys.env("HOME")
config_path = home_dir + "/config.yaml"
```

**Component references:**

```alloy
password = local.file.secret.content
```

Component references let you use data from other parts of your configuration.
To reference a component's data, combine three parts with periods:

- Component name: `local.file`
- Label: `secret`
- Export name: `content`
- Result: `local.file.secret.content`

## Configuration files

{{< param "PRODUCT_NAME" >}} configuration files use a `.alloy` file extension and must be UTF-8 encoded.
The syntax is declarative, which means the order of blocks and attributes doesn't matter.

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
- [Configuration syntax][] - Explore advanced syntax features and patterns

For hands-on learning:

- [Tutorials][] - Build complete data collection pipelines step by step
- [Component reference][] - Browse all available components and their options

[fmt]: ../../reference/cli/fmt/
[components]: ./components/
[expressions]: ./expressions/
[Configuration syntax]: ./syntax/
[tutorials]: ../tutorials/
[Component reference]: ../reference/components/

{{< section >}}

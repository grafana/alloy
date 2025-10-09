---
canonical: https://Grafana.com/docs/alloy/latest/get-started/
aliases:
  - ./configuration-syntax/ # /docs/alloy/latest/get-started/configuration-syntax/
  - ./configuration-syntax/files/ # /docs/alloy/latest/get-started/configuration-syntax/files/
description: Get started with Grafana Alloy configuration
title: Get started with Grafana Alloy configuration
menuTitle: Get started
weight: 10
---

# Get started with {{% param "FULL_PRODUCT_NAME" %}} configuration

{{< param "FULL_PRODUCT_NAME" >}} uses a configuration language to connect and manage components.
Components are building blocks that collect, transform, and send your data.
Each component performs a specific task, such as reading files, collecting metrics, or sending data to external systems.

{{< figure src="/media/docs/alloy/flow-diagram-small-alloy.png" alt="Alloy flow diagram" >}}

Before exploring complex pipelines, start with basic concepts.
Understanding these building blocks helps you create effective configurations.

## Basic concepts

This configuration sets up logging for {{< param "PRODUCT_NAME" >}}:

```alloy
logging {
    level  = "info"
    format = "json"
}
```

This example shows the basic elements:

- **Block**: `logging` configures logging behavior
- **Attributes**: `level` and `format` are settings within the block
- **Values**: `"info"` and `"json"` are the assigned values

## Connect components

Components can work together to create data pipelines.
This example reads a configuration file and uses its content:

```alloy
local.file "config" {
    filename = "/etc/app/settings.txt"
}

logging {
    level = local.file.config.content
}
```

Here, the `logging` block uses data from the `local.file` component.
The expression `local.file.config.content` references the file's content.

## Build your first pipeline

Now you can combine multiple components to build a data pipeline.
This example shows how to collect and forward log data:

```alloy
// Read log files from a directory
loki.source.file "app_logs" {
    targets = [{"__path__" = "/var/log/app/*.log"}]
    forward_to = [loki.write.grafana_cloud.receiver]
}

// Send logs to Grafana Cloud
loki.write "grafana_cloud" {
    endpoint {
        url = "https://logs-prod-us-central1.grafana.net/loki/api/v1/push"
    }
}
```

This pipeline has two steps:

1. `loki.source.file` reads log files from a directory
1. `loki.write` sends the logs to Grafana Cloud

The `forward_to` attribute connects the components together.

## Complete pipeline example

For more complex use cases, you can build longer pipelines that transform data.
This example demonstrates a complete log processing pipeline:

```alloy
// Collection: mount a local directory with a certain path spec
local.file_match "applogs" {
    path_targets = [{"__path__" = "/tmp/app-logs/app.log"}]
}

// Collection: Take the file match as input, and scrape those mounted log files
loki.source.file "local_files" {
    targets    = local.file_match.applogs.targets

    // This specifies which component should process the logs next, the "link in the chain"
    forward_to = [loki.process.add_new_label.receiver]
}

// Transformation: pull some data out of the log message, and turn it into a label
loki.process "add_new_label" {
    stage.logfmt {
        mapping = {
            "extracted_level" = "level",
        }
    }

    // Add the value of "extracted_level" from the extracted map as a "level" label
    stage.labels {
        values = {
            "level" = "extracted_level",
        }
    }

    // The next link in the chain is the local_loki "receiver" (receives the telemetry)
    forward_to = [loki.write.local_loki.receiver]
}

// Anything that comes into this component gets written to the loki remote API
loki.write "local_loki" {
    endpoint {
        url = "http://loki:3100/loki/api/v1/push"
    }
}
```

This pipeline shows how components work together:

1. **Collection**: `local.file_match` finds log files to read
1. **Processing**: `loki.source.file` reads the files and forwards logs to the next component
1. **Transformation**: `loki.process` extracts data from log messages and adds labels
1. **Output**: `loki.write` sends the processed logs to a Loki server

## Why use {{< param "PRODUCT_NAME" >}}?

The {{< param "PRODUCT_NAME" >}} syntax makes configurations easier to read and write.
It uses blocks, attributes, and expressions that you can copy from the documentation.

The {{< param "PRODUCT_NAME" >}} syntax is declarative.
This means the order of components, blocks, and attributes doesn't matter.
The relationships between components determine how the pipeline operates.

## Configuration files

{{< param "PRODUCT_NAME" >}} configuration files are plain text files with a `.alloy` file extension.
Refer to each {{< param "PRODUCT_NAME" >}} file as a "configuration file" or an "{{< param "PRODUCT_NAME" >}} configuration."

{{< param "PRODUCT_NAME" >}} configuration files must be UTF-8 encoded and support Unicode characters.
They can use Unix-style line endings (LF) or Windows-style line endings (CRLF).
Code formatting tools may replace all line endings with Unix-style ones.

## Blocks

Blocks group related settings and configure components.
Each block can include attributes or nested blocks.

```alloy
prometheus.remote_write "default" {
  endpoint {
    url = "http://localhost:9009/api/prom/push"
  }
}
```

This example contains two blocks:

- `prometheus.remote_write "default"`: Creates a `prometheus.remote_write` component with the label `"default"`
- `endpoint`: Configures where to send metrics and sets the `url` attribute

## Attributes

Attributes configure individual settings within blocks.
They follow the format `ATTRIBUTE_NAME = ATTRIBUTE_VALUE`.

This example sets the `log_level` attribute to `"debug"`:

```alloy
log_level = "debug"
```

## Expressions

Expressions compute values for attributes.
You can use constants like `"debug"`, `32`, or `[1, 2, 3, 4]`.

{{< param "PRODUCT_NAME" >}} also supports complex expressions:

- Reference component exports: `local.file.password_file.content`
- Mathematical operations: `1 + 2`, `3 * 4`, `(5 * 6) + (7 + 8)`
- Equality checks: `local.file.file_a.content == local.file.file_b.content`
- Function calls: `sys.env("HOME")` retrieves the `HOME` environment variable

You can use expressions for any attribute in a component definition.

### Reference component exports

The most common expression references a component's exports.
For example: `local.file.password_file.content`.

To create a reference, combine three parts with periods:

- Component name: `local.file`
- Label: `password_file`
- Export name: `content`
- Result: `local.file.password_file.content`

## Configuration syntax design goals

{{< param "PRODUCT_NAME" >}} has these design goals:

- **Fast**: The configuration language is fast and the controller evaluates changes quickly
- **Readable**: The configuration language is straightforward to read and write, reducing the learning curve
- **Easy to debug**: The configuration language provides detailed error information

The {{< param "PRODUCT_NAME" >}} configuration syntax is a distinct language with custom syntax and features, such as first-class functions.

Key elements:

- **Blocks** group related settings and typically represent component creation.
  Blocks have a name consisting of identifiers separated by `.`, an optional user label, and a body containing attributes and nested blocks.
- **Attributes** appear within blocks and assign values to names.
- **Expressions** represent values, either literally or by referencing and combining other values.
  You use expressions to compute attribute values.

## Tooling

You can use the following tools to write {{< param "PRODUCT_NAME" >}} configuration files:

- Editor support for:
  - [VS Code](https://github.com/grafana/vscode-alloy)
  - [Vim/Neovim](https://github.com/grafana/vim-alloy)
- Code formatting with the [`alloy fmt` command][fmt]

[fmt]: ../../reference/cli/fmt/

## Next steps

To continue learning about {{< param "PRODUCT_NAME" >}} configuration:

- Learn about [components][] to understand the building blocks of {{< param "PRODUCT_NAME" >}} pipelines
- Explore [expressions][] to create dynamic configurations that respond to runtime conditions
- Review [configuration syntax][] for detailed syntax reference and advanced features
- Try the [tutorials][] to build complete data collection pipelines
- Check the [component reference][] for available components and their configuration options

[components]: ./components/
[expressions]: ./expressions/
[configuration syntax]: ./configuration-syntax/
[tutorials]: ../tutorials/
[component reference]: ../reference/components/

{{< section >}}

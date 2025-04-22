---
canonical: https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/
aliases:
  - ../concepts/configuration-syntax/ # /docs/alloy/latest/concepts/configuration-syntax/
description: Learn about the Alloy configuration syntax
title: Alloy configuration syntax
menuTitle: Configuration syntax
weight: 10
---

# {{% param "PRODUCT_NAME" %}} configuration syntax

{{< param "FULL_PRODUCT_NAME" >}} dynamically configures and connects components using the {{< param "PRODUCT_NAME" >}} configuration syntax.
{{< param "PRODUCT_NAME" >}} collects, transforms, and delivers telemetry data.
Each configuration component performs a specific task or defines data flow and component connections.

{{< figure src="/media/docs/alloy/flow-diagram-small-alloy.png" alt="Alloy flow diagram" >}}

The following example demonstrates the basic concepts and how an {{< param "PRODUCT_NAME" >}} configuration forms a pipeline.

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

The {{< param "PRODUCT_NAME" >}} syntax reduces configuration errors by making files easier to read and write.
It uses blocks, attributes, and expressions.
You can copy and paste blocks from the documentation to get started quickly.

The {{< param "PRODUCT_NAME" >}} syntax is declarative, so the order of components, blocks, and attributes doesn't matter.
The relationships between components determine the pipeline's operation sequence.

## Blocks

Use _Blocks_ to configure components and groups of attributes.
Each block can include attributes or nested blocks.
Blocks represent steps in the pipeline.

```alloy
prometheus.remote_write "default" {
  endpoint {
    url = "http://localhost:9009/api/prom/push"
  }
}
```

The preceding example contains two blocks:

* `prometheus.remote_write "default"`: A labeled block that creates a `prometheus.remote_write` component.
  The label is the string `"default"`.
* `endpoint`: An unlabeled block inside the component that configures an endpoint for sending metrics.
  This block sets the `url` attribute to specify the endpoint.

## Attributes

Use _Attributes_ to configure individual settings.
Attributes follow the format `ATTRIBUTE_NAME = ATTRIBUTE_VALUE`.

The following example sets the `log_level` attribute to `"debug"`.

```alloy
log_level = "debug"
```

## Expressions

Use expressions to compute attribute values.
Simple expressions include constants like `"debug"`, `32`, or `[1, 2, 3, 4]`.
The {{< param "PRODUCT_NAME" >}} syntax supports complex expressions, such as:

* Referencing component exports: `local.file.password_file.content`
* Mathematical operations: `1 + 2`, `3 * 4`, `(5 * 6) + (7 + 8)`
* Equality checks: `local.file.file_a.content == local.file.file_b.content`
* Calling functions from the standard library: `sys.env("HOME")` retrieves the `HOME` environment variable.

You can use expressions for any attribute in a component definition.

### Reference component exports

The most common expression references a component's exports, such as `local.file.password_file.content`.
You form a reference by combining the component's name (for example, `local.file`), label (for example, `password_file`), and export name (for example, `content`), separated by periods.

## Configuration syntax design goals

{{< param "PRODUCT_NAME" >}} is:

* _Fast_: The configuration language is fast and the controller evaluates changes quickly.
* _Simple_: The configuration language is easy to read and write, reducing the learning curve.
* _Easy to debug_: The configuration language provides detailed error information.

The {{< param "PRODUCT_NAME" >}} configuration syntax is a distinct language with custom syntax and features, such as first-class functions.

* Blocks group related settings and typically represent component creation.
  Blocks have a name consisting of identifiers separated by `.`, an optional user label, and a body containing attributes and nested blocks.
* Attributes appear within blocks and assign values to names.
* Expressions represent values, either literally or by referencing and combining other values.
  You use expressions to compute attribute values.

## Tooling

You can use the following tools to write {{< param "PRODUCT_NAME" >}} configuration files:

* Editor support for:
  * [VSCode](https://github.com/grafana/vscode-alloy)
  * [Vim/Neovim](https://github.com/grafana/vim-alloy)
* Code formatting with the [`alloy fmt` command][fmt]

[fmt]: ../../reference/cli/fmt/

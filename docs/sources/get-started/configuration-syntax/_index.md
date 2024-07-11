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

{{< param "FULL_PRODUCT_NAME" >}} dynamically configures and connects components with the {{< param "PRODUCT_NAME" >}} configuration syntax.
{{< param "PRODUCT_NAME" >}} handles the collection, transformation, and delivery of telemetry data.
Each component in the configuration handles one of those tasks or specifies how data flows and how the components are bound together.

{{< figure src="/media/docs/alloy/flow-diagram-small-alloy.png" alt="Alloy flow diagram" >}}

The following simple example shows the basic concepts and how an {{< param "PRODUCT_NAME" >}} configuration comes together into a pipeline.

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

The {{< param "PRODUCT_NAME" >}} syntax aims to reduce errors in configuration files by making configurations easier to read and write.
The {{< param "PRODUCT_NAME" >}} syntax uses blocks, attributes, and expressions.
The blocks can be copied and pasted from the documentation to help you get started as quickly as possible.

The {{< param "PRODUCT_NAME" >}} syntax is declarative, so ordering components, blocks, and attributes does not matter.
The relationship between components determines the order of operations in the pipeline.

## Blocks

You use _Blocks_ to configure components and groups of attributes.
Each block can contain any number of attributes or nested blocks. 
Blocks are steps in the overall pipeline expressed by the configuration.

```alloy
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

## Attributes

You use _Attributes_ to configure individual settings.
Attributes always take the form of `ATTRIBUTE_NAME = ATTRIBUTE_VALUE`.

The following example shows how to set the `log_level` attribute to `"debug"`.

```alloy
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

## Configuration syntax design goals

{{< param "PRODUCT_NAME" >}} is:

* _Fast_: The configuration language is fast, so the component controller can quickly evaluate changes.
* _Simple_: The configuration language is easy to read and write to minimize the learning curve.
* _Easy to debug_: The configuration language gives detailed information when there's a mistake in the configuration file.

The {{< param "PRODUCT_NAME" >}} configuration syntax is a distinct language with custom syntax and features, such as first-class functions.

* Blocks are a group of related settings and usually represent creating a component.
  Blocks have a name that consists of zero or more identifiers separated by `.`, an optional user label, and a body containing attributes and nested blocks.
* Attributes appear within blocks and assign a value to a name.
* Expressions represent a value, either literally or by referencing and combining other values.
  You use expressions to compute a value for an attribute.

## Tooling

You can use one or all of the following tools to help you write {{< param "PRODUCT_NAME" >}} configuration files.

* Editor support for:
  * [VSCode](https://github.com/grafana/vscode-alloy)
  * [Vim/Neovim](https://github.com/grafana/vim-alloy)
* Code formatting using the [`alloy fmt` command][fmt]

[fmt]: ../../reference/cli/fmt/

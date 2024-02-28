---
aliases:
- /docs/grafana-cloud/agent/flow/concepts/config-language/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/config-language/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/config-language/
- /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/
- configuration-language/ # /docs/agent/latest/flow/concepts/configuration-language/
# Previous page aliases for backwards compatibility:
- /docs/grafana-cloud/agent/flow/config-language/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/config-language/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/config-language/
- /docs/grafana-cloud/send-data/agent/flow/config-language/
- ../configuration-language/ # /docs/agent/latest/flow/configuration-language/
- ../concepts/configuration_language/ # /docs/agent/latest/flow/concepts/configuration_language/
- /docs/grafana-cloud/agent/flow/concepts/configuration_language/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/configuration_language/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/configuration_language/
- /docs/grafana-cloud/send-data/agent/flow/concepts/configuration_language/
canonical: https://grafana.com/docs/agent/latest/flow/concepts/config-language/
description: Learn about the configuration language
title: Configuration language
weight: 10
---

# Configuration language

{{< param "PRODUCT_NAME" >}} dynamically configures and connects components with a custom configuration language called River.

River aims to reduce errors in configuration files by making configurations easier to read and write.
River configurations use blocks that can be easily copied and pasted from the documentation to help you get started as quickly as possible.

A River configuration file tells {{< param "PRODUCT_NAME" >}} which components to launch and how to bind them together into a pipeline.

The River syntax uses blocks, attributes, and expressions.

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

[River is designed][RFC] with the following requirements in mind:

* _Fast_: The configuration language must be fast so the component controller can quickly evaluate changes.
* _Simple_: The configuration language must be easy to read and write to minimize the learning curve.
* _Debuggable_: The configuration language must give detailed information when there's a mistake in the configuration file.

River is similar to HCL, the language Terraform and other Hashicorp projects use.
It's a distinct language with custom syntax and features, such as first-class functions.

* Blocks are a group of related settings and usually represent creating a component.
  Blocks have a name that consists of zero or more identifiers separated by `.`, an optional user label, and a body containing attributes and nested blocks.
* Attributes appear within blocks and assign a value to a name.
* Expressions represent a value, either literally or by referencing and combining other values.
  You use expressions to compute a value for an attribute.

River is declarative, so ordering components, blocks, and attributes within a block isn't significant.
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
River supports complex expressions, for example:

* Referencing the exports of components: `local.file.password_file.content`
* Mathematical operations: `1 + 2`, `3 * 4`, `(5 * 6) + (7 + 8)`
* Equality checks: `local.file.file_a.content == local.file.file_b.content`
* Calling functions from River's standard library: `env("HOME")` retrieves the value of the `HOME` environment variable.

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

You can use one or all of the following tools to help you write configuration files in River.

* Experimental editor support for
  * [vim](https://github.com/rfratto/vim-river)
  * [VSCode](https://github.com/rfratto/vscode-river)
  * [river-mode](https://github.com/jdbaldry/river-mode) for Emacs
* Code formatting using the [`agent fmt` command][fmt]

You can also start developing your own tooling using the {{< param "PRODUCT_ROOT_NAME" >}} repository as a go package or use the
[tree-sitter grammar][] with other programming languages.

[RFC]: https://github.com/grafana/agent/blob/97a55d0d908b26dbb1126cc08b6dcc18f6e30087/docs/rfcs/0005-river.md
[vim]: https://github.com/rfratto/vim-river
[VSCode]: https://github.com/rfratto/vscode-river
[river-mode]: https://github.com/jdbaldry/river-mode
[tree-sitter grammar]: https://github.com/grafana/tree-sitter-river

{{% docs/reference %}}
[fmt]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/cli/fmt"
[fmt]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/cli/fmt"
{{% /docs/reference %}}
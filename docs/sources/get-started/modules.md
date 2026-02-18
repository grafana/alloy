---
canonical: https://grafana.com/docs/alloy/latest/get-started/modules/
aliases:
  - ../concepts/modules/ # /docs/alloy/latest/concepts/modules/
description: Learn about modules
title: Modules
weight: 60
---

# Modules

You learned about components, expressions, and the configuration syntax in the previous sections.
Now you'll learn about modules, the organizational structure that lets you package and reuse {{< param "PRODUCT_NAME" >}} configurations.

Modules enable powerful configuration management capabilities:

1. **Code reuse**: Package common pipeline patterns into reusable custom components.
1. **Organization**: Structure complex configurations into maintainable units.
1. **Sharing**: Distribute configurations across teams and projects.
1. **Composition**: Combine multiple modules to build sophisticated data collection systems.

{{< admonition type="note" >}}
Most {{< param "PRODUCT_NAME" >}} users don't need modules when they first get started.
You can collect, process, and export metrics, logs, traces, and profiles using built-in components directly in your configuration.
Modules don't add capabilities to {{< param "PRODUCT_NAME" >}} or enable signal types, they're purely a way to organize and reuse configuration.
Think of a module as a function in programming: it packages logic you already have so you can reuse it cleanly.

If you're just getting started, you can skip modules for now and return to them when your configuration grows or you need reuse.
However, if you plan to use [Fleet Management][] to manage multiple collectors, understanding modules helps you create reusable configurations that can be distributed across your fleet.

[Fleet Management]: https://grafana.com/docs/grafana-cloud/send-data/fleet-management/
{{< /admonition >}}

A _module_ is a unit of {{< param "PRODUCT_NAME" >}} configuration that contains components, custom component definitions, and import statements.
The module you pass to [the `run` command][run] becomes the _main configuration_ that {{< param "PRODUCT_NAME" >}} executes.

You can [import modules](#import-modules) to reuse [custom components][] defined by that module.

## When to use a module

Create a module when you want to:

- Reuse the same pipeline pattern multiple times
- Reduce duplicated configuration
- Hide complex pipelines behind a single interface
- Standardize pipelines across teams or environments
- Make large {{< param "PRODUCT_NAME" >}} configurations easier to maintain

If your configuration is small and each pipeline is unique, you probably don't need a module.
If you find yourself copying and pasting the same group of components with small changes, a module can help.

## Why modules help

Without modules, repeated pipelines can look like this:

```alloy
otelcol.receiver.otlp "app1" { ... }
otelcol.processor.batch "app1" { ... }
otelcol.exporter.otlp "app1" { ... }

otelcol.receiver.otlp "app2" { ... }
otelcol.processor.batch "app2" { ... }
otelcol.exporter.otlp "app2" { ... }
```

With a module, you define the pipeline once and reuse it:

```alloy
import.file "pipeline" {
  filename = "pipeline.alloy"
}

pipeline.app "app1" { ... }
pipeline.app "app2" { ... }
```

This reduces duplication, improves readability, and makes updates safer because you only change the logic in one place.

## Example

This example demonstrates how modules integrate with the component and expression concepts you learned earlier.
The module defines a reusable log filtering component that showcases:

1. **Custom component definition**: Using `declare` to create reusable logic.
1. **Arguments**: Accepting configuration from component users.
1. **Exports**: Exposing values for other components to reference.
1. **Internal pipeline**: Using built-in components within the custom component.

**Module definition** `helpers.alloy`:

```alloy
declare "log_filter" {
  // argument.write_to is a required argument that specifies where filtered
  // log lines are sent.
  //
  // The value of the argument is retrieved in this file with
  // argument.write_to.value.
  argument "write_to" {
    optional = false
  }

  // loki.process.filter is our component which executes the filtering,
  // passing filtered logs to argument.write_to.value.
  loki.process "filter" {
    // Drop all debug- and info-level logs.
    stage.match {
      selector = `{job!=""} |~ "level=(debug|info)"`
      action   = "drop"
    }

    // Send processed logs to our argument.
    forward_to = argument.write_to.value
  }

  // export.filter_input exports a value to the module consumer.
  export "filter_input" {
    // Expose the receiver of loki.process so the module importer can send
    // logs to our loki.process component.
    value = loki.process.filter.receiver
  }
}
```

**Use the module** `main.alloy`:

```alloy
// Import our helpers.alloy module, exposing its custom components as
// helpers.COMPONENT_NAME.
import.file "helpers" {
  filename = "helpers.alloy"
}

loki.source.file "self" {
  targets = LOG_TARGETS

  // Forward collected logs to the input of our filter.
  forward_to = [helpers.log_filter.default.filter_input]
}

helpers.log_filter "default" {
  // Configure the filter to forward filtered logs to loki.write below.
  write_to = [loki.write.default.receiver]
}

loki.write "default" {
  endpoint {
    url = LOKI_URL
  }
}
```

This example shows how:

1. **Modules encapsulate logic**: The filtering logic is contained within the module.
1. **Arguments provide flexibility**: The `write_to` argument lets users specify where filtered logs go.
1. **Exports enable connections**: The `filter_input` export allows other components to send data to the filter.
1. **Component references work across modules**: The main configuration references exports from the imported module using `helpers.log_filter.default.filter_input`.

## Import modules

To use modules in your configuration, you import them using one of the `import` configuration blocks.
The component controller processes import statements during configuration loading to make the imported custom components available.

1. [`import.file`][import.file]: Imports a module from a file on disk.
1. [`import.git`][import.git]: Imports a module from a file in a Git repository.
1. [`import.http`][import.http]: Imports a module from an HTTP request response.
1. [`import.string`][import.string]: Imports a module from a string.

{{< admonition type="warning" >}}
You can't import a module that contains top-level blocks other than `declare` or `import`.
{{< /admonition >}}

You import modules into a _namespace_.
This exposes the top-level custom components of the imported module to the importing module.
The label of the import block specifies the namespace of an import.

For example, if a configuration contains a block called `import.file "my_module"`, then custom components defined by that module appear as `my_module.CUSTOM_COMPONENT_NAME`.
Namespaces for imports must be unique within a given importing module.

{{< admonition type="warning" >}}
If you use a label for an `import` or `declare` block that matches a built-in component namespace, such as `prometheus` or `mimir`, the built-in namespace becomes shadowed and unavailable in your configuration.
For example, if you use the label `import.file "mimir"`, you can't use components starting with `mimir`, such as `mimir.rules.kubernetes`, because the label refers to the imported module.
{{< /admonition >}}

## Security

Since modules can load arbitrary configurations from potentially remote sources, you must carefully consider the security implications.
The component controller executes all module content with the same privileges as the main {{< param "PRODUCT_NAME" >}} process.

Best practices for secure module usage:

1. **Protect main configuration**: Ensure attackers can't modify the {{< param "PRODUCT_NAME" >}} configuration files.
1. **Secure remote sources**: Protect modules fetched from remote locations, such as Git repositories or HTTP servers.
1. **Validate module content**: Review imported modules for malicious or unintended behavior.
1. **Use authentication**: When fetching modules over HTTP or Git, use appropriate authentication mechanisms.
1. **Network security**: Restrict network access for {{< param "PRODUCT_NAME" >}} processes that load remote modules.
1. **File permissions**: Set appropriate file system permissions for module files and directories.

## Next steps

To learn more about modules, explore these resources:

- [Custom components][] - Learn how to create reusable components that you can package in modules
- [Component configuration][components] - Understand how built-in components work within module contexts
- [Import configuration blocks][imports] - Detailed reference for different module import methods
- [Run command reference][run] - Module configuration and execution options

[custom components]: ../components/custom-components/
[components]: ../components/
[imports]: ../../reference/config-blocks/
[run]: ../../reference/cli/run/
[import.file]: ../../reference/config-blocks/import.file/
[import.git]: ../../reference/config-blocks/import.git/
[import.http]: ../../reference/config-blocks/import.http/
[import.string]: ../../reference/config-blocks/import.string/

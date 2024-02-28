---
aliases:
- ../../concepts/modules/
- /docs/grafana-cloud/agent/flow/concepts/modules/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/modules/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/modules/
- /docs/grafana-cloud/send-data/agent/flow/concepts/modules/
canonical: https://grafana.com/docs/agent/latest/flow/concepts/modules/
description: Learn about modules
title: Modules
weight: 400
---

# Modules

A _Module_ is a unit of {{< param "PRODUCT_NAME" >}} configuration, which combines all the other concepts, containing a mix of configuration blocks, instantiated components, and custom component definitions.
The module passed as an argument to [the `run` command][run] is called the _main configuration_.

Modules can be [imported](#importing-modules) to enable the reuse of [custom components][] defined by that module.

[custom components]: {{< relref "./custom_components.md" >}}
[run]: {{< relref "../reference/cli/run.md" >}}

## Importing modules

A module can be _imported_, allowing the custom components defined by that module to be used by other modules, called the _importing module_.
Modules can be imported from multiple locations using one of the `import` configuration blocks:

* [import.file]: Imports a module from a file on disk.
* [import.git]: Imports a module from a file located in a Git repository.
* [import.http]: Imports a module from the response of an HTTP request.
* [import.string]: Imports a module from a string.

[import.file]: {{< relref "../reference/config-blocks/import.file.md" >}}
[import.git]: {{< relref "../reference/config-blocks/import.git.md" >}}
[import.http]: {{< relref "../reference/config-blocks/import.http.md" >}}
[import.string]: {{< relref "../reference/config-blocks/import.string.md" >}}

{{< admonition type="warning" >}}
You can't import a module that contains top-level blocks other than `declare` or `import`.
{{< /admonition >}}

Modules are imported into a _namespace_ where the top-level custom components of the imported module are exposed to the importing module.
The label of the import block specifies the namespace of an import.
For example, if a configuration contains a block called `import.file "my_module"`, then custom components defined by that module are exposed as `my_module.CUSTOM_COMPONENT_NAME`. Imported namespaces must be unique across a given importing module.

If an import namespace matches the name of a built-in component namespace, such as `prometheus`, the built-in namespace is hidden from the importing module, and only components defined in the imported module may be used.

## Example

This example module defines a component to filter out debug-level and info-level log lines:

```river
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

You can save this module to a file called `helpers.river` and import it:

```river
// Import our helpers.river module, exposing its custom components as
// helpers.COMPONENT_NAME.
import.file "helpers" {
  filename = "helpers.river"
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

{{< collapse title="Classic modules" >}}
# Classic modules (deprecated)

{{< admonition type="caution" >}}
Modules were redesigned in v0.40 to simplify concepts. This section outlines the design of the original modules prior to v0.40. Classic modules are scheduled to be removed in the release after v0.40.
{{< /admonition >}}


You use _Modules_ to create {{< param "PRODUCT_NAME" >}} configurations that you can load as a component.
Modules are a great way to parameterize a configuration to create reusable pipelines.

Modules are {{< param "PRODUCT_NAME" >}} configurations which have:

* _Arguments_: Settings that configure a module.
* _Exports_: Named values that a module exposes to the consumer of the module.
* _Components_: {{< param "PRODUCT_NAME" >}} components to run when the module is running.

You use a [Module loader][] to load Modules into {{< param "PRODUCT_NAME" >}}.

Refer to [argument block][] and [export block][] to learn how to define arguments and exports for a module.

## Module loaders

A _Module loader_ is a {{< param "PRODUCT_NAME" >}} component that retrieves a module and runs the defined components.

Module loader components are responsible for the following functions:

* Retrieving the module source.
* Creating a [Component controller][] for the module.
* Passing arguments to the loaded module.
* Exposing exports from the loaded module.

Module loaders are typically called `module.LOADER_NAME`.

{{< admonition type="note" >}}
Some module loaders may not support running modules with arguments or exports.
{{< /admonition >}}

Refer to [Components][] for more information about the module loader components.

## Module sources

Modules are flexible, and you can retrieve their configuration anywhere, such as:

* The local filesystem.
* An S3 bucket.
* An HTTP endpoint.

Each module loader component supports different ways of retrieving `module.sources`.
The most generic module loader component, `module.string`, can load modules from the export of another {{< param "PRODUCT_NAME" >}} component.

```river
local.file "my_module" {
  filename = "PATH_TO_MODULE"
}

module.string "my_module" {
  content = local.file.my_module.content

  arguments {
    MODULE_ARGUMENT_NAME_1 = MODULE_ARGUMENT_VALUE_1
    MODULE_ARGUMENT_NAME_2 = MODULE_ARGUMENT_VALUE_2
    // ...
  }
}
```

## Example module

This example module manages a pipeline that filters out debug-level and info-level log lines.

```river
// argument.write_to is a required argument that specifies where filtered
// log lines are sent.
//
// The value of the argument is retrieved in this file with
// argument.write_to.value.
argument "write_to" {
  optional = false
}

// loki.process.filter is our component which executes the filtering, passing
// filtered logs to argument.write_to.value.
loki.process "filter" {
  // Drop all debug- and info-level logs.
  stage.match {
    selector = "{job!=\"\"} |~ \"level=(debug|info)\""
    action   = "drop"
  }

  // Send processed logs to our argument.
  forward_to = argument.write_to.value
}

// export.filter_input exports a value to the module consumer.
export "filter_input" {
  // Expose the receiver of loki.process so the module consumer can send
  // logs to our loki.process component.
  value = loki.process.filter.receiver
}
```

You can save the module to a file and then use it as a processing step before writing logs to Loki.

```river
loki.source.file "self" {
  targets = LOG_TARGETS

  // Forward collected logs to the input of our filter.
  forward_to = [module.file.log_filter.exports.filter_input]
}

module.file "log_filter" {
  filename = "/path/to/modules/log_filter.river"

  arguments {
    // Configure the filter to forward filtered logs to loki.write below.
    write_to = [loki.write.default.receiver],
  }
}

loki.write "default" {
  endpoint {
    url = "LOKI_URL"
  }
}
```

[Module loader]: #module-loaders
[argument block]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/config-blocks/argument
[export block]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/config-blocks/export
[Component controller]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/concepts/component_controller
[Components]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components
{{< /collapse >}}

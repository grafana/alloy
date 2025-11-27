---
canonical: https://grafana.com/docs/alloy/latest/get-started/components/configure-components/
aliases:
  - ./configuration-syntax/components/ # /docs/alloy/latest/get-started/configuration-syntax/components/
description: Learn how to declare and configure components
title: Configure components
weight: 10
---

# Configure components

Components are the defining feature of {{< param "PRODUCT_NAME" >}}.
They're reusable pieces of business logic that perform a single task, such as retrieving secrets or collecting Prometheus metrics.
You can wire them together to form programmable pipelines of telemetry data.

To build effective configurations, you need to understand how to define components, set their arguments, and connect them using exports.

## Basic component syntax

You create [components][] by defining a top-level block with a component name and a user-specified label.

```alloy
COMPONENT_NAME "LABEL" {
  // Component configuration goes here
}
```

For example, this creates a `local.file` component with the label "config":

```alloy
local.file "config" {
  filename = "/etc/app/settings.yaml"
}
```

The [_component controller_][controller] schedules components, reports their health and debug status, re-evaluates their arguments, and provides their exports.

## Arguments and exports

Most user interactions with components center around two basic concepts: _arguments_ and _exports_.

### Arguments

_Arguments_ are settings that modify a component's behavior.
They can include attributes or nested unlabeled blocks, some of which you must provide and others that are optional.
Optional arguments that aren't set use their default values.

```alloy
local.file "targets" {
  filename = "/etc/alloy/targets"  // Required argument

  // Optional arguments
  poll_frequency = "1m"
  is_secret = false
}
```

### Exports

_Exports_ are zero or more output values that other components can refer to.
They can be of any {{< param "PRODUCT_NAME" >}} type.

The `local.file.targets` component from the previous example exposes the file content as a string in its exports.
You'll learn how to reference these exports in [Expressions][].

## Configuration blocks

Some components use nested blocks to organize related settings.

```alloy
prometheus.remote_write "production" {
  endpoint {
    url = "https://prometheus.example.com/api/v1/write"

    basic_auth {
      username = "metrics"
      password_file = "/etc/secrets/password"
    }
  }

  queue_config {
    capacity = 10000
    batch_send_deadline = "5s"
  }
}
```

In this example:

- `endpoint` is a block that configures the remote endpoint
- `basic_auth` is a nested block within `endpoint`
- `queue_config` is another top-level block within the component

## Component references

To wire components together, use the exports of one as the arguments to another by using references.
References can only appear in component arguments.

```alloy
local.file "api_key" {
  filename = "/etc/secrets/api.key"
}

prometheus.remote_write "production" {
  endpoint {
    url = "https://prometheus.example.com/api/v1/write"

    basic_auth {
      username = "metrics"
      password = local.file.api_key.content  // Reference to component export
    }
  }
}
```

Each time the file contents change, the `local.file` component updates its exports.
The component sends the updated value to the `prometheus.remote_write` component's `password` field.

### Reference syntax

To reference a component export, combine three parts with periods:

- Component name: `local.file`
- Component label: `api_key`
- Export name: `content`
- Result: `local.file.api_key.content`

Each argument and exported field has an underlying [type][].
{{< param "PRODUCT_NAME" >}} checks the expression type before assigning a value to an attribute.

## Multiple component instances

You can create multiple instances of the same component type by using different labels:

```alloy
prometheus.scrape "api" {
  targets = [{"__address__" = "api.example.com:8080"}]
  forward_to = [prometheus.remote_write.production.receiver]
}

prometheus.scrape "database" {
  targets = [{"__address__" = "db.example.com:9090"}]
  forward_to = [prometheus.remote_write.production.receiver]
}
```

Both scrape configurations send metrics to the same `prometheus.remote_write` component.

## Component rules

Components can't form cycles.
This means that a component can't reference itself directly or indirectly.
This prevents infinite loops from forming in your configuration.

```alloy
// INVALID: Component can't reference itself
local.file "self_reference" {
  filename = local.file.self_reference.content
}
```

```alloy
// INVALID: Cyclic reference between components
local.file "a" {
  filename = local.file.b.content
}

local.file "b" {
  filename = local.file.a.content
}
```

## Next steps

Now that you understand how to configure components, learn about more advanced topics:

- [Build data pipelines][] - Connect components together to create data processing workflows
- [Component controller][] - Understand how {{< param "PRODUCT_NAME" >}} manages components at runtime
- [Expressions][] - Write dynamic configuration using component references and functions
- [Component reference][] - Explore all available components and their arguments and exports

For practical examples, try the [tutorials][] to build complete data collection pipelines.

[components]: ../../reference/components/
[controller]: ./component-controller/
[type]: ../expressions/types_and_values/
[Build data pipelines]: ./build-pipelines/
[Component controller]: ./component-controller/
[Expressions]: ../expressions/
[Component reference]: ../../reference/components/
[tutorials]: ../../tutorials/

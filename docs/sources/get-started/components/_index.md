---
canonical: https://grafana.com/docs/alloy/latest/get-started/components/
aliases:
  - ./components/ # /docs/alloy/latest/get-started/components/
  - ./concepts/components/ # /docs/alloy/latest/concepts/components/
  - ./concepts/configuration-syntax/components/ # /docs/alloy/latest/concepts/configuration-syntax/
  - ./get-started/configuration-syntax/components/components/ # /docs/alloy/latest/get-started/configuration-syntax/components/
description: Learn about components
title: Components
weight: 30
---

# Components

_Components_ are the building blocks of {{< param "PRODUCT_NAME" >}}.
Each component performs a single task, such as retrieving secrets, collecting metrics, or processing data.

You learned about blocks, attributes, and expressions in the previous section.
Components use the same syntax structure. They're special blocks that {{< param "PRODUCT_NAME" >}} knows how to execute.
When you define a component block in your configuration, {{< param "PRODUCT_NAME" >}} creates a running instance that performs the component's specific task.

Components are the key to the flexibility of {{< param "PRODUCT_NAME" >}}.
You can combine them to create data pipelines that collect, transform, and send telemetry data exactly how you need.

## Component structure

Every component has two main parts:

- **Arguments:** Settings that configure the component's behavior. These are the attributes and blocks you define inside the component block.
- **Exports:** Values that the component makes available to other components. The running component generates these exports and they can change over time.

When {{< param "PRODUCT_NAME" >}} loads your configuration:

1. It reads the arguments you provided in each component block.
1. It creates a running instance of the component with those arguments.
1. The component starts its work and may update its exports as it runs.
1. Other components can reference these exports in their arguments.

## Component syntax

Components use the block syntax you learned about earlier.
The general pattern is:

```alloy
COMPONENT_NAME "LABEL" {
  // Arguments (attributes and nested blocks)
  attribute_name = "value"

  nested_block {
    setting = "value"
  }
}
```

The `COMPONENT_NAME` tells {{< param "PRODUCT_NAME" >}} which type of component to create.
The `"LABEL"` is a unique identifier you choose to distinguish between multiple instances of the same component type.

## Component names

Each component has a name that describes its purpose.
For example:

- `local.file` retrieves file contents from disk.
- `prometheus.scrape` collects Prometheus metrics.
- `loki.write` sends log data to Loki.

You define components by specifying the component name with a user-defined label:

```alloy
local.file "my_config" {
  filename = "/etc/app/config.yaml"
}

prometheus.scrape "api_metrics" {
  targets = [{"__address__" = "localhost:8080"}]
  forward_to = [prometheus.remote_write.default.receiver]
}
```

## Component references

You reference components by combining their name with their label.
For example, you can reference the `local.file` component labeled `my_config` as `local.file.my_config`.

The combination of component name and label must be unique in your configuration.
This allows you to define multiple instances of the same component type:

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

## Component exports

Components can share data through exports.
When one component references another component's export, it creates a connection between them.
You'll learn more about these component references in [Expressions][].

```alloy
local.file "api_key" {
  filename = "/etc/secrets/api.key"
}

prometheus.remote_write "production" {
  endpoint {
    url = "https://prometheus.example.com/api/v1/write"

    basic_auth {
      username = "metrics"
      password = local.file.api_key.content  // Reference the file content
    }
  }
}
```

In this example:

1. The `local.file` component reads a file and exports its content.
1. The `prometheus.remote_write` component uses that content as a password.
1. When the file changes, {{< param "PRODUCT_NAME" >}} automatically updates the `local.file` component's exports.
1. This causes {{< param "PRODUCT_NAME" >}} to re-evaluate the `prometheus.remote_write` component with the new password.

You'll learn the details of how to write these references in [Expressions][].

## Next steps

Now that you understand what components are and how they work, learn how to use them effectively:

- [Configure components][] - Learn how to define and configure components with arguments and blocks
- [Build data pipelines][] - Connect components together to create data processing workflows
- [Component controller][] - Understand how {{< param "PRODUCT_NAME" >}} manages components at runtime

To extend component functionality:

- [Custom components][] - Create reusable custom components for your specific needs
- [Community components][] - Use components shared by the {{< param "PRODUCT_NAME" >}} community

For hands-on learning:

- [Tutorials][] - Build complete data collection pipelines step by step

For reference:

- [Expressions][] - Use component exports in dynamic configurations
- [Alloy syntax][] - Explore advanced syntax features and patterns
- [Component reference][] - Browse all available components and their options

[Configure components]: ./configure-components/
[Build data pipelines]: ./build-pipelines/
[Component controller]: ./component-controller/
[Custom components]: ./custom-components/
[Community components]: ./community-components/
[Expressions]: ../expressions/
[Alloy syntax]: ../syntax/
[tutorials]: ../../tutorials/
[Component reference]: ../../reference/components/

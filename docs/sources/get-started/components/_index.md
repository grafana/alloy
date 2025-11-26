---
canonical: https://grafana.com/docs/alloy/latest/get-started/components/
aliases:
  - ./components/ # /docs/alloy/latest/get-started/components/
  - ./concepts/components/ # /docs/alloy/latest/concepts/components/
  - ./concepts/configuration-syntax/components/ # /docs/alloy/latest/concepts/configuration-syntax/components/
description: Learn about components
title: Components
weight: 40
---

# Components

_Components_ are the building blocks of {{< param "PRODUCT_NAME" >}}.
Each component performs a single task, such as retrieving secrets, collecting metrics, or processing data.

Components are the key to {{< param "PRODUCT_NAME" >}}'s flexibility.
You can combine them to create data pipelines that collect, transform, and send telemetry data exactly how you need.

## Component structure

Every component has two main parts:

- **Arguments:** Settings that configure the component's behavior
- **Exports:** Values that the component makes available to other components

## Component naming

Each component has a name that describes its purpose.
For example:

- `local.file` retrieves file contents from disk
- `prometheus.scrape` collects Prometheus metrics
- `loki.write` sends log data to Loki

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

## Using component exports

Components can share data through exports.
When one component references another component's export, it creates a connection between them.

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

1. The `local.file` component reads a file and exports its content
2. The `prometheus.remote_write` component uses that content as a password
3. When the file changes, {{< param "PRODUCT_NAME" >}} automatically updates the password

## Next steps

Learn more about working with components:

- [Configure components][] - Learn how to define and configure components with arguments and blocks
- [Build data pipelines][] - Connect components together to create data processing workflows
- [Component controller][] - Understand how {{< param "PRODUCT_NAME" >}} manages components at runtime

For practical examples:

- [Expressions][] - Use component exports in dynamic configurations
- [Component reference][] - Browse all available components and their options

[Configure components]: ./configure-components/
[Build data pipelines]: ./build-pipelines/
[Component controller]: ./component-controller/
[Expressions]: ../expressions/
[Component reference]: ../../reference/components/

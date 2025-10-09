---
canonical: https://Grafana.com/docs/alloy/latest/get-started/components/
aliases:
  - ./components/ # /docs/alloy/latest/get-started/components/
description: Learn about components
title: Components
weight: 40
---

# Components

_Components_ are the building blocks of {{< param "PRODUCT_NAME" >}}.
Each component performs a single task, such as retrieving secrets or collecting Prometheus metrics.

Components have two main parts:

- **Arguments:** Settings that configure a component
- **Exports:** Named values that a component makes available to other components

Each component has a name that describes what it does.
For example, the `local.file` component retrieves the contents of files on disk.

You define components in your configuration file by specifying the component's name with a user-defined label, followed by arguments to configure the component.

```alloy
discovery.kubernetes "pods" {
  role = "pod"
}

discovery.kubernetes "nodes" {
  role = "node"
}
```

You reference components by combining their name with their label.
For example, you can reference a `local.file` component labeled `foo` as `local.file.foo`.

The combination of a component's name and label must be unique within your configuration file.
This naming approach allows you to define multiple instances of a component, as long as each instance has a unique label.

## Pipelines

Most arguments for a component in a configuration file are constant values, such as setting a `log_level` attribute to `"debug"`:

```alloy
log_level = "debug"
```

You use _expressions_ to compute an argument's value dynamically at runtime.
Expressions can retrieve environment variable values (`log_level = sys.env("LOG_LEVEL")`) or reference an exported field of another component (`log_level = local.file.log_level.content`).

A dependent relationship is created when a component's argument references an exported field of another component.
The component's arguments depend on the other component's exports.
The input of the component is re-evaluated whenever the referenced component's exports are updated.

The flow of data through these references forms a _pipeline_.

An example pipeline might look like this:

1. A `local.file` component watches a file containing an API key
1. A `prometheus.remote_write` component receives metrics and forwards them to an external database using the API key from the `local.file` for authentication
1. A `discovery.kubernetes` component discovers and exports Kubernetes Pods where metrics can be collected
1. A `prometheus.scrape` component references the exports of the previous component and sends collected metrics to the `prometheus.remote_write` component

{{< figure src="/media/docs/alloy/diagram-concepts-example-pipeline.png" width="600" alt="Example of a pipeline" >}}

The following configuration file represents the pipeline.

```alloy
// Get our API key from disk.
//
// This component has an exported field called "content", holding the content
// of the file.
//
// local.file will watch the file and update its exports any time the
// file changes.
local.file "api_key" {
  filename  = "/var/data/secrets/api-key"

  // Mark this file as sensitive to prevent its value from being shown in the
  // UI.
  is_secret = true
}

// Create a prometheus.remote_write component, which other components can send
// metrics to.
//
// This component exports a "receiver" value, which can be used by other
// components to send metrics.
prometheus.remote_write "prod" {
  endpoint {
    url = "https://prod:9090/api/v1/write"

    basic_auth {
      username = "admin"

      // Use the password file to authenticate with the production database.
      password = local.file.api_key.content
    }
  }
}

// Find Kubernetes pods where we can collect metrics.
//
// This component exports a "targets" value, which contains the list of
// discovered pods.
discovery.kubernetes "pods" {
  role = "pod"
}

// Collect metrics from Kubernetes pods and send them to prod.
prometheus.scrape "default" {
  targets    = discovery.kubernetes.pods.targets
  forward_to = [prometheus.remote_write.prod.receiver]
}
```

## Next steps

Learn more about working with components:

- [Configure components][] to understand component arguments, exports, and configuration blocks
- [Component controller][] to learn how {{< param "PRODUCT_NAME" >}} manages components at runtime
- [Expressions][] to write dynamic expressions that reference component exports

[Configure components]: ./configure-components/
[Component controller]: ./component-controller/
[Expressions]: ../expressions/

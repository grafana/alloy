---
canonical: https://Grafana.com/docs/alloy/latest/get-started/components/configure-components/
aliases:
  - ./configuration-syntax/components/ # /docs/alloy/latest/get-started/configuration-syntax/components/
description: Learn how to declare and configure components
title: Declare and configure components
weight: 50
---

# Declare and configure components

Components are the defining feature of {{< param "PRODUCT_NAME" >}}.
They're reusable pieces of business logic that perform a single task, such as retrieving secrets or collecting Prometheus metrics.
You can wire them together to form programmable pipelines of telemetry data.

The [_component controller_][controller] schedules components, reports their health and debug status, re-evaluates their arguments, and provides their exports.

## Components overview

_Components_ are the building blocks of {{< param "PRODUCT_NAME" >}}.
Each component performs a single task, such as retrieving secrets or collecting Prometheus metrics.

Components consist of the following:

- **Arguments:** Settings that configure a component.
- **Exports:** Named values that a component makes available to other components.

Each component has a name that describes its responsibility.
For example, the `local.file` component retrieves the contents of files on disk.

You define components in the configuration file by specifying the component's name with a user-defined label, followed by arguments to configure the component.

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

The combination of a component's name and label must be unique within the configuration file.
This naming approach allows you to define multiple instances of a component, as long as each instance has a unique label.

## Configure components

You create [components][] by defining a top-level block.
Each component has a name, which describes its responsibility, and a user-specified _label_.

## Arguments and exports

Most user interactions with components center around two basic concepts, _arguments_ and _exports_.

- _Arguments_ are settings that modify a component's behavior.
  They can include attributes or nested unlabeled blocks, some of which you must provide and others that are optional.
  Optional arguments that aren't set use their default values.

- _Exports_ are zero or more output values that other components can refer to.
  They can be of any {{< param "PRODUCT_NAME" >}} type.

The following block defines a `local.file` component labeled "targets".
The `local.file.targets` component exposes the file `content` as a string in its exports.

The `filename` attribute is a _required_ argument.
You can also define several _optional_ arguments, such as `detector`, `poll_frequency`, and `is_secret`.
These arguments configure how and how often {{< param "PRODUCT_NAME" >}} polls the file and whether its contents are sensitive.

```alloy
local.file "targets" {
  // Required argument
  filename = "/etc/alloy/targets"

  // Optional arguments: Components may have some optional arguments that
  // do not need to be defined.
  //
  // The optional arguments for local.file are is_secret, detector, and
  // poll_frequency.

  // Exports: a single field named `content`
  // It can be referred to as `local.file.targets.content`
}
```

## Reference components

To wire components together, use the exports of one as the arguments to another by using references.
References can only appear in components.

For example, here's a component that scrapes Prometheus metrics.
The `targets` field contains two scrape targets, a constant target `localhost:9001` and an expression that ties the target to the value of `local.file.targets.content`.

```alloy
prometheus.scrape "default" {
  targets = [
    { "__address__" = local.file.targets.content }, // tada!
    { "__address__" = "localhost:9001" },
  ]

  forward_to = [prometheus.remote_write.default.receiver]
  scrape_config {
    job_name = "default"
  }
}
```

Each time the file contents change, the `local.file` component updates its exports.
The component sends the updated value to the `prometheus.scrape` targets field.

Each argument and exported field has an underlying [type][].
{{< param "PRODUCT_NAME" >}} checks the expression type before assigning a value to an attribute.
Refer to the documentation of each [component][components] for more information about wiring components together.

In the previous example, {{< param "PRODUCT_NAME" >}} evaluates the contents of the `local.file.targets.content` expression to a concrete value.
The system type-checks the value and substitutes it into `prometheus.scrape.default`, where you can configure it.

## Pipelines

Most arguments for a component in a configuration file are constant values, such as setting a `log_level` attribute to `"debug"`.

```alloy
log_level = "debug"
```

You use _expressions_ to compute an argument's value dynamically at runtime.
Expressions can retrieve environment variable values like `log_level = sys.env("LOG_LEVEL")` or reference an exported field of another component like `log_level = local.file.log_level.content`.

A component creates a dependent relationship when its argument references an exported field of another component.
The component's arguments depend on the other component's exports.
{{< param "PRODUCT_NAME" >}} re-evaluates the component's input whenever the referenced component updates its exports.

The flow of data through these references forms a _pipeline_.

An example pipeline might look like this:

1. A `local.file` component watches a file containing an API key.
1. A `prometheus.remote_write` component receives metrics and forwards them to an external database using the API key from the `local.file` for authentication.
1. A `discovery.kubernetes` component discovers and exports Kubernetes Pods where you can collect metrics.
1. A `prometheus.scrape` component references the exports of the previous component and sends collected metrics to the `prometheus.remote_write` component.

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

## Practical example: Your first pipeline

This example creates a pipeline that collects metrics from the host and sends them to Prometheus:

```alloy
local.file "example" {
    filename = sys.env("HOME") + "/file.txt"
}

prometheus.remote_write "local_prom" {
    endpoint {
        url = "http://localhost:9090/api/v1/write"

        basic_auth {
            username = "admin"
            password = local.file.example.content
        }
    }
}
```

This pipeline has two components: `local.file` and `prometheus.remote_write`.
The `local.file` component uses a single argument, `filename`, which calls the [sys.env][] standard library function to retrieve the value of the `HOME` environment variable and concatenates it with the string `"file.txt"`.
The `local.file` component has a single export, `content`, which contains the contents of the file.

The `prometheus.remote_write` component uses an `endpoint` block, containing the `url` attribute and a `basic_auth` block.
The `url` attribute specifies the URL of the Prometheus remote write endpoint.
The `basic_auth` block contains the `username` and `password` attributes, which specify the string `"admin"` and the `content` export of the `local.file` component, respectively.
The `content` export uses the syntax `local.file.example.content`, where `local.file.example` is the fully qualified name of the component—the component's type plus its label—and `content` is the name of the export.

{{< figure src="/media/docs/alloy/diagram-example-basic-alloy.png" width="600" alt="Example pipeline with local.file and prometheus.remote_write components" >}}

{{< admonition type="note" >}}
The `local.file` component's label uses `"example"`, so the fully qualified name of the component is `local.file.example`.
The `prometheus.remote_write` component's label uses `"local_prom"`, so the fully qualified name of the component is `prometheus.remote_write.local_prom`.
{{< /admonition >}}

## Component rules

One rule is that components can't form a cycle.
This means that a component can't reference itself directly or indirectly.
This prevents infinite loops from forming in the pipeline.

```alloy
// INVALID: local.file.some_file can't reference itself:
local.file "self_reference" {
  filename = local.file.self_reference.content
}
```

```alloy
// INVALID: cyclic reference between local.file.a and local.file.b:
local.file "a" {
  filename = local.file.b.content
}
local.file "b" {
  filename = local.file.a.content
}
```

## Next steps

Learn more about working with components:

- [Component controller][] to understand how {{< param "PRODUCT_NAME" >}} manages components at runtime
- [Component reference][components] to explore all available components and their arguments and exports
- [Expressions][] to write dynamic configuration using component references and functions

[components]: ../../reference/components/
[controller]: ./component-controller/
[type]: ../expressions/types_and_values/
[sys.env]: ../../reference/stdlib/sys/
[Component controller]: ./component-controller/
[Expressions]: ../expressions/

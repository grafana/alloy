---
canonical: https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/components/
aliases:
  - ../../concepts/configuration-syntax/components/ # /docs/alloy/latest/concepts/configuration-syntax/components/
description: Learn about the components configuration language
title: Configure components
weight: 300
---

# Configure components

Components are the defining feature of {{< param "PRODUCT_NAME" >}}.
They're small, reusable pieces of business logic that perform a single task, such as retrieving secrets or collecting Prometheus metrics.
You can wire them together to form programmable pipelines of telemetry data.

The [_component controller_][controller] schedules components, reports their health and debug status, re-evaluates their arguments, and provides their exports.

## Configuring components

You create [components][] by defining a top-level block.
Each component is identified by its name, which describes its responsibility, and a user-specified _label_.

## Arguments and exports

Most user interactions with components center around two basic concepts, _arguments_ and _exports_.

* _Arguments_ are settings that modify a component's behavior.
  They can include attributes or nested unlabeled blocks, some of which are required and others optional.
  Optional arguments that aren't set use their default values.

* _Exports_ are zero or more output values that other components can refer to.
  They can be of any {{< param "PRODUCT_NAME" >}} type.

The following block defines a `local.file` component labeled "targets".
The `local.file.targets` component exposes the file `content` as a string in its exports.

The `filename` attribute is a _required_ argument.
You can also define several _optional_ arguments, such as `detector`, `poll_frequency`, and `is_secret`.
These arguments configure how and how often the file is polled and whether its contents are sensitive.

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
The `targets` field is populated with two scrape targets, a constant target `localhost:9001` and an expression that ties the target to the value of `local.file.targets.content`.

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
The new value is sent to the `prometheus.scrape` targets field.

Each argument and exported field has an underlying [type][].
{{< param "PRODUCT_NAME" >}} checks the expression type before assigning a value to an attribute.
Refer to the documentation of each [component][components] for more information about wiring components together.

In the previous example, the contents of the `local.file.targets.content` expression are evaluated to a concrete value.
The value is type-checked and substituted into `prometheus.scrape.default`, where you can configure it.

[components]: ../../../reference/components/
[controller]: ../../component_controller/
[type]: ../expressions/types_and_values/

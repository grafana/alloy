---
aliases:
- ../configuration-language/components/ # /docs/agent/latest/flow/concepts/configuration-language/components/
- /docs/grafana-cloud/agent/flow/concepts/config-language/components/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/config-language/components/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/config-language/components/
- /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/components/
# Previous page aliases for backwards compatibility:
- ../../configuration-language/components/ # /docs/agent/latest/flow/configuration-language/components/
- /docs/grafana-cloud/agent/flow/config-language/components/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/config-language/components/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/config-language/components/
- /docs/grafana-cloud/send-data/agent/flow/config-language/components/
canonical: https://grafana.com/docs/agent/latest/flow/concepts/config-language/components/
description: Learn about the components configuration language
title: Components configuration language
weight: 300
---

# Components configuration language

Components are the defining feature of {{< param "PRODUCT_NAME" >}}.
Components are small, reusable pieces of business logic that perform a single task like retrieving secrets or collecting Prometheus metrics,
and you can wire them together to form programmable pipelines of telemetry data.

The [_component controller_][controller] is responsible for scheduling components, reporting their health and debug status, re-evaluating their arguments, and providing their exports.

## Configuring components

You create [components][] by defining a top-level River block.
All components are identified by their name, describing what the component is responsible for, and a user-specified _label_.

## Arguments and exports

Most user interactions with components center around two basic concepts, _arguments_ and _exports_.

* _Arguments_ are settings that modify the behavior of a component.
  They can be any number of attributes or nested unlabeled blocks, some required and some optional.
  Any optional arguments that aren't set take on their default values.

* _Exports_ are zero or more output values that other components can refer to and can be of any River type.

The following block defines a `local.file` component labeled "targets".
The `local.file.targets` component exposes the file `content` as a string in its exports.

The `filename` attribute is a _required_ argument.
You can also define a number of _optional_ arguments, in this case, `detector`, `poll_frequency`, and `is_secret`,
that configure how and how often the file should be polled and whether its contents are sensitive.

```river
local.file "targets" {
  // Required argument
  filename = "/etc/agent/targets"

  // Optional arguments: Components may have some optional arguments that
  // do not need to be defined.
  //
  // The optional arguments for local.file are is_secret, detector, and
  // poll_frequency.

  // Exports: a single field named `content`
  // It can be referred to as `local.file.targets.content`
}
```

## Referencing components

To wire components together, one can use the exports of one as the arguments to another by using references.
References can only appear in components.

For example, here's a component that scrapes Prometheus metrics.
The `targets` field is populated with two scrape targets, a constant target `localhost:9001` and an expression that ties the target to the value of `local.file.targets.content`.

```river
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

Each time the file contents change, the `local.file` updates its exports. The new value is sent to the `prometheus.scrape` targets field.

Each argument and exported field has an underlying [type][].
River checks the expression type before assigning a value to an attribute.
The documentation of each [component][components] provides more information about how to wire components together.

In the previous example, the contents of the `local.file.targets.content` expression is evaluated to a concrete value.
The value is type-checked and substituted into `prometheus.scrape.default`, where you can configure it.

{{% docs/reference %}}
[components]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components"
[components]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components"
[controller]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/component_controller"
[controller]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/component_controller"
[type]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/config-language/expressions/types_and_values"
[type]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/expressions/types_and_values"
{{% /docs/reference %}}
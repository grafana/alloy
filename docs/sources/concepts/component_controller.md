---
aliases:
- ../../concepts/component-controller/
- /docs/grafana-cloud/agent/flow/concepts/component_controller/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/concepts/component_controller/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/concepts/component_controller/
- /docs/grafana-cloud/send-data/agent/flow/concepts/component_controller/
canonical: https://grafana.com/docs/agent/latest/flow/concepts/component_controller/
description: Learn about the component controller
title: Component controller
weight: 200
---

# Component controller

The _component controller_ is the core part of {{< param "PRODUCT_NAME" >}} which manages components at runtime.

The component controller is responsible for:

* Reading and validating the configuration file.
* Managing the lifecycle of defined components.
* Evaluating the arguments used to configure components.
* Reporting the health of defined components.

## Component graph

A relationship between [components][Components] is created when an expression is used to set the argument of one component to an exported field of another component.

The set of all components and the relationships between them define a [Directed Acyclic Graph][DAG] (DAG),
which informs the component controller which references are valid and in what order components must be evaluated.

For a configuration file to be valid, components must not reference themselves or contain a cyclic reference.

```river
// INVALID: local.file.some_file can not reference itself:
local.file "self_reference" {
  filename = local.file.self_reference.content
}
```

```river
// INVALID: cyclic reference between local.file.a and local.file.b:
local.file "a" {
  filename = local.file.b.content
}
local.file "b" {
  filename = local.file.a.content
}
```

## Component evaluation

A component is _evaluated_ when its expressions are computed into concrete values.
The computed values configure the component's runtime behavior.
The component controller is finished loading once all components are evaluated, configured, and running.

The component controller only evaluates a given component after evaluating all of that component's dependencies.
Components that don't depend on other components can be evaluated anytime during the evaluation process.

## Component reevaluation

A [component][Components] is dynamic. A component can update its exports any number of times throughout its lifetime.

A _controller reevaluation_ is triggered when a component updates its exports.
The component controller reevaluates any component that references the changed component, any components that reference those components,
and so on, until all affected components are reevaluated.

## Component health

At any given time, a component can have one of the following health states:

1. Unknown: The default state. The component isn't running yet.
1. Healthy: The component is working as expected.
1. Unhealthy: The component isn't working as expected.
1. Exited: The component has stopped and is no longer running.

By default, the component controller determines the health of a component.
The component controller marks a component as healthy as long as that component is running and its most recent evaluation succeeded.

Some components can report their own component-specific health information.
For example, the `local.file` component reports itself as unhealthy if the file it was watching gets deleted.

The overall health of a component is determined by combining the controller-reported health of the component with the component-specific health information.

An individual component's health is independent of the health of any other components it references.
A component can be marked as healthy even if it references an exported field of an unhealthy component.

## Handling evaluation failures

When a component fails to evaluate, it's marked as unhealthy with the reason for why the evaluation failed.

When an evaluation fails, the component continues operating as normal.
The component continues using its previous set of evaluated arguments and can continue exporting new values.

This behavior prevents failure propagation.
If your `local.file` component, which watches API keys, suddenly stops working, other components continue using the last valid API key until the component returns to a healthy state.

## In-memory traffic

Components that expose HTTP endpoints, such as [prometheus.exporter.unix][], can expose an internal address that completely bypasses the network and communicate in-memory.
Components within the same process can communicate with one another without needing to be aware of any network-level protections such as authentication or mutual TLS.

The internal address defaults to `agent.internal:12345`.
If this address collides with a real target on your network, change it to something unique using the `--server.http.memory-addr` flag in the [run][] command.

Components must opt-in to using in-memory traffic.
Refer to the individual documentation for components to learn if in-memory traffic is supported.

## Updating the configuration file

The `/-/reload` HTTP endpoint and the `SIGHUP` signal can inform the component controller to reload the configuration file.
When this happens, the component controller synchronizes the set of running components with the ones in the configuration file,
removing components no longer defined in the configuration file and creating new components added to the configuration file.
All components managed by the controller are reevaluated after reloading.

[DAG]: https://en.wikipedia.org/wiki/Directed_acyclic_graph

{{% docs/reference %}}
[prometheus.exporter.unix]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.exporter.unix.md"
[prometheus.exporter.unix]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.exporter.unix.md"
[run]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/cli/run.md"
[run]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/cli/run.md"
[Components]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/components.md"
[Components]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/components.md"
{{% /docs/reference %}}

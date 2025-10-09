---
canonical: https://Grafana.com/docs/alloy/latest/get-started/components/component-controller/
aliases:
  - ./component_controller/ # /docs/alloy/latest/get-started/component_controller/
description: Learn about the component controller
title: Manage components dynamically
weight: 60
---

# Component controller

The _component controller_ is the core part of {{< param "PRODUCT_NAME" >}} that manages components at runtime.

The component controller:

- Reads and validates the configuration file.
- Manages the lifecycle of defined components.
- Evaluates the arguments used to configure components.
- Reports the health of defined components.

## Component graph

A relationship between [components][Components] forms when an expression sets one component's argument to an exported field of another component.

The set of all components and their relationships defines a [Directed Acyclic Graph][DAG] (DAG).
This graph tells the component controller which references are valid and the order in which to evaluate components.

For a configuration file to be valid, components can't reference themselves or create cyclic references.

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

## Component evaluation

Component evaluation is the process of computing expressions into concrete values.
These values configure the component's runtime behavior.
The component controller is fully loaded once all components are evaluated, configured, and running.

The component controller evaluates a component only after evaluating all its dependencies.
The controller can evaluate components without dependencies at any time during the process.

## Component reevaluation

A [component][Components] is dynamic and can update its exports multiple times during its lifetime.

A _controller reevaluation_ occurs when a component updates its exports.
The component controller reevaluates any component that references the changed component, along with their dependents, until it reevaluates all affected components.

## Component health

At any time, a component can have one of these health states:

1. Unknown: The default state. The component isn't running yet.
1. Healthy: The component is working as expected.
1. Unhealthy: The component isn't working as expected.
1. Exited: The component has stopped and is no longer running.

By default, the component controller determines a component's health.
The controller marks a component as healthy if it's running and its most recent evaluation succeeded.

Some components can report their own health information.
For example, the `local.file` component reports itself as unhealthy if the file it's watching gets deleted.

The system determines the overall health of a component by combining the controller-reported health of the component with the component-specific health information.

A component's health is independent of the health of any components it references.
A component can be healthy even if it references an exported field of an unhealthy component.

## Evaluation failures

When a component fails to evaluate, it's marked as unhealthy with the reason for the failure.

Despite the failure, the component continues operating normally.
It uses its last valid set of evaluated arguments and can keep exporting updated values.

This behavior prevents failure propagation.
For example, if your `local.file` component, which watches API keys, stops working, other components continue using the last valid API key until the component recovers.

## In-memory traffic

Components that expose HTTP endpoints, such as [`prometheus.exporter.unix`][prometheus.exporter.unix], can use an internal address to bypass the network and communicate in-memory.
Components in the same process can communicate without needing network-level protections like authentication or mutual TLS.

The internal address defaults to `alloy.internal:12345`.
If this address conflicts with a real target on your network, change it using the `--server.http.memory-addr` flag in the [run][] command.

Components must opt in to using in-memory traffic.
Refer to the individual component documentation to learn if a component supports in-memory traffic.

## Configuration file updates

The `/-/reload` HTTP endpoint and the `SIGHUP` signal notify the component controller to reload the configuration file.
When reloading, the controller synchronizes the running components with the configuration file.
It removes components no longer defined and creates additional ones added to the file.
After reloading, the controller reevaluates all managed components.

## Next steps

Learn more about working with the component controller:

- [Configure components][] to understand how to write components the controller can manage
- [Monitor the component controller][] to track component health and performance
- [{{< param "PRODUCT_NAME" >}} run command][run] for configuration options that affect the controller

[DAG]: https://en.wikipedia.org/wiki/Directed_acyclic_graph
[prometheus.exporter.unix]: ../../reference/components/prometheus/prometheus.exporter.unix
[run]: ../../reference/cli/run/
[Components]: ../components/
[Configure components]: ./configure-components/
[Monitor the component controller]: ../../../troubleshoot/controller_metrics/

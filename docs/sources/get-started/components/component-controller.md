---
canonical: https://grafana.com/docs/alloy/latest/get-started/components/component-controller/
aliases:
  - ./component_controller/ # /docs/alloy/latest/get-started/component_controller/
  - ./concepts/component_controller/ # /docs/alloy/latest/concepts/component_controller/
  - ../component_controller/ # /docs/alloy/latest/get-started/component_controller/
description: Learn about the component controller
title: Component controller
weight: 30
---

# Component controller

You learned how to build pipelines by connecting components through their exports and references in the previous section.
Now you'll learn how the _component controller_ manages these components at runtime to make your pipelines work.

The component controller is the core part of {{< param "PRODUCT_NAME" >}} responsible for:

1. Reading and validating the configuration file.
1. Managing the lifecycle of defined components.
1. Evaluating the arguments used to configure components.
1. Reporting the health of defined components.

## Component graph

When you create pipelines, you establish relationships between components by referencing one component's exports in another component's arguments.
The component controller uses these relationships to build a [Directed Acyclic Graph][DAG] (DAG) that represents all components and their dependencies.

This graph serves two critical purposes:

1. **Validation**: Ensures that component references are valid and don't create circular dependencies.
1. **Evaluation order**: Determines the correct sequence for evaluating components so that dependencies are resolved before dependent components run.

For a configuration file to be valid, components can't reference themselves or create circular dependencies.

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

Component evaluation is the process of computing expressions in a component's arguments into concrete values.
These values configure the component's runtime behavior.

The component controller follows a strict evaluation order:

1. **Dependency resolution**: The controller evaluates a component only after evaluating all its dependencies.
1. **Parallel evaluation**: Components without dependencies can be evaluated in parallel during the initial startup.
1. **Component creation**: After successful evaluation, the controller creates and starts the component instance.

The component controller is fully loaded once it evaluates, configures, and starts all components.

## Component reevaluation

Components are dynamic and can update their exports multiple times during their lifetime.
For example, a `local.file` component updates its exports whenever the file content changes.

When a component updates its exports, the component controller triggers a _reevaluation_ process:

1. **Identify dependents**: The controller finds all components that reference the changed component's exports.
1. **Cascade evaluation**: The controller reevaluates those dependent components with the new export values.
1. **Propagate changes**: If any dependent component also updates its exports, the process continues until all affected components are reevaluated.

This automatic reevaluation ensures that your pipelines stay current with changing data and conditions.

## Component health

Every component has a health state that indicates whether it's working properly.
The component controller tracks health to help you monitor and troubleshoot your pipelines.

Components can be in one of these health states:

1. **Unknown**: The default state when a component is created but not yet started.
1. **Healthy**: The component is working as expected.
1. **Unhealthy**: The component encountered an error or isn't working as expected.
1. **Exited**: The component has stopped and is no longer running.

The component controller determines health by combining multiple factors:

- **Evaluation health**: Whether the component's arguments evaluated successfully.
- **Runtime health**: Whether the component is running without errors.
- **Component-specific health**: Some components report their own health status (for example, `local.file` reports unhealthy if its target file is deleted).

A component's health is independent of the health of components it references.
A component can be healthy even if it references exports from an unhealthy component.

## Evaluation failures

When a component fails to evaluate its arguments (for example, due to invalid configuration or missing dependencies), the component controller marks it as unhealthy and logs the failure reason.

Critically, the component continues operating with its last valid configuration.
This prevents failures from cascading through your entire pipeline.

For example:

- If a `local.file` component watching an API key file encounters a temporary file access error, dependent components continue using the last successfully read API key.
- If a `discovery.kubernetes` component temporarily loses connection to the API server, scrapers continue monitoring the last discovered targets.

This graceful degradation keeps your monitoring operational even when individual components encounter temporary issues.

## In-memory traffic

Some components that expose HTTP endpoints, such as [`prometheus.exporter.unix`][prometheus.exporter.unix], support in-memory communication for improved performance.
When components within the same {{< param "PRODUCT_NAME" >}} process communicate, they can bypass the network stack entirely.

Benefits of in-memory traffic:

- **Performance**: Eliminates network overhead for local component communication.
- **Security**: No need for network-level authentication or TLS since communication stays within the process.
- **Reliability**: Avoids potential network-related failures between components.

The internal address defaults to `alloy.internal:12345`.
If this conflicts with a real network address, you can change it using the `--server.http.memory-addr` flag when running {{< param "PRODUCT_NAME" >}}.

Components must explicitly support in-memory traffic to use this feature.
Check individual component documentation to see if it's available.

## Configuration file updates

The `/-/reload` HTTP endpoint and the `SIGHUP` signal notify the component controller to reload the configuration file.
When reloading, the controller synchronizes the running components with the configuration file.
It removes components no longer defined and creates additional ones added to the file.
After reloading, the controller reevaluates all managed components.

## Next steps

Now that you understand how the component controller works, explore advanced component topics and monitoring:

- [Configure components][] - Learn how to write components the controller can manage
- [Expressions][] - Create dynamic configurations using functions and component references

For monitoring and troubleshooting:

- [Monitor the component controller][] - Track component health and performance
- [{{< param "PRODUCT_NAME" >}} run command][run] - Configuration options that affect the controller

[DAG]: https://en.wikipedia.org/wiki/Directed_acyclic_graph
[prometheus.exporter.unix]: ../../../reference/components/prometheus/prometheus.exporter.unix
[run]: ../../../reference/cli/run/
[Configure components]: ../configure-components/
[Expressions]: ../../expressions/
[Monitor the component controller]: ../../../troubleshoot/controller_metrics/

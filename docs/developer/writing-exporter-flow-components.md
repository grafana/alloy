# Create Prometheus Exporter Flow Components

This guide will walk you through the process of creating a new Prometheus exporter Flow component and best practices for implementing it.

It is required that the exporter has an existing [Agent integration](../sources/static/configuration/integrations/_index.md) in order to wrap it as a Flow component. In the future, we will drop this requirement and Flow components will expose the logic of the exporter directly.

Use the following exporters as a reference:
- [process_exporter](../../component/prometheus/exporter/process/process.go) - [documentation](../sources/flow/reference/components/prometheus.exporter.process.md)
- [blackbox_exporter](../../component/prometheus/exporter/blackbox/blackbox.go) - [documentation](../sources/flow/reference/components/prometheus.exporter.blackbox.md)
- [node_exporter](../../component/prometheus/exporter/unix/unix.go) - [documentation](../sources/flow/reference/components/prometheus.exporter.unix.md)

## Arguments (Configuration)

`Arguments` struct defines the arguments that can be passed to the component. In most cases, this would be exactly the same as the arguments that the integration for this exporter uses. Some recommendations:

- Use `attr` tag for representing values. Use `attr,optional` tag for optional arguments.
- Use `alloytypes.Secret` type for sensitive arguments (e.g. API keys, passwords, etc). The original integration should have a similar field type called `Secret` from Prometheus.
- Use `block` tag for representing nested values such slices or structs. For example, the [process_exporter](../../component/prometheus/exporter/process/process.go) `Arguments` struct has `ProcessExporter` param which is a `[]MatcherGroup`. The name of the parameter should be in singular. This will allow the user to define multiple blocks of the same type.

The config would look like this using `matcher` block multiple times:

```river
prometheus.exporter.process "example" {
  track_children = false
  matcher {
    comm = ["grafana-agent"]
  }
  matcher {
    comm = ["firefox"]
  }
}
```

- Use `label` tag in field of struct represented as block to define named blocks. For example, the [blackbox_exporter](../../component/prometheus/exporter/blackbox/blackbox.go) `BlackboxTarget` struct has a `Name` param which represents the name of the block.

The config would look like this:

```river
prometheus.exporter.blackbox "example" {
    config_file = "blackbox_modules.yml"

    target {
        name    = "example"
        address = "http://example.com"
        module  = "http_2xx"
    }
}
```

- Define `DefaultArguments` as a global variable to define the default arguments for the component.

## Functions

- Define `init` function to register the component using `component.Register`.
  - The `Build` param should be a function that returns a `component.Component` interface.
  - The name used in the second parameter of `exporter.New` when defining the `Build` function it's important as it will define the label `job` in the form of `integrations/<name>`.
  - Avoid creating components with `Singleton: true` as it will make it impossible to run multiple instances of the exporter.

- If the exporter follows the multi-target pattern, add a function to define Prometheus discovery targets and use `exporter.NewWithTargetBuilder` for the `Build` param of the `component.Register` function.

- If the exporter implements a custom `InstanceKey`, add a function to customize the value of the instance label and use `exporter.NewWithTargetBuilder` for the `Build` param of the `component.Register` function.

- Define the `SetToDefault` function implementing syntax.Defaulter to specify the default arguments for the component.

- Define the `Validate` function implementing syntax.Validator to specify any validation rules for the component arguments.

- Add a test to validate the unmarshalling covering as many cases as possible.

- Define a `Convert` function to convert nested structs to the ones that the integration uses. Please, also add a test to validate the conversion covering as many cases as possible.

## Registering the component

In order to make the component visible for Agent Flow, it needs to be added to [all.go](../../component/all/all.go) file.

## Documentation

Writing the documentation for the component is very important. Please, follow the [Writing documentation for Flow components](./writing-flow-component-documentation.md) and take a look at the existing documentation for other exporters.

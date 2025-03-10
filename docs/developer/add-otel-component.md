# Add OpenTelemetry collector components to Grafana Alloy

Alloy is a vendor-neutral distribution of the OpenTelemetry Collector primarily maintained by Grafana Labs.
To read more about what's great about Grafana Alloy, refer to the introductory [Alloy documentation](https://grafana.com/docs/alloy/latest/introduction/).
You can dig deeper into the many components that come supported out of the box, from receivers, exporters, processors, and more, from the OpenTelemetry Collector repositories to embedded Prometheus exporters and the high-performance native Prometheus pipeline or native Loki logging pipelines.

This document focuses on adding additional OpenTelemetry Collector components from the upstream OpenTelemetry repositories that aren't shipped with Alloy by default.

It's straightforward to add a component to Alloy, whether for your personal use or to contribute it back to the community.
This document uses a simple example to show how you add a component to Alloy that exists in an upstream OpenTelemetry Collector repository.
You can apply this information to add any part of an OpenTelemetry pipeline to Alloy.

## Before you begin

The OpenTelemetry component must be the same version as the OpenTelemetry components included in the version of Alloy you are using.
You can find out which version of OpenTelemetry components are included by inspecting the `go.mod` module management file.

Each component added to Alloy has a stability level that maps to a [release life cycle](https://grafana.com/docs/release-life-cycle/) level.
When you add an OpenTelemetry component, try to match it to the component's stability level within the **Core** or **Contrib** repository.
You can modify the stability level of the component based on maturity, stability, and expectations of support.
When you add your component, you must pick a stability level, **Experimental**, **Public Preview**, or **Generally Available**, and provide it in the component registration.
If the stability of a component isn't set to **Generally Available**, you must [run Alloy](https://grafana.com/docs/alloy/latest/reference/cli/run/#the-run-command) with the `--stability.level` flag to enable the relevant stability level.

You'll need to select a unique identifier for your component.
Most Alloy components fall into one of several [categories](https://grafana.com/docs/alloy/latest/reference/components/).
For this example, you can assume you're adding a fictional OpenTelemetry processor named `exampleprocessor`.
By Alloy convention, you would refer to that component as `otelcol.processor.example`.

## Wrap the component

To create a component, you'll need a [fork](https://github.com/grafana/alloy/fork) of Alloy to work on.
If you already have a fork you can work on, make sure it's up to date with the current version of Alloy, or set it to the Alloy version to which you want to add your component.

Create a directory for your component in `internal/component/otelcol/processor/example`.

You can begin by registering the component and defining the arguments.
Each component registers itself with the Alloy runtime in an `init()` function with a few details.
The component must provide a unique identifier as its `Name`, a stability level, an arguments struct, a set of exports, and a factory-style build function.
While most of these are familiar to developers in the OpenTelemetry Collector ecosystem, exports are an [Alloy feature](https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/expressions/referencing_exports/) that allows components to affect the behavior of other components and provide methods for passing telemetry between components.
For OpenTelemetry components, you only need to use the predefined `otelcol.ConsumerExports{}` struct as the only exports used in an OpenTelemetry pipeline are `input` exports used to pass telemetry.
Other component-specific exports aren't necessary as they're not part of the paradigm of the OpenTelemetry collector.

```go
import (
    ...
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/exampleprocessor"
    ...
    )

func init() {
        component.Register(component.Registration{
        Name:      "otelcol.processor.example",
        Stability: featuregate.StabilityGenerallyAvailable,
        Args:      Arguments{},
        Exports:   otelcol.ConsumerExports{},
        Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
            return processor.New(opts, exampleprocessor.NewFactory(), args.(Arguments))
        },
    })
}
```

The arguments struct defines the user-facing configuration of the component.
In this case, this `example` processor has five attributes.
Attribute and value are strings and aren't marked as optional.
The three `Affect` booleans are optional.
The example also has two `blocks` defined which are standard parts of an OpenTelemetry component in Alloy.

* `Output`: Defines the next components to receive each type of telemetry from this component.
* `DebugMetrics`: Allows users to modify the behavior of the internal Alloy telemetry.

```go
type Arguments struct {
    Attribute string `alloy:"attribute,attr"`
    Value string `alloy:"value,attr"`

    AffectLogs bool `alloy:"affect_logs,attr,optional"`
    AffectMetrics bool `alloy:"affect_metrics,attr,optional"`
    AffectTraces bool `alloy:"affect_traces,attr,optional"`

    // Output configures where to send processed data. Required.
    Output *otelcol.ConsumerArguments `alloy:"output,block"`

    // DebugMetrics configures component internal metrics. Optional.
    DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}
```

Now that you have defined the `Arguments`, there are a few more things you need, to round out the processor definition.
You can start with a few compile time assertions, features of the Go language that confirm that the struct satisfies various interface definitions.
The rest of the functions shown are the functions required to satisfy each of those interfaces.
This includes setting default attribute values, validating attribute values that come from user configuration, and converting the Alloy `Arguments` struct into the correct struct for the OpenTelemetry component configuration.

```go
// Compile time assertions
var (
    _ processor.Arguments = Arguments{}
    _ syntax.Defaulter = &Arguments{}
    _ syntax.Validator = &Arguments{}
)

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
    AffectLogs: true,
    AffectMetrics: true,
    AffectTraces: true,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
    *args = DefaultArguments
    args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
    if args.Attribute == "" {
        return fmt.Errorf("attribute must not be empty")
    }
    return nil
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
    return &exampleprocessor.Config{
        Attribute: args.Attribute,
        Value: args.Value,
        AffectLogs: args.AffectLogs,
        AffectMetrics: args.AffectMetrics,
        AffectTraces: args.AffectTraces,
    }, nil
}

// Extensions implements processor.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
    return nil
}

// Exporters implements processor.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
    return nil
}

// NextConsumers implements processor.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
    return args.Output
}

// DebugMetricsConfig implements processor.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
    return args.DebugMetrics
}
```

Now that you have implemented a wrapper around the OpenTelemetry processor you want to add to Alloy, you need to add it to the list of all components to ensure that the `init()` function is called that registers the definition with the runtime.
To do this, import the wrapper in the `internal/component/all.go` file.

```go
_ "github.com/grafana/alloy/internal/component/otelcol/processor/example"                 // Import otelcol.processor.example
```

If you want to use the component for your personal or organizational use cases, you're done.
The Alloy [Makefile](https://github.com/grafana/alloy/blob/main/Makefile) provides the various ways to build executables and container images for running Alloy in your environments.
However, if you're looking to contribute back to the Alloy community, there's a few more steps you need to do

## Make a contribution

The Alloy ecosystem already supports many components from the OpenTelemetry Collector and Collector Contrib repositories and we would greatly appreciate any contributions that would add support for more upstream OpenTelemetry components.
If your OpenTelemetry component isn't in an upstream OpenTelemetry repository we will consider any contributions, but are also likely to recommend you contribute your component to OpenTelemetry first.

The most important additional step for a component within the Alloy ecosystem is documentation.
We provide in-depth reference documentation for each component in Alloy.
The documentation for this example component is a Markdown file that you must add as `docs/sources/reference/components/otelcol/otelcol.processor.example.md`.
The documentation should include an overview of the processor, its configuration arguments, and at least one example configuration.
There are many examples to look through in the Alloy documentation that you can use as a starting point.

We don't expect a large number of unit tests when you add something like an OpenTelemetry component that already has its own suite of tests upstream in the OpenTelemetry Collector repository, but there should at least be basic tests for the arguments struct, including its defaults and validation.

The current support for the OpenTelemetry configuration YAML requires that you write a simple converter to convert from YAML to Alloy configuration syntax.
For this example, you add it as `internal/converter/otelconvert/converter\_exampleprocessor.go`.
It must convert an incoming OpenTelemetry configuration into the appropriate Alloy `Arguments` struct.
There are many examples of this in the codebase that you can utilize to kickstart your component's converter.

Grafana Labs provides commercial support for Alloy.
If Grafana Labs can't offer commercial support for the component you are seeking to contribute to Alloy we recommend that you mark it as a [community component](https://grafana.com/docs/alloy/latest/get-started/community_components/).
Current community components include the OpenTelemetry DataDog Exporter and OpenTelemetry Splunk HEC Exporter.
While these components are a welcome part of the official releases of Alloy, we require users to opt-in to enable them in Alloy.
You can find [detailed Alloy developer documentation](https://github.com/grafana/alloy/blob/main/docs/developer/adding-community-components.md) about the process for proposing a community component if the component you are contributing falls under that category.

### Example contributions

The following list provides some examples of OpenTelemetry components added by both Grafana Labs employees and Alloy community members.
These should provide good examples of pull requests that follow the guidelines above, as well as examples of more complex components than the `example` processor above.

* [`otelcol.receiver.filelog`](https://github.com/grafana/alloy/pull/2711)  
* [`otelcol.processor.cumulativetodelta`](https://github.com/grafana/alloy/pull/2689)  
* [`otelcol.receiver.tcplog`](https://github.com/grafana/alloy/pull/2701)  
* [`otelcol.receiver.awscloudwatch`](https://github.com/grafana/alloy/pull/2822)

## Example configuration

Now that you have your Alloy executable or image, you can configure the processor in Alloy configuration syntax and add it to your telemetry pipelines.
The following example shows a sample configuration of the processor, along with an `otelcol.receiver.filelog` component that would send logs to it and an `otelcol.exporter.debug` component that would receive telemetry from it.

```alloy
otelcol.receiver.filelog "default" {
    include = ["/var/log/syslog"]
    output {
        logs = [otelcol.processor.example.default.input]
    }
}

otelcol.processor.example "default" {
    attribute = "test"
    value = "example.com"
    affect_traces = false

    output {
        logs = [otelcol.exporter.debug.default.input]
        metrics = [otelcol.exporter.debug.default.input]
        traces = [otelcol.exporter.debug.default.input]
    }
}

otelcol.exporter.debug "default" {}
```

## Reach out

Let us know if you want to add components to Alloy or any other Alloy-related topic.
You can find us most easily in the `#alloy` channel in the Grafana [community slack](https://slack.grafana.com/) or by raising a [GitHub issue](https://github.com/grafana/alloy/issues/new?template=feature_request.yaml).
We also have monthly community calls that you can participate in.
You can find more details in Slack or in the [community calendar](https://calendar.google.com/calendar/u/0/embed?src=grafana.com_n57lluqpn4h4edroeje6199o00@group.calendar.google.com).

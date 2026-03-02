---
canonical: https://grafana.com/docs/alloy/latest/get-started/expressions/function_calls/
aliases:
  - ./concepts/configuration-syntax/expressions/function_calls/ # /docs/alloy/latest/concepts/configuration-syntax/expressions/function_calls/
  - ../configuration-syntax/expressions/function_calls/ # /docs/alloy/latest/get-started/configuration-syntax/expressions/function_calls/
description: Learn about function calls
title: Function calls
weight: 40
---

# Function calls

You learned how to use operators to manipulate and combine values in your expressions in the previous topic.
Now you'll learn how to use function calls to transform data, interact with the system, and create more sophisticated dynamic configurations.

Functions are essential building blocks that extend what you can accomplish with expressions:

1. **Transform data**: Parse JSON, decode Base64, format strings, and convert between data types.
1. **Access system information**: Read environment variables, paths, and system properties.
1. **Process collections**: Combine arrays, group data, and extract specific elements.
1. **Enhance component logic**: Use functions within component arguments alongside operators and references.

The component controller evaluates function calls as part of expression evaluation, ensuring that function results integrate seamlessly with operators and component references.

## How functions work

Functions take zero or more arguments as input and always return a single value as output.
The component controller evaluates function calls during expression evaluation, following these steps:

1. **Evaluate arguments**: All function arguments are evaluated first, from left to right.
1. **Type checking**: The function validates that arguments match expected types.
1. **Execute function**: The function performs its operation and returns a result.
1. **Integration**: The result becomes available for use with operators or assignment to component attributes.

{{< param "PRODUCT_NAME" >}} provides functions through two sources:

1. **Standard library functions**: Built-in functions available in any configuration.
1. **Component exports**: Functions exported by components in your configuration.

You can't create your own functions in {{< param "PRODUCT_NAME" >}} configuration files.

If a function fails during evaluation, {{< param "PRODUCT_NAME" >}} stops processing the expression and reports an error.
This fail-fast behavior prevents invalid data from propagating through your configuration and helps you identify issues quickly.

## Standard library functions

The {{< param "PRODUCT_NAME" >}} configuration syntax includes a [standard library][] of functions organized into namespaces.
These functions fall into several categories:

1. **System functions**: Interact with the host system, like reading environment variables.
1. **Encoding functions**: Transform data between different formats like JSON and YAML.
1. **String functions**: Manipulate and format text values.
1. **Array functions**: Work with lists of values and perform collection operations.
1. **Convert functions**: Transform data between different types.

### Common examples

The following examples show frequently used standard library functions combined with component references and operators:

```alloy
// System functions - get environment variables
log_level = sys.env("LOG_LEVEL")
data_dir = sys.env("DATA_PATH") + "/metrics"

// Encoding functions - parse configuration data
config_data = encoding.from_json(local.file.config.content)
metrics_config = encoding.from_yaml(local.file.metrics.content)

// Extract and combine values
service_name = config_data["service"]["name"]
full_address = config_data["host"] + ":" + string.format("%d", config_data["port"])

// String functions - format dynamic values
job_name = string.format("%s-%s", service_name, sys.env("ENVIRONMENT"))
log_file = string.join(["/var/log", service_name, "app.log"], "/")

// Array functions - combine target lists
all_targets = array.concat(
  discovery.kubernetes.services.targets,
  discovery.file.local.targets,
  [{ "__address__" = "localhost:9090" }]
)
```

## Component export functions

Components can export functions that other components can call.
This creates reusable logic and enables sharing computed values across your configuration.

Component-exported functions work the same way as standard library functions:

1. **Discovery and referencing**: Use component references to access exported functions.
1. **Function calls**: Call the functions with appropriate arguments.
1. **Result integration**: Use function results in expressions with operators and other components.

For example, a custom component might export functions that:

1. **Validate data**: Check if configuration values meet specific criteria.
1. **Transform metrics**: Convert metric formats or apply mathematical operations.
1. **Generate dynamic values**: Create timestamps, UUIDs, or computed identifiers.
1. **Process collections**: Filter, sort, or group data for downstream components.

```alloy
// Example: Using a function exported by a custom component
processed_targets = data_processor.custom.transform_targets(
  discovery.kubernetes.services.targets,
  { "environment" = sys.env("ENV"), "region" = "us-west-2" }
)

// Combine function results with operators and references
final_config = {
  "targets" = processed_targets,
  "labels" = custom_labeler.instance.generate_labels(service_name) + base_labels,
  "enabled" = validator.config.is_valid(processed_targets) && sys.env("MONITORING_ENABLED") == "true"
}
```

## Next steps

Now that you understand function calls, continue learning about expressions:

- [Types and values][type] - Learn how the type system ensures function arguments and return values work correctly

For building complete configurations:

- [Component exports][refer to values] - Use function calls with component references to create sophisticated pipelines
- [Operators][operators] - Combine function results with operators for complex expressions

For detailed function reference:

- [Standard library reference][standard library] - Complete documentation of all available functions and their usage

[type]: ../types_and_values/
[refer to values]: ../referencing_exports/
[operators]: ../operators/
[standard library]: ../../../reference/stdlib/

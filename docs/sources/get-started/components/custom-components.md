---
canonical: https://grafana.com/docs/alloy/latest/get-started/components/custom-components/
aliases:
  - ../custom_components/ # /docs/alloy/latest/get-started/custom_components/
  - ../concepts/custom_components/ # /docs/alloy/latest/concepts/custom_components/
description: Learn about custom components
title: Custom components
weight: 40
---

# Custom components

You learned how the component controller manages the lifecycle of components in the previous section.
Now you'll learn how to create your own reusable components by combining existing components into _custom components_.

Custom components allow you to package a pipeline of built-in and other custom components into a single, reusable unit.
This makes it easier to share common patterns across your configurations and across teams.

A custom component includes:

1. **Arguments**: Settings that configure the custom component's behavior.
1. **Exports**: Values the custom component exposes to consumers.
1. **Components**: Built-in and custom components that run as part of the custom component.

## Create custom components

Use the [`declare` configuration block][declare] to create a custom component.
The block's label becomes the custom component's name, which you can then use like any built-in component.

Inside a `declare` block, you can use these configuration blocks:

- [`argument`][argument]: Defines a named input parameter that users must provide when using your custom component. Access the value with `argument.NAME.value`.
- [`export`][export]: Defines a named output value that your custom component exposes to other components.
- Built-in and custom components: The actual pipeline logic that processes data.

The component controller treats custom components the same as built-in components:

1. **Evaluation**: Arguments are evaluated and passed to the internal pipeline.
1. **Execution**: The internal components run and process data.
1. **Export updates**: When internal components update their exports, the custom component can update its own exports.
1. **Health reporting**: The custom component's health reflects the health of its internal components.

Custom components are particularly useful for packaging common pipeline patterns that you want to reuse across multiple configurations.

## Example

This example creates a custom component called `add` that demonstrates the key concepts:

```alloy
declare "add" {
    argument "a" {
        comment = "First number to add"
        optional = false
    }

    argument "b" {
        comment = "Second number to add"
        optional = false
    }

    export "sum" {
        value = argument.a.value + argument.b.value
    }
}

add "example" {
    a = 15
    b = 17
}

// Reference the export: add.example.sum == 32
```

This custom component:

1. **Declares two required arguments**: `a` and `b` with descriptive comments.
1. **Exports one value**: `sum` that computes the addition of the two arguments.
1. **Can be instantiated**: Create an instance labeled `example` by providing values for `a` and `b`.
1. **Exposes computed results**: Other components can reference `add.example.sum` to use the result.

For more complex examples that combine multiple built-in components, refer to the [`declare` block reference][declare].

## Next steps

Now that you understand custom components, explore related topics and advanced usage:

- [Configuration blocks reference][] - Learn about `declare`, `argument`, and `export` blocks in detail
- [Configure components][] - Use custom components in your pipelines

For sharing and organizing:

- [Modules][] - Share custom components across multiple configuration files

[declare]: ../../../reference/config-blocks/declare/
[argument]: ../../../reference/config-blocks/argument/
[export]: ../../../reference/config-blocks/export/
[Modules]: ../../modules/
[Configuration blocks reference]: ../../../reference/config-blocks/
[Configure components]: ../configure-components/

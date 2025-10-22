---
canonical: https://grafana.com/docs/alloy/latest/get-started/custom_components/
aliases:
  - ../concepts/custom_components/ # /docs/alloy/latest/concepts/custom_components/
description: Learn about custom components
title: Custom components
weight: 300
---

# Custom components

_Custom components_ are a way to create new components from a pipeline of built-in and other custom components.

A custom component includes:

* _Arguments_: Settings that configure the custom component.
* _Exports_: Values the custom component exposes to its consumers.
* _Components_: Built-in and custom components that run as part of the custom component.

## Create custom components

Use [the `declare` configuration block][declare] to create a new custom component.
The block's label specifies the custom component's name.

You can use the following configuration blocks inside a `declare` block:

* [argument][]: Define a named argument whose current value you can reference using the expression `argument.NAME.value`.
  The user of the custom component determines argument values.
* [export][]: Define a named value to expose to custom component users.

Custom components are helpful for reusing a common pipeline multiple times.
To learn how to share custom components across files, refer to [Modules][].

## Example

This example creates a custom component called `add`, which exports the sum of two arguments:

```alloy
declare "add" {
    argument "a" { }
    argument "b" { }

    export "sum" {
        value = argument.a.value + argument.b.value
    }
}

add "example" {
    a = 15
    b = 17
}

// add.example.sum == 32
```

[declare]: ../../reference/config-blocks/declare/
[argument]: ../../reference/config-blocks/argument/
[export]: ../../reference/config-blocks/export/
[Modules]: ../modules/

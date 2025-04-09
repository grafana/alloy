---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/argument/
description: Learn about the argument configuration block
menuTitle: argument
title: argument block
---

# argument block

`argument` is an optional configuration block used to specify parameterized input to a [custom component][].
`argument` blocks must be given a label which determines the name of the argument.

The `argument` block may only be specified inside the definition of [a `declare` block][declare].

## Example

```alloy
argument "ARGUMENT_NAME" {}
```

## Arguments

{{< admonition type="note" >}}
For clarity, "argument" in this section refers to arguments which can be given to the argument block.
"Module argument" refers to the argument being defined for a module, determined by the label of the argument block.
{{< /admonition >}}

The following arguments are supported:

Name       | Type     | Description                          | Default | Required
-----------|----------|--------------------------------------|---------|---------
`comment`  | `string` | Description for the argument.        | `false` | no
`default`  | `any`    | Default value for the argument.      | `null`  | no
`optional` | `bool`   | Whether the argument may be omitted. | `false` | no

By default, all module arguments are required.
The `optional` argument can be used to mark the module argument as optional.
When `optional` is `true`, the initial value for the module argument is specified by `default`.

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type  | Description
--------|-------|-----------------------------------
`value` | `any` | The current value of the argument.

If you use a custom component, you are responsible for determining the values for arguments.
Other expressions within a custom component may use `argument.ARGUMENT_NAME.value` to retrieve the value you provide.

## Example

This example creates a custom component that self-collects process metrics and forwards them to an argument specified by the user of the custom component:

```alloy
declare "self_collect" {
  argument "metrics_output" {
    optional = false
    comment  = "Where to send collected metrics."
  }

  prometheus.scrape "selfmonitor" {
    targets = [{
      __address__ = "127.0.0.1:12345",
    }]

    forward_to = [argument.metrics_output.value]
  }
}
```

[custom component]: ../../../get-started/custom_components/
[declare]: ../../config-blocks/declare/

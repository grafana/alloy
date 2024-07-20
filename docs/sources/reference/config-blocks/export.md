---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/export/
description: Learn about the export configuration block
menuTitle: export
title: export block
---

# export block

`export` is an optional configuration block used to specify an emitted value of a [custom component][].
`export` blocks must be given a label which determine the name of the export.

The `export` block may only be specified inside the definition of [a `declare` block][declare].

## Example

```alloy
export "ARGUMENT_NAME" {
  value = ARGUMENT_VALUE
}
```

## Arguments

The following arguments are supported:

Name    | Type  | Description      | Default | Required
--------|-------|------------------|---------|---------
`value` | `any` | Value to export. |         | yes

The `value` argument determines what the value of the export is.
To expose an exported field of another component, set `value` to an expression that references that exported value.

## Exported fields

The `export` block doesn't export any fields.

## Example

This example creates a custom component where the output of discovering Kubernetes pods and nodes are exposed to the user:

```alloy
declare "pods_and_nodes" {
  discovery.kubernetes "pods" {
    role = "pod"
  }

  discovery.kubernetes "nodes" {
    role = "nodes"
  }

  export "kubernetes_resources" {
    value = concat(
      discovery.kubernetes.pods.targets,
      discovery.kubernetes.nodes.targets,
    )
  }
}
```

[custom component]: ../../../get-started/custom_components/
[declare]: ../declare/

---
aliases:
- /docs/grafana-cloud/agent/flow/reference/config-blocks/export/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/config-blocks/export/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/config-blocks/export/
- /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/export/
canonical: https://grafana.com/docs/agent/latest/flow/reference/config-blocks/export/
description: Learn about the export configuration block
menuTitle: export
title: export block
---

# export block

`export` is an optional configuration block used to specify an emitted value of a [custom component][].
`export` blocks must be given a label which determine the name of the export.

The `export` block may only be specified inside the definition of [a `declare` block][declare].

{{< admonition type="note" >}}
In [classic modules][], the `export` block is valid as a top-level block in a classic module. Classic modules are deprecated and scheduled to be removed in the release after v0.40.

[classic modules]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/concepts/modules/#classic-modules-deprecated
{{< /admonition >}}

## Example

```river
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

```river
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

{{% docs/reference %}}
[custom component]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/custom_components"
[custom component]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/custom_components"
[declare]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/config-blocks/declare"
[declare]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/declare"
{{% /docs/reference %}}

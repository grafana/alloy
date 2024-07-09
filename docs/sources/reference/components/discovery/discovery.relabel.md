---
canonical: https://grafana.com/docs/alloy/latest/reference/components/discovery/discovery.relabel/
aliases:
  - ../discovery.relabel/ # /docs/alloy/latest/reference/components/discovery.relabel/
description: Learn about discovery.relabel
title: discovery.relabel
---

# discovery.relabel

In {{< param "PRODUCT_NAME" >}}, targets are defined as sets of key-value pairs called _labels_.

`discovery.relabel` rewrites the label set of the input targets by applying one or more relabeling rules.
If no rules are defined, then the input targets are exported as-is.

The most common use of `discovery.relabel` is to filter targets or standardize the target label set that's passed to a downstream component.
The `rule` blocks are applied to the label set of each target in order of their appearance in the configuration file.
The configured rules can be retrieved by calling the function in the `rules` export field.

Target labels which start with a double underscore `__` are considered internal, and may be removed by other components prior to telemetry collection.
To retain any of these labels, use a `labelmap` action to remove the prefix, or remap them to a different name.
Service discovery mechanisms usually group their labels under `__meta_*`.
For example, the discovery.kubernetes component populates a set of `__meta_kubernetes_*` labels to provide information about the discovered Kubernetes resources.
If a relabeling rule needs to store a label value temporarily, for example as the input to a subsequent step, use the `__tmp` label name prefix, as it's guaranteed to never be used.

Multiple `discovery.relabel` components can be specified by giving them different labels.

## Usage

```alloy
discovery.relabel "LABEL" {
  targets = TARGET_LIST

  rule {
    ...
  }

  ...
}
```

## Arguments

The following arguments are supported:

Name      | Type                | Description        | Default | Required
----------|---------------------|--------------------|---------|---------
`targets` | `list(map(string))` | Targets to relabel |         | yes

## Blocks

The following blocks are supported inside the definition of
`discovery.relabel`:

Hierarchy | Block    | Description                           | Required
----------|----------|---------------------------------------|---------
rule      | [rule][] | Relabeling rules to apply to targets. | no

[rule]: #rule-block

### rule block

{{< docs/shared lookup="reference/components/rule-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name     | Type                | Description
---------|---------------------|----------------------------------------------
`output` | `list(map(string))` | The set of targets after applying relabeling.
`rules`  | `RelabelRules`      | The currently configured relabeling rules.

## Component health

`discovery.relabel` is only reported as unhealthy when given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`discovery.relabel` does not expose any component-specific debug information.

## Debug metrics

`discovery.relabel` does not expose any component-specific debug metrics.

## Example

```alloy
discovery.relabel "keep_backend_only" {
  targets = [
    { "__meta_foo" = "foo", "__address__" = "localhost", "instance" = "one",   "app" = "backend"  },
    { "__meta_bar" = "bar", "__address__" = "localhost", "instance" = "two",   "app" = "database" },
    { "__meta_baz" = "baz", "__address__" = "localhost", "instance" = "three", "app" = "frontend" },
  ]

  rule {
    source_labels = ["__address__", "instance"]
    separator     = "/"
    target_label  = "destination"
    action        = "replace"
  }

  rule {
    source_labels = ["app"]
    action        = "keep"
    regex         = "backend"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`discovery.relabel` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)

`discovery.relabel` has exports that can be consumed by the following components:

- Components that consume [Targets](../../../compatibility/#targets-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

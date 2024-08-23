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

Hierarchy         | Block         | Description                                       | Required
------------------|---------------|---------------------------------------------------|---------
rule              | [rule][]      | Relabeling rules to apply to targets.             | no
join              | [join][]      | Joined labels from other lists of targets.        | no
join > condition  | [condition][] | Only join the labels if the condition is true.    | yes

[rule]: #rule-block
[join]: #join-block

### rule block

{{< docs/shared lookup="reference/components/rule-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### join block

<!-- TODO: Add a similar block to loki.relabel and prometheus.relabel. 
           Enriching logs in such a way might be especially useful. -->

The `join` block allows you to add additional labels to the list of `targets`.

The following arguments are supported:

| Name               | Type           | Description                                       | Default  | Required |
|--------------------|----------------|---------------------------------------------------|----------|----------|
| `incoming_targets` | `targets`      | A list of targets whose labels to merge.          |          | yes      |
| `labels_to_join`   | `list(string)` | Labels from `targets` which to join.              | `[]`     | no       |
| `action`           | `string`       | Whether to update the label if it already exists. | `upsert` | no

`incoming_targets` contains labels which could come from a `discovery` or a `prometheus.exporter` component.

Only the labels in `labels_to_join` will be appended to the current targets.
Labels which are in `labels_to_join`, but not in `incoming_targets` will be ignored.

The supported values for `action` are:
* `insert`: Inserts a new label if the label does not already exist. 
  If the label already exists, it is not modified.
* `update`: Updates a label if it already exists. 
  If the label does not exist, it will not be created.
* `upsert`: Inserts a new label if the label does not already exist.
  If a label already exists, its value will be updated.

When `incoming_targets` are received, they will be stripped of any labels which are not mentioned in `labels_to_join` and `condition`.
Duplicate sets of targets in `incoming_targets` will then be removed.
We then iterate over every target in `targets`. If all of the `condition` blocks are satisfied, 
the labels listed in `labels_to_join` will be added to `targets` along with their values.

### condition block

The `condition` block will join labels from `incoming_targets` only if the value of a `label_incoming` matches the value of a `label_current` label.
Multiple `condition` blocks can be specified.

The following arguments are supported:

| Name             | Type     | Description                             | Default | Required |
|------------------|----------|-----------------------------------------|---------|----------|
| `label_incoming` | `string` | Name of a label in `incoming_targets`.  |         | yes      |
| `label_current`  | `string` | Name of a label in `targets`.           |         | yes      |

<!-- TODO: Are the arguments named well? What about "incoming label" -->

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

## Examples

### Using rules

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

### Joining targets from Kubernetes discovery

```alloy
prometheus.exporter.redis "example" {
  redis_addr = "localhost:6379"
}

discovery.kubernetes "k8s_pods" {
  role = "pod"
}

discovery.relabel "joining" {
  targets = prometheus.exporter.redis.example.targets

  join {
    incoming_targets = discovery.kubernetes.k8s_pods.targets
    labels_to_join   = "__meta_kubernetes_namespace"

    // You can use multiple condition blocks within the same join block
    condition {
      label_incoming = "__meta_kubernetes_pod_ip"
      label_current  = "__address__"
    }

    condition {
      label_incoming = "__meta_kubernetes_pod_name"

      // TODO: The "__redis_instance__" label is not real. Can we use a real one as an example?
      label_current  = "__redis_instance__"
    }
  }
}

prometheus.scrape "demo" {
  targets    = discovery.relabel.joining.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
  }
}
```

### Joining multiple targets are relabeling

This example demonstrates how you could use multiple interweaved `join` and `rule` blocks.
The blocks will be executed in the same sequence that they are configured.

<!-- TODO: Is this example realistic? If not - is there a more realistic one? -->

```alloy
prometheus.exporter.redis "example" {
  redis_addr = "localhost:6379"
}

discovery.kubernetes "k8s_pods" {
  role = "pod"
}

discovery.ec2 "example" {
}

discovery.relabel "joining" {
  targets = prometheus.exporter.redis.example.targets

  // TODO: Fill this with something
  rule {}

  join {
    incoming_targets = discovery.kubernetes.k8s_pods.targets
    labels_to_join   = "__meta_kubernetes_namespace"
    condition {
      label_incoming = "__meta_kubernetes_pod_ip"
      label_current  = "__address__"
    }
  }

  // TODO: Fill this with something
  rule {}

  join {
    incoming_targets = discovery.ec2.example.targets
    labels_to_join   = "__meta_ec2_availability_zone"
    condition {
      label_incoming = "__meta_ec2_public_ip"
      label_current  = "__address__"
    }
  }

  // TODO: Fill this with something
  rule {}
}

prometheus.scrape "demo" {
  targets    = discovery.relabel.joining.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = PROMETHEUS_REMOTE_WRITE_URL

    basic_auth {
      username = USERNAME
      password = PASSWORD
    }
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

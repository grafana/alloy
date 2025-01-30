---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.relabel/
aliases:
  - ../pyroscope.relabel/ # /docs/alloy/latest/reference/components/pyroscope.relabel/
description: Learn about pyroscope.relabel
title: pyroscope.relabel
---

# pyroscope.relabel

The `pyroscope.relabel` component rewrites the label set of each profile passed to its receiver by applying one or more relabeling `rule`s and forwards the results to the list of receivers in the component's arguments.

If no rules are defined or applicable to some profiles, then those profiles are forwarded as-is to each receiver passed in the component's arguments. If no labels remain after the relabeling rules are applied, then the profile is dropped.

The most common use of `pyroscope.relabel` is to filter profiles or standardize the label set that is passed to one or more downstream receivers. The `rule` blocks are applied to the label set of each profile in order of their appearance in the configuration file.

## Usage

```alloy
pyroscope.relabel "process" {
    forward_to = RECEIVER_LIST

    rule {
        ...
    }

    ...
}
```

## Arguments

The following arguments are supported:

| Name | Type | Description | Default | Required |
| ---- | ---- | ----------- | ------- | -------- |
| `forward_to` | `list(pyroscope.Appendable)` | List of receivers to forward profiles to after relabeling | | yes |
| `max_cache_size` | `number` | Maximum number of entries in the label cache | 10000 | no |

## Blocks

The following blocks are supported inside the definition of `pyroscope.relabel`:

Hierarchy | Name     | Description                                        | Required
----------|----------|----------------------------------------------------|---------
rule      | [rule][] | Relabeling rules to apply to received log entries. | no

[rule]: #rule-block

### rule block

{{< docs/shared lookup="reference/components/rule-block-logs.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
-----|------|------------
`receiver` | `ProfilesReceiver` | A receiver that accepts profiles for relabeling.
`rules` | `[]relabel.Config` | The list of relabeling rules.

## Component health

`pyroscope.relabel` is reported as unhealthy if it is given an invalid configuration.

## Debug metrics

* `pyroscope_relabel_profiles_dropped` (counter): Total number of profiles dropped by relabeling rules.
* `pyroscope_relabel_profiles_processed` (counter): Total number of profiles processed.
* `pyroscope_relabel_profiles_written` (counter): Total number of profiles forwarded.
* `pyroscope_relabel_cache_misses` (counter): Total number of cache misses.
* `pyroscope_relabel_cache_hits` (counter): Total number of cache hits.
* `pyroscope_relabel_cache_size` (gauge): Total size of relabel cache.

## Example

```alloy
pyroscope.relabel "process" {
    forward_to = [pyroscope.write.backend.receiver]

    # Rules are applied in order
    rule {
        source_labels = ["env"]
        target_label = "environment"
        action = "replace"
        regex = "(.)"
        replacement = "$1"
    }

    rule {
        action = "labeldrop"
        regex = "env"
    }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->
## Compatible components

`pyroscope.relabel` can accept arguments from the following components:

- Components that export [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-exporters)

`pyroscope.relabel` has exports that can be consumed by the following components:

- Components that consume [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

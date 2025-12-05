---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.relabel/
aliases:
  - ../pyroscope.relabel/ # /docs/alloy/latest/reference/components/pyroscope.relabel/
description: Learn about pyroscope.relabel
labels:
  stage: general-availability
  products:
    - oss
title: pyroscope.relabel
---

# `pyroscope.relabel`

The `pyroscope.relabel` component rewrites the label set of each profile passed to its receiver by applying one or more relabeling rules and forwards the results to the list of receivers.

If no rules are defined or applicable to some profiles, then those profiles are forwarded as-is to each receiver passed in the component's arguments.
The profile is dropped if no labels remain after the relabeling rules are applied.

The most common use of `pyroscope.relabel` is to filter profiles or standardize the label set that is passed to one or more downstream receivers.
The `rule` blocks are applied to the label set of each profile in order of their appearance in the configuration file.

## Usage

```alloy
pyroscope.relabel "<LABEL>" {
    forward_to = <RECEIVER_LIST>

    rule {
        ...
    }

    ...
}
```

## Arguments

You can use the following arguments with `pyroscope.relabel`:

| Name             | Type                         | Description                                               | Default | Required |
| ---------------- | ---------------------------- | --------------------------------------------------------- | ------- | -------- |
| `forward_to`     | `list(pyroscope.Appendable)` | List of receivers to forward profiles to after relabeling |         | yes      |
| `max_cache_size` | `number`                     | Maximum number of entries in the label cache              | `10000` | no       |

## Blocks

You can use the following block with `pyroscope.relabel`:

|      Name      |                      Description                       | Required |
| -------------- | ------------------------------------------------------ | -------- |
| [`rule`][rule] | Relabeling rules to apply to received profile entries. | no       |

[rule]: #rule

### `rule`

{{< docs/shared lookup="reference/components/rule-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type               | Description                                      |
| ---------- | ------------------ | ------------------------------------------------ |
| `receiver` | `ProfilesReceiver` | A receiver that accepts profiles for relabeling. |
| `rules`    | `[]relabel.Config` | The list of relabeling rules.                    |

## Component health

`pyroscope.relabel` is reported as unhealthy if it is given an invalid configuration.

## Debug metrics

* `pyroscope_relabel_cache_hits` (counter): Total number of cache hits.
* `pyroscope_relabel_cache_misses` (counter): Total number of cache misses.
* `pyroscope_relabel_cache_size` (gauge): Total size of relabel cache.
* `pyroscope_relabel_profiles_dropped` (counter): Total number of profiles dropped by relabeling rules.
* `pyroscope_relabel_profiles_processed` (counter): Total number of profiles processed.
* `pyroscope_relabel_profiles_written` (counter): Total number of profiles forwarded.

## Example

```alloy
pyroscope.receive_http "default" {
    forward_to = [pyroscope.relabel.filter_profiles.receiver]

    http {
        listen_address = "0.0.0.0"
        listen_port = 9999
    }
}

pyroscope.relabel "filter_profiles" {
    forward_to = [pyroscope.write.staging.receiver]

    // This creates a consistent hash value (0 or 1) for each unique combination of labels
    // Using multiple source labels provides better sampling distribution across your profiles
    rule {
        source_labels = ["env"]
        target_label = "__tmp_hash"
        action = "hashmod"
        modulus = 2
    }

    // This effectively samples ~50% of profile series
    // The same combination of source label values will always hash to the same number,
    // ensuring consistent sampling
    rule {
        source_labels = ["__tmp_hash"]
        action       = "drop"
        regex        = "^1$"
    }
}

pyroscope.write "staging" {
  endpoint {
    url = "http://pyroscope-staging:4040"
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

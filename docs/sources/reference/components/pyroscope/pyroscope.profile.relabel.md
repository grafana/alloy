---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.profile.relabel/
aliases:
  - ../pyroscope.profile.relabel/ # /docs/alloy/latest/reference/components/pyroscope.profile.relabel/
description: Learn about pyroscope.profile.relabel
labels:
  stage: experimental
  products:
    - oss
title: pyroscope.profile.relabel
---

# `pyroscope.profile.relabel`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `pyroscope.profile.relabel` component rewrites labels embedded in [pprof][] samples by applying one or more relabeling rules, then forwards the transformed profiles to the configured receivers.

Unlike [`pyroscope.relabel`](../pyroscope.relabel/), this component parses each pprof payload and modifies in-profile sample labels.

{{< admonition type="caution" >}}
`pyroscope.profile.relabel` parses and re-encodes profile payloads.
This can increase CPU and memory usage, especially with high profile volume or large profiles.
{{< /admonition >}}

## Usage

```alloy
pyroscope.profile.relabel "<LABEL>" {
    forward_to = <RECEIVER_LIST>

    rule {
        ...
    }

    ...
}
```

## Arguments

You can use the following arguments with `pyroscope.profile.relabel`:

| Name             | Type                     | Description                                                              | Default | Required |
| ---------------- | ------------------------ | ------------------------------------------------------------------------ | ------- | -------- |
| `forward_to`     | `list(ProfilesReceiver)` | List of receivers to forward profiles to after in-profile relabeling.    |         | yes      |
| `max_cache_size` | `number`                 | Maximum number of entries in the sample-label relabel cache.             | `10000` | no       |

## Blocks

You can use the following block with `pyroscope.profile.relabel`:

|      Name      |                              Description                              | Required |
| -------------- | --------------------------------------------------------------------- | -------- |
| [`rule`][rule] | Relabeling rules to apply to labels embedded in each pprof sample.    | no       |

[rule]: #rule

### `rule`

{{< docs/shared lookup="reference/components/rule-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type               | Description                                                      |
| ---------- | ------------------ | ---------------------------------------------------------------- |
| `receiver` | `ProfilesReceiver` | A receiver that accepts profiles for in-profile relabeling.      |
| `rules`    | `[]relabel.Config` | The list of relabeling rules.                                    |

## Component health

`pyroscope.profile.relabel` is reported as unhealthy if it is given an invalid configuration.

## Component behavior

`pyroscope.profile.relabel` has the following behavior:

* Rules apply only to pprof sample string labels.
* Numeric sample labels are preserved as-is.
* If relabeling drops all string labels for a sample and the sample has no numeric labels, that sample is dropped.
* If all samples are dropped from a profile, the profile is dropped.
* Only pprof payloads are transformed.
* Payloads that cannot be parsed as pprof are forwarded unchanged.
* For multi-valued pprof string labels, only the first value is used for relabeling.

## Debug metrics

* `pyroscope_profile_relabel_cache_hits` (counter): Total number of cache hits.
* `pyroscope_profile_relabel_cache_misses` (counter): Total number of cache misses.
* `pyroscope_profile_relabel_cache_size` (gauge): Total size of relabel cache.
* `pyroscope_profile_relabel_pprof_parse_failures` (counter): Total number of profiles forwarded unchanged because pprof parsing failed.
* `pyroscope_profile_relabel_pprof_write_failures` (counter): Total number of profiles forwarded unchanged because pprof encoding failed.
* `pyroscope_profile_relabel_profiles_dropped` (counter): Total number of profiles dropped after relabeling.
* `pyroscope_profile_relabel_profiles_processed` (counter): Total number of profiles processed.
* `pyroscope_profile_relabel_profiles_written` (counter): Total number of profiles forwarded.
* `pyroscope_profile_relabel_samples_dropped` (counter): Total number of pprof samples dropped by relabeling rules.
* `pyroscope_profile_relabel_samples_processed` (counter): Total number of pprof samples processed.
* `pyroscope_profile_relabel_samples_written` (counter): Total number of pprof samples forwarded.

## Example

```alloy
pyroscope.scrape "default" {
  targets    = [{"__address__" = "localhost:4040", "service_name" = "my-app"}]
  forward_to = [pyroscope.profile.relabel.pprof_labels.receiver]
}

pyroscope.profile.relabel "pprof_labels" {
  forward_to = [pyroscope.write.default.receiver]

  rule {
    source_labels = ["thread_name"]
    target_label  = "thread"
    action        = "replace"
  }

  rule {
    action = "labeldrop"
    regex  = "^thread_name$"
  }
}

pyroscope.write "default" {
  endpoint {
    url = "http://pyroscope:4040"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`pyroscope.profile.relabel` can accept arguments from the following components:

- Components that export [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-exporters)

`pyroscope.profile.relabel` has exports that can be consumed by the following components:

- Components that consume [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

[pprof]: https://github.com/google/pprof/blob/main/proto/profile.proto

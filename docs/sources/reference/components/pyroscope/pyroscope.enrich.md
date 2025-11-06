---
canonical: https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.enrich/
description: Learn about pyroscope.enrich
labels:
  stage: experimental
  products:
    - oss
title: pyroscope.enrich
---

# `pyroscope.enrich`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`pyroscope.enrich` enriches profiles with additional labels from service discovery targets.
It matches a label from incoming profiles against a label from discovered targets, and copies specified labels from the matched target to the profile.

## Usage

```alloy
pyroscope.enrich "<LABEL>" {
  targets = <DISCOVERY_COMPONENT>.targets
  target_match_label = "<LABEL>"
  forward_to = [<RECEIVER_LIST>]
}
```

## Arguments

You can use the following arguments with `pyroscope.enrich`:

| Name                   | Type                     | Description                                                                                   | Default | Required |
| ---------------------- | ------------------------ | --------------------------------------------------------------------------------------------- | ------- | -------- |
| `forward_to`           | `list(ProfilesReceiver)` | List of receivers to send enriched profiles to.                                               |         | yes      |
| `target_match_label`   | `string`                 | The label from discovered targets to match against.                                           |         | yes      |
| `targets`              | `list(Target)`           | List of targets from a discovery component.                                                   |         | yes      |
| `labels_to_copy`       | `list(string)`           | List of labels to copy from discovered targets to profiles. If empty, all labels are copied. |         | no       |
| `profiles_match_label` | `string`                 | The label from incoming profiles to match against discovered targets.                         |         | no       |

If `profiles_match_label` isn't provided, `target_match_label` is used for matching profile labels.

## Blocks

`pyroscope.enrich` doesn't support any blocks.
Configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type                 | Description                         |
| ---------- | -------------------- | ----------------------------------- |
| `receiver` | `ProfilesReceiver`   | The receiver for profiles.          |

## Component health

`pyroscope.enrich` is only reported as unhealthy if given an invalid configuration.

## Debug information

`pyroscope.enrich` exposes debug information about its state and enrichment activity.

The following fields are available:

| Field                    | Type        | Description                                                                |
| ------------------------ | ----------- | -------------------------------------------------------------------------- |
| `targets_cached`         | `int`       | The number of discovery targets currently cached for enrichment.           |
| `profiles_processed`     | `uint64`    | The total number of profiles processed by the component.                   |
| `profiles_enriched`      | `uint64`    | The number of profiles that were successfully enriched with target labels. |
| `profiles_unmatched`     | `uint64`    | The number of profiles that couldn't be matched to a target.               |
| `last_enrichment_time`   | `time.Time` | The timestamp of the last successful enrichment.                           |
| `last_unmatched_value`   | `string`    | The match label value of the most recent unmatched profile.                |
| `recent_matches`         | `list`      | The 10 most recent successful matches with timestamp and labels added.     |

## Debug metrics

`pyroscope.enrich` doesn't expose additional metrics.

## Example

This example enriches profiles received over HTTP with metadata from Kubernetes service discovery:

```alloy
// Discover Kubernetes pods
discovery.kubernetes "pods" {
  role = "pod"
}

// Add custom labels from Kubernetes metadata
discovery.relabel "pods" {
  targets = discovery.kubernetes.pods.targets
  
  rule {
    source_labels = ["__meta_kubernetes_namespace"]
    target_label  = "namespace"
  }
  
  rule {
    source_labels = ["__meta_kubernetes_pod_node_name"]
    target_label  = "node"
  }
  
  rule {
    source_labels = ["__meta_kubernetes_pod_label_app"]
    target_label  = "app"
  }
  
  rule {
    source_labels = ["__meta_kubernetes_pod_label_environment"]
    target_label  = "environment"
  }
  
  rule {
    source_labels = ["__meta_kubernetes_pod_ip"]
    target_label  = "pod_ip"
  }
}

// Receive profiles over HTTP
pyroscope.receive_http "default" {
  http {
    listen_address = "0.0.0.0"
    listen_port    = 4040
  }
  forward_to = [pyroscope.enrich.metadata.receiver]
}

// Enrich profiles with Kubernetes metadata
pyroscope.enrich "metadata" {
  targets               = discovery.relabel.pods.output
  target_match_label    = "pod_ip"
  profiles_match_label  = "service_name"
  labels_to_copy        = ["namespace", "node", "app", "environment"]
  forward_to            = [pyroscope.write.default.receiver]
}

// Write profiles to Pyroscope
pyroscope.write "default" {
  endpoint {
    url = "http://pyroscope:4040"
  }
}
```

## Component behavior

The component matches profiles to discovered targets and enriches them with additional labels:

1. For each profile, it looks up the value of `profiles_match_label` from the profile's labels, or `target_match_label` if `profiles_match_label` isn't specified.
1. It matches this value against the `target_match_label` in discovered targets.
1. If a match is found, it copies the requested `labels_to_copy` from the discovered target to the profile. If `labels_to_copy` is empty, all labels are copied.
1. The profile, enriched or unchanged, is forwarded to the configured receivers.

{{< admonition type="caution" >}}
By default, `pyroscope.enrich` is ready as soon as it starts, even if no targets are discovered.
If profiles are sent to this component before the metadata is synced, they're passed through as-is, without enrichment.
This is most likely to impact `pyroscope.enrich` on startup for a short time before the discovery components send a new list of targets.
{{< /admonition >}}

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`pyroscope.enrich` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-exporters)

`pyroscope.enrich` has exports that can be consumed by the following components:

- Components that consume [Pyroscope `ProfilesReceiver`](../../../compatibility/#pyroscope-profilesreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

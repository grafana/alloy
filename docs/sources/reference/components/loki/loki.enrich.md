---
aliases:
- /docs/alloy/latest/reference/components/loki/loki.enrich/
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.enrich/
title: loki.enrich
labels:
  stage: experimental
  products:
    - oss
description: The loki.enrich component enriches logs with labels from service discovery.
---

# `loki.enrich`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `loki.enrich` component enriches logs with additional labels from service discovery targets.
It matches a label from incoming logs against a label from discovered targets, and copies specified labels from the matched target to the log entry.

## Usage

```alloy
loki.enrich "<LABEL>" {
  // List of targets from a discovery component
  targets = <DISCOVERY_COMPONENT>.targets
  
  // Which label from discovered targets to match against
  target_match_label = "<LABEL>"
  
  // Which label from incoming logs to match against
  logs_match_label = "<LABEL>"
  
  // List of labels to copy from discovered targets to logs
  labels_to_copy = ["<LABEL>", ...]
  
  // Where to send enriched logs
  forward_to = [<RECEIVER_LIST>]
}
```

## Arguments

You can use the following arguments with `loki.enrich`:

| Name                 | Type                  | Description                                                                                      | Default                | Required |
| -------------------- | --------------------- | ------------------------------------------------------------------------------------------------ | ---------------------- | -------- |
| `forward_to`         | `[]loki.LogsReceiver` | List of receivers to send enriched logs to.                                                      |                        | yes      |
| `target_match_label` | `string`              | The label from discovered targets to match against, for example, `"__inventory_consul_service"`. |                        | yes      |
| `targets`            | `[]discovery.Target`  | List of targets from a discovery component.                                                      |                        | yes      |
| `labels_to_copy`     | `[]string`            | List of labels to copy from discovered targets to logs. If empty, all labels will be copied.     |                        | no       |
| `logs_match_label`   | `string`              | The label from incoming logs to match against discovered targets, for example `"service_name"`.  |                        | no       |

If not provided, the `logs_match_label` attribute will default to the value of `target_match_label`.

## Blocks

The `loki.enrich` component doesn't support any blocks. You can configure this component with arguments.

## Exports

The following values are exported:

| Name       | Type                | Description                                                 |
| ---------- | ------------------- | ----------------------------------------------------------- |
| `receiver` | `loki.LogsReceiver` | A receiver that can be used to send logs to this component. |

## Example

```alloy
// Configure HTTP discovery
discovery.http "default" {
    url = "http://network-inventory.example.com/prometheus_sd"
}

discovery.relabel "default" {
    targets = discovery.http.default.targets
    rule {
        action        = "replace"
        source_labels = ["__inventory_rack"]
        target_label  = "rack"
    }
    rule {
        action        = "replace"
        source_labels = ["__inventory_datacenter"]
        target_label  = "datacenter"
    }
    rule {
        action        = "replace"
        source_labels = ["__inventory_environment"]
        target_label  = "environment"
    }
    rule {
        action        = "replace"
        source_labels = ["__inventory_tenant"]
        target_label  = "tenant"
    }
    rule {
        action        = "replace"
        source_labels = ["__inventory_primary_ip"]
        target_label  = "primary_ip"
    }
}

// Receive syslog messages
loki.source.syslog "incoming" {
    listener {
        address = ":514"
        protocol = "tcp"
        labels = {
            job = "syslog"
        }
    }
    forward_to = [loki.enrich.default.receiver]
}

// Enrich logs using HTTP discovery
loki.enrich "default" {
    // Use targets from HTTP discovery (after relabeling)
    targets = discovery.relabel.default.output

    // Match hostname from logs to DNS name
    target_match_label = "primary_ip"

    forward_to = [loki.write.default.receiver]
}
```

## Component Behavior

The component matches logs to discovered targets and enriches them with additional labels:

1. For each log entry, it looks up the value of `logs_match_label` from the log's labels or `target_match_label` if `logs_match_label` is not specified.
1. It matches this value against the `target_match_label` in discovered targets.
1. If a match is found, it copies the requested `labels_to_copy` from the discovered target to the log entry. If `labels_to_copy` is empty, all labels are copied.
1. The log entry, enriched or unchanged, is forwarded to the configured receivers.

{{< admonition type="caution" >}}
By default, `loki.enrich` is ready as soon as it starts, even if no targets have been discovered.
If telemetry is sent to this component before the metadata is synced, then it will be passed though as-is, without enrichment.
This is most likely to impact `loki.enrich` on startup for a short time before the `discovery` components have sent a new list of targets.
{{< /admonition >}}

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.enrich` can accept arguments from the following components:

- Components that export [Targets](../../../compatibility/#targets-exporters)
- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`loki.enrich` has exports that can be consumed by the following components:

- Components that consume [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
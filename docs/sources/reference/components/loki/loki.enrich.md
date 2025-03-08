---
aliases:
- /docs/alloy/latest/reference/components/loki/loki.enrich/
canonical: /docs/alloy/latest/reference/components/loki/loki.enrich/
title: loki.enrich
labels:
  stage: experimental
description: The loki.enrich component enriches logs with labels from service discovery.
---

# loki.enrich

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `loki.enrich` component enriches logs with additional labels from service discovery targets. It matches a label from incoming logs against a label from discovered targets, and copies specified labels from the matched target to the log entry.

## Usage

```alloy
loki.enrich "LABEL" {
  // List of targets from a discovery component
  targets = DISCOVERY_COMPONENT.targets
  
  // Which label from discovered targets to match against
  match_label = "LABEL"
  
  // Which label from incoming logs to match against
  source_label = "LABEL"
  
  // List of labels to copy from discovered targets to logs
  target_labels = ["LABEL", ...]
  
  // Where to send enriched logs
  forward_to = [RECEIVER_LIST]
}
```

## Arguments

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`targets` | `[]discovery.Target` | List of targets from a discovery component. | | yes
`target_match_label` | `string` | Which label from discovered targets to match against (e.g., "__inventory_consul_service"). | | yes
`logs_match_label` | `string` | Which label from incoming logs to match against discovered targets (e.g., "service_name"). If not specified, `target_match_label` will be used. | `target_match_label` | no
`target_labels` | `[]string` | List of labels to copy from discovered targets to logs. If empty, all labels will be copied. | | no
`forward_to` | `[]loki.LogsReceiver` | List of receivers to send enriched logs to. | | yes

## Exports

The following values are exported:

Name | Type | Description
---- | ---- | -----------
`receiver` | `loki.LogsReceiver` | A receiver that can be used to send logs to this component.

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

1. For each log entry, it looks up the value of `logs_match_label` from the log's labels (or `target_match_label` if `logs_match_label` is not specified)
2. It matches this value against the `target_match_label` in discovered targets
3. If a match is found, it copies the requested `target_labels` from the discovered target to the log entry (if `target_labels` is empty, all labels are copied)
4. The log entry (enriched or unchanged) is forwarded to the configured receivers

## See also

* [loki.source.syslog](../loki.source.syslog/)
* [loki.source.api](../loki.source.api/)
* [discovery.relabel](../discovery/discovery.relabel/)
* [discovery.http](../discovery/discovery.http/) <!-- START GENERATED COMPATIBLE COMPONENTS -->

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
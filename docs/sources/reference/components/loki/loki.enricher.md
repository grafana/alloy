---
aliases:
- /docs/alloy/latest/reference/components/loki/loki.enricher/
canonical: /docs/alloy/latest/reference/components/loki/loki.enricher/
title: loki.enricher
description: The loki.enricher component enriches logs with labels from service discovery.
---

# loki.enricher

<span class="badge docs-labels__stage docs-labels__item">Beta</span>

The `loki.enricher` component enriches logs with additional labels from service discovery targets. It matches a label from incoming logs against a label from discovered targets, and copies specified labels from the matched target to the log entry.

## Usage

```alloy
loki.enricher "LABEL" {
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
`match_label` | `string` | Which label from discovered targets to match against (e.g., "__meta_consul_service"). | | yes
`source_label` | `string` | Which label from incoming logs to match against discovered targets (e.g., "service_name"). | | yes
`target_labels` | `[]string` | List of labels to copy from discovered targets to logs. | | yes
`forward_to` | `[]loki.LogsReceiver` | List of receivers to send enriched logs to. | | yes

## Exports

The following values are exported:

Name | Type | Description
---- | ---- | -----------
`receiver` | `loki.LogsReceiver` | A receiver that can be used to send logs to this component.

## Example

```river
// Configure DNS discovery
discovery.dns "services" {
    names = ["*.service.consul"]
    type = "A"
    port = 80
}

// Receive syslog messages
loki.source.syslog "incoming" {
    listener {
        address = ":1514"
        protocol = "tcp"
        labels = {
            job = "syslog"
        }
    }
    forward_to = [loki.enricher.default.receiver]
}

// Enrich logs using DNS discovery
loki.enricher "default" {
    // Use targets from DNS discovery
    targets = discovery.dns.services.targets

    // Match hostname from logs to DNS name
    match_label = "__meta_dns_name"
    source_label = "hostname"

    // Copy these labels from discovered targets to logs
    target_labels = [
        "__meta_dns_name",
        "__meta_dns_a_record"
    ]

    forward_to = [loki.write.default.receiver]
}
```

## Component Behavior

The component matches logs to discovered targets and enriches them with additional labels:

1. For each log entry, it looks up the value of `source_label` from the log's labels
2. It matches this value against the `match_label` in discovered targets
3. If a match is found, it copies the requested `target_labels` from the discovered target to the log entry
4. The log entry (enriched or unchanged) is forwarded to the configured receivers

## See also

* [loki.source.syslog](../loki.source.syslog/)
* [discovery.dns](../discovery/discovery.dns/)
* [discovery.http](../discovery/discovery.http/) 
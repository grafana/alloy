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
It matches labels from incoming logs against labels from discovered targets, and copies specified labels from the matched target to the log entry.
If no match occurs, the component forwards the log entry unchanged.

Use the `target_to_log_match` argument to specify which target labels correspond to which log labels.
The map keys are target label names and the values are the corresponding log label names.
All labels in the map must match for enrichment to occur.

{{< admonition type="warning" >}}
The `target_match_label` and `logs_match_label` arguments are deprecated in favor of `target_to_log_match`.
If `target_to_log_match` is set, it takes precedence.
Replace `target_match_label = "hostname"` with `target_to_log_match = {"hostname" = "hostname"}`.
These deprecated arguments will be removed in a future release.
{{< /admonition >}}

## Usage

```alloy
loki.enrich "<LABEL>" {
  targets = <DISCOVERY_COMPONENT>.targets

  target_to_log_match = {
    "<TARGET_LABEL_1>" = "<LOG_LABEL_1>",
    "<TARGET_LABEL_2>" = "<LOG_LABEL_2>",
  }

  labels_to_copy = ["<LABEL>", ...]

  forward_to = [<RECEIVER_LIST>]
}
```

## Arguments

You can use the following arguments with `loki.enrich`:

| Name                  | Type                 | Description                                                                                                   | Default | Required |
| --------------------- | -------------------- | ------------------------------------------------------------------------------------------------------------- | ------- | -------- |
| `forward_to`          | `list(LogsReceiver)` | List of receivers to send log entries to.                                                                     |         | yes      |
| `targets`             | `list(map(string))`  | List of targets from a discovery component.                                                                   |         | yes      |
| `labels_to_copy`      | `list(string)`       | List of labels to copy from discovered targets to logs. If empty, all labels are copied.                      |         | no       |
| `logs_match_label`    | `string`             | (Deprecated) The label from incoming logs to match against discovered targets, for example, `"service_name"`. |         | no       |
| `target_match_label`  | `string`             | (Deprecated) The label from discovered targets to match against, for example, `"hostname"`.                   |         | no       |
| `target_to_log_match` | `map(string)`        | Map of target label names to log label names. All entries must match for enrichment.                          |         | no       |

## Blocks

The `loki.enrich` component doesn't support any blocks. You can configure this component with arguments.

## Exports

The following values are exported:

| Name       | Type                | Description                                                 |
| ---------- | ------------------- | ----------------------------------------------------------- |
| `receiver` | `loki.LogsReceiver` | A receiver that can be used to send logs to this component. |

## Example

The following example enriches syslog entries with labels from an HTTP service discovery target.
It matches the connection IP address and tenant label before it forwards entries to Loki.

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

loki.relabel "syslog" {
    rule {
        source_labels = ["__syslog_connection_ip_address"]
        target_label  = "source_ip"
    }
}

// Receive syslog messages
loki.source.syslog "incoming" {
    listener {
        address  = ":514"
        protocol = "tcp"
        labels = {
            job    = "syslog"
            tenant = "production"
        }
    }

    relabel_rules = loki.relabel.syslog.rules
    forward_to = [loki.enrich.default.receiver]
}

// Enrich logs using HTTP discovery
loki.enrich "default" {
    // Use targets from HTTP discovery (after relabeling)
    targets = discovery.relabel.default.output

    target_to_log_match = {
        "primary_ip" = "source_ip"
        "tenant"     = "tenant"
    }

    forward_to = [loki.write.default.receiver]
}

loki.write "default" {
    endpoint {
        url = "http://loki:3100/loki/api/v1/push"
    }
}
```

## Component behavior

The component matches logs to discovered targets and enriches them with additional labels:

1. For each log entry, it looks up the log labels specified by `target_to_log_match`.
1. It matches those values against the corresponding target labels. All label pairs must match the same target.
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

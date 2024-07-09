---
canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.relabel/
aliases:
  - ../loki.relabel/ # /docs/alloy/latest/reference/components/loki.relabel/
description: Learn about loki.relabel
title: loki.relabel
---

# loki.relabel

The `loki.relabel` component rewrites the label set of each log entry passed to
its receiver by applying one or more relabeling `rule`s and forwards the
results to the list of receivers in the component's arguments.

If no labels remain after the relabeling rules are applied, then the log
entries are dropped.

The most common use of `loki.relabel` is to filter log entries or standardize
the label set that is passed to one or more downstream receivers. The `rule`
blocks are applied to the label set of each log entry in order of their
appearance in the configuration file. The configured rules can be retrieved by
calling the function in the `rules` export field.

If you're looking for a way to process the log entry contents, take a look at
[the `loki.process` component][loki.process] instead.

[loki.process]: ../loki.process/

Multiple `loki.relabel` components can be specified by giving them different labels.

## Usage

```alloy
loki.relabel "LABEL" {
  forward_to = RECEIVER_LIST

  rule {
    ...
  }

  ...
}
```

## Arguments

The following arguments are supported:

Name             | Type             | Description                                                    | Default | Required
-----------------|------------------|----------------------------------------------------------------|---------|---------
`forward_to`     | `list(receiver)` | Where to forward log entries after relabeling.                 |         | yes
`max_cache_size` | `int`            | The maximum number of elements to hold in the relabeling cache | 10,000  | no

## Blocks

The following blocks are supported inside the definition of `loki.relabel`:

Hierarchy | Name     | Description                                        | Required
----------|----------|----------------------------------------------------|---------
rule      | [rule][] | Relabeling rules to apply to received log entries. | no

[rule]: #rule-block

### rule block

{{< docs/shared lookup="reference/components/rule-block-logs.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name       | Type           | Description
-----------|----------------|-------------------------------------------------------------
`receiver` | `receiver`     | The input receiver where log lines are sent to be relabeled.
`rules`    | `RelabelRules` | The currently configured relabeling rules.

## Component health

`loki.relabel` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields are kept at their last healthy values.

## Debug information

`loki.relabel` does not expose any component-specific debug information.

## Debug metrics

* `loki_relabel_entries_processed` (counter): Total number of log entries processed.
* `loki_relabel_entries_written` (counter): Total number of log entries forwarded.
* `loki_relabel_cache_misses` (counter): Total number of cache misses.
* `loki_relabel_cache_hits` (counter): Total number of cache hits.
* `loki_relabel_cache_size` (gauge): Total size of relabel cache.

## Example

The following example creates a `loki.relabel` component that only forwards entries whose 'level' value is set to 'error'.

```alloy
loki.relabel "keep_error_only" {
  forward_to = [loki.write.onprem.receiver]

  rule {
    action        = "keep"
    source_labels = ["level"]
    regex         = "error"
  }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.relabel` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)

`loki.relabel` has exports that can be consumed by the following components:

- Components that consume [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->

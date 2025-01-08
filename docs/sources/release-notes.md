---
canonical: https://grafana.com/docs/alloy/latest/release-notes/
description: Release notes for Grafana Alloy
menuTitle: Release notes
title: Release notes for Grafana Alloy
weight: 999
---

# Release notes for {{% param "FULL_PRODUCT_NAME" %}}

The release notes provide information about deprecations and breaking changes in {{< param "FULL_PRODUCT_NAME" >}}.

For a complete list of changes to {{< param "FULL_PRODUCT_NAME" >}}, with links to pull requests and related issues when available, refer to the [Changelog][].

[Changelog]: https://github.com/grafana/alloy/blob/main/CHANGELOG.md

## v1.6

### Breaking change: The `topics` argument in the component `loki.source.kafka` does not use regex by default anymore

A bug in `loki.source.kafka` caused the component to treat all topics as regex. For example, setting the topic value to "foo" would match any topic containing the substring "foo".
With the fix introduced in this version, topic values are now treated as exact matches by default.
Regular expression matching is still supported by prefixing a topic with "^", allowing it to match multiple topics.

## v1.5

### Breaking change: Change default value of `max_streams` in `otelcol.processor.deltatocumulative`

The default value was changed from `0` to `9223372036854775807` (max int).

### Breaking change: Change default value of `namespace` in `otelcol.connector.spanmetrics`

The default value was changed from `""` to `"traces.span.metrics"`.

### Breaking change: The component `otelcol.exporter.logging` has been removed in favor of `otelcol.exporter.debug`

Both components are very similar. More information can be found in the [announcement issue](https://github.com/open-telemetry/opentelemetry-collector/issues/11337).

### Breaking change: Change default value of `revision` in `import.git`

The default value was changed from `"HEAD"` to `"main"`.
Setting the `revision` to `"HEAD"`, `"FETCH_HEAD"`, `"ORIG_HEAD"`, `"MERGE_HEAD"` or `"CHERRY_PICK_HEAD"` is no longer allowed.

## v1.4

### Breaking change: Some debug metrics for `otelcol` components have changed

For example, `otelcol.exporter.otlp`'s `exporter_sent_spans_ratio_total` metric is now `otelcol_exporter_sent_spans_total`.
You may need to change your dashboard and alert settings to reference the new metrics.
Refer to each component's documentation page for more information.

### Breaking change: The `convert_sum_to_gauge` and `convert_gauge_to_sum` functions in `otelcol.processor.transform` change context

The `convert_sum_to_gauge` and `convert_gauge_to_sum` functions must now be used in the `metric` context rather than in the `datapoint` context.
This is due to a [change upstream](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34567).

### Breaking change: Renamed metrics in `beyla.ebpf`

`process.cpu.state` is renamed to `cpu.mode` and `beyla_build_info` is renamed to `beyla_internal_build_info`.

## v1.3

### Breaking change: `remotecfg` block updated argument name from `metadata` to `attributes`

{{< admonition type="note" >}}
This feature is in [Public preview][] and is not covered by {{< param "FULL_PRODUCT_NAME" >}} [backward compatibility][] guarantees.

[Public preview]: https://grafana.com/docs/release-life-cycle/
[backward compatibility]: ../introduction/backward-compatibility/
{{< /admonition >}}

The `remotecfg` block has an updated argument name from `metadata` to `attributes`.

## v1.2

### Breaking change: `remotecfg` block updated for Agent rename

{{< admonition type="note" >}}
This feature is in [Public preview][] and is not covered by {{< param "FULL_PRODUCT_NAME" >}} [backward compatibility][] guarantees.

[Public preview]: https://grafana.com/docs/release-life-cycle/
[backward compatibility]: ../introduction/backward-compatibility/
{{< /admonition >}}

The `remotecfg` block has been updated to use [alloy-remote-config](https://github.com/grafana/alloy-remote-config)
over [agent-remote-config](https://github.com/grafana/agent-remote-config). This change
aligns `remotecfg` API terminology with Alloy and includes updated endpoints.
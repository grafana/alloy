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

### Breaking change: Change decision precedence in `otelcol.processor.tail_sampling` when using `and_sub_policy` and `invert_match` 

Alloy v1.5 upgraded to [OpenTelemetry Collector v0.104.0][otel-v0_104], which included a [fix][#33671] to the tail sampling processor:

> Previously if the decision from a policy evaluation was `NotSampled` or `InvertNotSampled` 
  it would return a `NotSampled` decision regardless, effectively downgrading the result.
  This was breaking the documented behaviour that inverted decisions should take precedence over all others.

The "documented behavior" which the above quote is referring to is in the [processor documentation][tail-sample-docs]:

> Each policy will result in a decision, and the processor will evaluate them to make a final decision:
> 
> * When there's an "inverted not sample" decision, the trace is not sampled;
> * When there's a "sample" decision, the trace is sampled;
> * When there's a "inverted sample" decision and no "not sample" decisions, the trace is sampled;
> * In all other cases, the trace is NOT sampled
> 
> An "inverted" decision is the one made based on the "invert_match" attribute, such as the one from the string, numeric or boolean tag policy.
    
However, in [OpenTelemetry Collector v0.1116.0][otel-v0_116] this fix was [reverted][#36673]:

> Reverts [#33671][], allowing for composite policies to specify inverted clauses in conjunction with other policies. 
  This is a change bringing the previous state into place, breaking users who rely on what was introduced as part of [#33671][].

[otel-v0_104]: https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.104.0
[otel-v0_116]: https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.116.0
[#33671]: https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33671
[#33671]: https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33671
[#36673]: https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36673
[tail-sample-docs]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.116.0/processor/tailsamplingprocessor/README.md

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
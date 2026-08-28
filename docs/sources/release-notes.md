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

## v1.12

### Breaking changes due to bugfixes in Prometheus exporters

The `prometheus.exporter.blackbox`, `prometheus.exporter.snmp` and `prometheus.exporter.statsd` components now use the component ID instead of the hostname as
their `instance` label in their exported metrics. This is a consequence of a bug fix that could lead to [missing data when using the exporter
with clustering](https://github.com/grafana/alloy/issues/1009).

If you would like to retain the previous behaviour, you can use `discovery.relabel` with `action = "replace"` rule to
set the `instance` label to `sys.env("HOSTNAME")`.

## v1.11

### Breaking changes due to major version upgrade of Prometheus

Prometheus dependency had a major version upgrade from v2.55.1 to v3.4.2.

- The `.` pattern in regular expressions in PromQL matches newline characters now. With this change a regular expressions like `.*` matches strings that include `\n`. This applies to matchers in queries and relabel configs in Prometheus and Loki components.

- The `enable_http2` in `prometheus.remote_write` component's endpoints has been changed to `false` by default. Previously, in Prometheus v2 the remote write http client would default to use http2. In order to parallelize multiple remote write queues across multiple sockets its preferable to not default to http2. If you prefer to use http2 for remote write you must now set `enable_http2` to `true` in your `prometheus.remote_write` endpoints configuration section.

- The experimental CLI flag `--feature.prometheus.metric-validation-scheme` has been deprecated and has no effect. You can configure the metric validation scheme individually for each `prometheus.scrape` component.

- Log message format has changed for some of the `prometheus.*` components as part of the upgrade to Prometheus v3.

- The values of the `le` label of classic histograms and the `quantile` label of summaries are now normalized upon ingestion. In previous Alloy versions, that used Prometheus v2, the value of these labels depended on the scrape protocol (protobuf vs text format) in some situations. This led to label values changing based on the scrape protocol. E.g. a metric exposed as `my_classic_hist{le="1"}` would be ingested as `my_classic_hist{le="1"}` via the text format, but as `my_classic_hist{le="1.0"}` via protobuf. This changed the identity of the metric and caused problems when querying the metric. In current Alloy release, which uses Prometheus v3, these label values will always be normalized to a float like representation. I.e. the above example will always result in `my_classic_hist{le="1.0"}` being ingested into Prometheus, no matter via which protocol. The effect of this change is that alerts, recording rules and dashboards that directly reference label values as whole numbers such as `le="1"` will stop working.

  The recommended way to deal with this change is to fix references to integer `le` and `quantile` label values, but otherwise do nothing and accept that some queries that span the transition time will produce inaccurate or unexpected results.

See the upstream [Prometheus v3 migration guide](https://prometheus.io/docs/prometheus/3.4/migration/) for more details.

### Breaking changes in `prometheus.scrape`

`scrape_native_histograms` attribute for `prometheus.scrape` is now set to `false`, whereas in previous versions of Alloy it would default to `true`. 
This means that it is no longer enough to just configure `scrape_protocols` to start with `PrometheusProto` to scrape native histograms - `scrape_native_histograms` has to be enabled. 
If `scrape_native_histograms` is enabled, `scrape_protocols` will automatically be configured correctly for you to include `PrometheusProto`.
If you configure it explicitly, Alloy will validate that `PrometheusProto` is in the `scrape_protocols` list.

In previous versions of Alloy configuring `scrape_protocols` to start with `PrometheusProto` was enough to start scraping native histograms because `scrape_native_histogram` defaulted to true:
```alloy
prometheus.scrape "scrape" {
  scrape_protocols = ["PrometheusProto"]
}
```

Now it has to be enabled and `scrape_protocols` can be omitted:
```alloy
prometheus.scrape "scrape" {
  scrape_native_histograms = true
}
```

### Breaking changes in `prometheus.exporter.windows`

As the `windows_exporter` continues to be refactored upstream, there are various breaking changes in metrics.
- `windows_process_start_time` -> `windows_process_start_time_seconds_timestamp` in the `process` collector.
- `windows_time_clock_frequency_adjustment_ppb_total` -> `windows_time_clock_frequency_adjustment_ppb` in the `time` collector.
- `windows_net_nic_info` -> `windows_net_nic_address_info` in `net` collector.
- `windows_system_boot_time_timestamp_seconds` -> `windows_system_boot_time_timestamp` in `system` collector.
- `windows_os_physical_memory_free_bytes` -> `windows_memory_physical_free_bytes` from `os` collector -> `memory` collector.
- `windows_os_process_memory_limit_bytes` -> `windows_memory_process_memory_limit_bytes` from `os` collector -> `memory` collector.
- `windows_os_processes` -> `windows_system_processes` from `os` collector -> `system` collector.
- `windows_os_processes_limit` -> `windows_system_process_limit` from `os` collector -> `system` collector.
- `windows_os_time` -> `windows_time_current_timestamp_seconds` from `os` collector -> `time` collector.
- `windows_os_timezone` -> `windows_time_timezone` from `os` collector -> `time` collector.
- `windows_os_users` from `os` collector can be reconstructed by aggregating `windows_terminal_services_session_info{state="active"}` in `terminal_services` collector.
- `windows_os_virtual_memory_bytes` -> `windows_memory_commit_limit` from `os` collector -> `memory` collector.
- `windows_os_virtual_memory_free_bytes` from `os` collector can be reconstructed by subtracting `windows_memory_committed_bytes` from `windows_memory_commit_limit` in `memory` collector.
- `windows_os_visible_memory_bytes` -> `windows_memory_physical_total_bytes` from `os` collector -> `memory` collector.

Deprecated collectors have been removed. Configuration of these collectors will be allowed in Alloy for at least one minor version, but will have no effect.
- `logon` collector removed, use the `terminal_services` collector instead.
- `cs` collector removed, use the `os`, `memory`, or `cpu` collectors instead.

Refer to the [release notes][windows_exporter_31] for the windows_exporter version v0.31.0 for the breaking change details.

[windows_exporter_31]: https://github.com/prometheus-community/windows_exporter/releases/tag/v0.31.0

## v1.9

### Breaking change: The `prometheus.exporter.oracledb` component now embeds a different exporter

The `prometheus.exporter.oracledb` component now embeds the [`oracledb_exporter from oracle`](https://github.com/oracle/oracle-db-appdev-monitoring) instead of the deprecated [`oracledb_exporter from iamseth`](https://github.com/iamseth/oracledb_exporter).

As a result of this change, the following metrics are no longer available by default:

- `oracledb_sessions_activity`
- `oracledb_tablespace_free_bytes`

The previously undocumented argument `custom_metrics` is now expecting a list of paths to custom metrics files.

### Breaking change: The `enable_context_propagation` argument in `beyla.ebpf` has been replaced with the `context_propagation` argument.

Set `enable_context_propagation` to `all` to get the same behaviour as `enable_context_propagation` being set to `true`.

### Breaking change: In `prometheus.exporter.windows`, the `service` and `msmq` collectors no longer work with WMI

The `msmq` block has been removed. The `enable_v2_collector`, `where_clause`, and `use_api` attributes in the `service` block are also removed.

Prior to Alloy v1.9.0, the `service` collector exists in 2 different versions. 
Version 1 used WMI (Windows Management Instrumentation) to query all services and was able to provide additional information. 
Version 2 is a more efficient solution by directly connecting to the service manager, 
but is not able to provide additional information like run_as or start configuration.

In Alloy v1.9.0 the Version 1 collector was removed, hence why some arguments and blocks were removed.
In Alloy v1.9.2 those arguments and blocks were re-introduced as a no-op in order to make migrations easier for customers.

Due to this change, the metrics produced by `service` collector are different in v1.9.0 and above.
The `msmq` collector metrics are unchanged.

Example V2 `service` metrics:

```
windows_service_state{display_name="Declared Configuration(DC) service",name="dcsvc",status="continue pending"} 0
windows_service_state{display_name="Declared Configuration(DC) service",name="dcsvc",status="pause pending"} 0
windows_service_state{display_name="Declared Configuration(DC) service",name="dcsvc",status="paused"} 0
windows_service_state{display_name="Declared Configuration(DC) service",name="dcsvc",status="running"} 0
windows_service_state{display_name="Declared Configuration(DC) service",name="dcsvc",status="start pending"} 0
windows_service_state{display_name="Declared Configuration(DC) service",name="dcsvc",status="stop pending"} 0
windows_service_state{display_name="Declared Configuration(DC) service",name="dcsvc",status="stopped"} 1
```

For more information on V1 and V2 `service` metrics, see the upstream exporter documentation for [version 0.27.3 of the Windows Exporter][win-exp-svc-0-27-3],
which is the version used in Alloy v1.8.3. 
Alloy v1.9.2 uses [version 0.30.7 of the Windows Exporter][win-exp-svc-0-30-7].

[win-exp-svc-0-27-3]: https://github.com/prometheus-community/windows_exporter/blob/v0.27.3/docs/collector.service.md
[win-exp-svc-0-30-7]: https://github.com/prometheus-community/windows_exporter/blob/v0.30.7/docs/collector.service.md

## v1.6

### Breaking change: The `topics` argument in the component `loki.source.kafka` does not use regex by default anymore

A bug in `loki.source.kafka` caused the component to treat all topics as regular expressions. For example, setting the topic value to "telemetry" would match any topic containing the substring "telemetry".
With the fix introduced in this version, topic values are now treated as exact matches by default.
Regular expression matching is still supported by prefixing a topic with "^", allowing it to match multiple topics.

### Breaking change: Change decision precedence in `otelcol.processor.tail_sampling` when using `and_sub_policy` and `invert_match` 

Alloy v1.5 upgraded to [OpenTelemetry Collector v0.104.0][otel-v0_104], which included a [fix][#33671] to the tail sampling processor:

> Previously if the decision from a policy evaluation was `NotSampled` or `InvertNotSampled` 
> it would return a `NotSampled` decision regardless, effectively downgrading the result.
> This was breaking the documented behaviour that inverted decisions should take precedence over all others.

The "documented behavior" which the above quote is referring to is in the [processor documentation][tail-sample-docs]:

> Each policy will result in a decision, and the processor will evaluate them to make a final decision:
> 
> * When there's an "inverted not sample" decision, the trace is not sampled;
> * When there's a "sample" decision, the trace is sampled;
> * When there's a "inverted sample" decision and no "not sample" decisions, the trace is sampled;
> * In all other cases, the trace is NOT sampled
> 
> An "inverted" decision is the one made based on the "invert_match" attribute, such as the one from the string, numeric or boolean tag policy.
    
However, in [OpenTelemetry Collector v0.116.0][otel-v0_116] this fix was [reverted][#36673]:

> Reverts [#33671][], allowing for composite policies to specify inverted clauses in conjunction with other policies. 
> This is a change bringing the previous state into place, breaking users who rely on what was introduced as part of [#33671][].

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

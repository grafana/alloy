---
canonical: https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.exporter.datadog/
aliases:
  - ../otelcol.exporter.datadog/ # /docs/alloy/latest/reference/components/otelcol.exporter.datadog/
description: Learn about otelcol.exporter.datadog
labels:
  stage: experimental
title: otelcol.exporter.datadog
---

# otelcol.exporter.datadog

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`otelcol.exporter.datadog` accepts metrics and traces telemetry data from 
other `otelcol` components and sends it to Datadog.

{{< admonition type="note" >}}
`otelcol.exporter.datadog` is a wrapper over the upstream OpenTelemetry Collector `datadog` exporter from the `otelcol-contrib`  distribution.
Bug reports or feature requests will be redirected to the upstream repository, if necessary.
{{< /admonition >}}

Multiple `otelcol.exporter.datadog` components can be specified by giving them
different labels.

## Usage

```alloy
otelcol.exporter.datadog "LABEL" {
    api {
        api_key = "YOUR_API_KEY_HERE"
    }
}
```

## Arguments

The following arguments are supported:

Name                                       | Type            | Description                                                                         | Default              | Required
------------------------------------------ | --------------- | ----------------------------------------------------------------------------------- | -------------------- | --------
`hostname`                              | `string`        | The fallback hostname used for payloads without hostname-identifying attributes.                                                    |                      | no
`only_metadata`                                  | `bool`  | Whether to send only metadata.                                                        | `false` | no

If `hostname` is unset, the hostname will be determined automatically (see [here for details](https://docs.datadoghq.com/opentelemetry/schema_semantics/hostname/?tab=datadogexporter#fallback-hostname-logic)). 
This option will **NOT** change the hostname applied to metrics or traces if they already have hostname-identifying attributes.

## Blocks

The following blocks are supported inside the definition of `otelcol.exporter.datadog`:

Hierarchy                        | Block               | Description                                                                 | Required
-------------------------------- | ------------------- | --------------------------------------------------------------------------- | --------
api | [api][] | Configures authentication with Datadog | yes
traces | [traces][] | Trace exporter specific configuration. | no
metrics | [metrics][] |  Metric exporter specific configuration. | no
metrics > histograms | [histograms][] | Histograms specific configuration. | no
metrics > sums | [sums][] | Sums specific configuration | no
metrics > summaries | [summaries][] | Summaries specific configuration | no
host_metadata | [host_metadata][] | Host metadata specific configuration. | no
client           | [client][]           | Configures the HTTP server to send telemetry data to.                      | no
client > tls     | [tls][]              | Configures TLS for the HTTP client.                                        | no
retry_on_failure                 | [retry_on_failure][] | Configures retry mechanism for failed requests.                             | no
queue                            | [queue][]            | Configures batching of data before sending.                                 | no
debug_metrics                    | [debug_metrics][]    | Configures the metrics which this component generates to monitor its state. | no

The `>` symbol indicates deeper levels of nesting. For example, `client > tls`
refers to a `tls` block defined inside a `client` block.

[api]: #api-block
[traces]: #traces-block
[metrics]: #metrics-block
[histograms]: #histograms-block
[sums]: #sums-block
[summaries]: #summaries-block
[host_metadata]: #host_metadata-block
[client]: #client-block
[tls]: #tls-block
[retry_on_failure]: #retry_on_failure-block
[queue]: #queue-block
[debug_metrics]: #debug_metrics-block

### api block

The `api` block configures authentication with the Datadog API. This is required to send telemetry
to Datadog. If the `api` block isn't provided, telemetry cannot be sent to Datadog.

The following arguments are supported:

Name                                       | Type            | Description                                                                         | Default              | Required
------------------------------------------ | --------------- | ----------------------------------------------------------------------------------- | -------------------- | --------
`api_key` | `secret` | API Key for Datadog | | yes
`site` | `string` | The site of the Datadog intake to send Agent data to. | `"datadoghq.com"` | no
`fail_on_invalid_key` | `bool` | Whether to exit at startup on an invalid API key | `false` | no

### traces block

The `traces` block configures Trace specific exporter settings.

The following arguments are supported:

Name                                       | Type            | Description                                                                         | Default              | Required
------------------------------------------ | --------------- | ----------------------------------------------------------------------------------- | -------------------- | --------
`endpoint` | `string` | The host of the Datadog intake server to send traces to. | `"https://trace.agent.datadoghq.com"` | no
`ignore_resources` | `list(string)` |  A blocklist of regular expressions can be provided to disable certain traces based on their resource name. || no
`span_name_remappings` | `map(string)` | A map of Datadog span operation name keys and preferred name values to update those names to. || no
`span_name_as_resource_name` | `bool` | Use OpenTelemetry semantic convention for span naming |`true`| no
`compute_stats_by_span_kind` | `bool` | Enables APM stats computation based on `span.kind` | `true` | no
`compute_top_level_by_span_kind` | `bool` | Enables top-level span identification based on `span.kind` | `false` | no
`peer_tags_aggregation` | `bool` | Enables aggregation of peer related tags in Datadog exporter | `false` | no
`peer_tags` | `list(string)` | List of supplementary peer tags that go beyond the defaults. || no
`trace_buffer` | `number` | Specifies the number of outgoing trace payloads to buffer before dropping | `10` | no

If `compute_stats_by_span_kind` is disabled, only top-level and measured spans will have stats computed. 
If you are sending OTel traces and want stats on non-top-level spans, this flag will need to be set to `true`.
If you are sending OTel traces and do not want stats computed by span kind, you need to disable this flag and disable `compute_top_level_by_span_kind`.

If `endpoint` is unset, the value is obtained through the `site` parameter in the [api][] section.

[api]: #api-block

### metrics block

The `metrics` block configures Metric specific exporter settings.

The following arguments are supported:

Name                                       | Type            | Description                                                                         | Default              | Required
------------------------------------------ | --------------- | ----------------------------------------------------------------------------------- | -------------------- | --------
`delta_ttl` | `number` | The amount of seconds values are kept in memory for calculating deltas. | `3600` | no
`endpoint` | `string` | The host of the Datadog intake server to send metrics to. | `"https://api.datadoghq.com"` | no
`resource_attributes_as_tags` | `bool` | Set to true to add resource attributes of a metric to its metric tags. | `false` | no
`instrumentation_scope_metadata_as_tags` | `bool` | Set to true to add metadata about the instrumentation scope that created a metric. | `false` | no

Please note that any of the subset of resource attributes in this [list](https://docs.datadoghq.com/opentelemetry/guide/semantic_mapping/) 
are converted to datadog conventions and set to to metric tags whether `resource_attributes_as_tags` is enabled or not.

If `endpoint` is unset, the value is obtained through the `site` parameter in the [api][] section.

[api]: #api-block

### histograms block

The `histograms` block configures Histogram specific settings.

The following arguments are supported:

Name                                       | Type            | Description                                                                         | Default              | Required
------------------------------------------ | --------------- | ----------------------------------------------------------------------------------- | -------------------- | --------
`mode` | `string` | How to report histograms. | `"distributions"` | no
`send_aggregation_metrics` | `bool` | Whether to report sum, count, min and max as separate histogram metrics. | `false` | no

Valid values for `mode` are:
- `"distributions"` to report metrics as Datadog distributions (recommended).
- `"nobuckets"` to not report bucket metrics,
- `"counters"` to report one metric per histogram bucket.

### sums block

The `sums` block configures Sums specific settings.

The following arguments are supported:

Name                                       | Type            | Description                                                                         | Default              | Required
------------------------------------------ | --------------- | ----------------------------------------------------------------------------------- | -------------------- | --------
`cumulative_monotonic_mode` | `string` | How to report cumulative monotonic sums. | `"to_delta"` | no
`initial_cumulative_monotonic_value` | `string` | How to report the initial value for cumulative monotonic sums. | `"auto"` | no

Valid values for `cumulative_monotonic_mode` are:
- `"to_delta"` to calculate delta for sum in the client side and report as Datadog counts.
- `"raw_value"` to report the raw value as a Datadog gauge.

Valid values for `initial_cumulative_monotonic_value` are:
- `"auto"` reports the initial value if its start timestamp is set and it happens after the process was started.
- `"drop"` always drops the initial value.
- `"keep"` always reports the initial value.

### summaries block

The `summaries` block configures Summary specific settings.

The following arguments are supported:

Name                                       | Type            | Description                                                                         | Default              | Required
------------------------------------------ | --------------- | ----------------------------------------------------------------------------------- | -------------------- | --------
`mode` | `string` | How to report summaries.| `"gauges"` | no

Valid values for `mode` are:
- `"noquantiles"` to not report quantile metrics
- `"gauges"` to report one gauge metric per quantile.

### host_metadata block

The `host_metadata` block configures Host metadata specific configuration. Host metadata is the information used for populating the infrastructure list, the host map and providing host tags functionality within the Datadog app.

The following arguments are supported:

Name                                       | Type            | Description                                                                         | Default              | Required
------------------------------------------ | --------------- | ----------------------------------------------------------------------------------- | -------------------- | --------
`enabled` | `bool` | Enable the host metadata functionality | `true` | no
`hostname_source` | `string` | Source for the hostname of host metadata. | `"config_or_system"` | no
`tags` | `list(string)` | List of host tags to be sent as part of the host metadata. || no

By default, the exporter will only send host metadata for a single host, whose name is chosen according to `host_metadata::hostname_source`.

Valid values for `hostname_source` are:
- `"first_resource"` picks the host metadata hostname from the resource attributes on the first OTLP payload that gets to the exporter. 
  If the first payload lacks hostname-like attributes, it will fallback to 'config_or_system' behavior. **Do not use this hostname source if receiving data from multiple hosts**.
- `"config_or_system"` picks the host metadata hostname from the 'hostname' setting, falling back to system and cloud provider APIs.

### client block

The `client` block configures the HTTP client used by the component.

The following arguments are supported:

Name                      | Type                       | Description                                                                                                        | Default    | Required
--------------------------|----------------------------|--------------------------------------------------------------------------------------------------------------------|------------|---------
`read_buffer_size`        | `string`                   | Size of the read buffer the HTTP client uses for reading server responses.                                         | `0`        | no
`write_buffer_size`       | `string`                   | Size of the write buffer the HTTP client uses for writing requests.                                                | `"512KiB"` | no
`timeout`                 | `duration`                 | Time to wait before marking a request as failed.                                                                   | `"30s"`    | no
`max_idle_conns`          | `int`                      | Limits the number of idle HTTP connections the client can keep open.                                               | `100`      | no
`max_idle_conns_per_host` | `int`                      | Limits the number of idle HTTP connections the host can keep open.                                                 | `0`        | no
`max_conns_per_host`      | `int`                      | Limits the total (dialing,active, and idle) number of connections per host.                                        | `0`        | no
`idle_conn_timeout`       | `duration`                 | Time to wait before an idle connection closes itself.                                                              | `"90s"`    | no
`disable_keep_alives`     | `bool`                     | Disable HTTP keep-alive.                                                                                           | `false`    | no

### tls block

The `tls` block configures TLS settings used for the connection to the HTTP server.

The following arguments are supported:

Name                           | Type           | Description                                                                                  | Default     | Required
-------------------------------|----------------|----------------------------------------------------------------------------------------------|-------------|---------
`insecure_skip_verify`         | `boolean`      | Ignores insecure server TLS certificates.                                                    |             | no

### retry_on_failure block

The `retry_on_failure` block configures how failed requests to Datadog are retried.

{{< docs/shared lookup="reference/components/otelcol-retry-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### queue block

The `queue` block configures an in-memory buffer of batches before data is sent to the HTTP server.

{{< docs/shared lookup="reference/components/otelcol-queue-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### debug_metrics block

{{< docs/shared lookup="reference/components/otelcol-debug-metrics-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Exported fields

The following fields are exported and can be referenced by other components:

Name    | Type               | Description
--------|--------------------|-----------------------------------------------------------------
`input` | `otelcol.Consumer` | A value that other components can use to send telemetry data to.

`input` accepts `otelcol.Consumer` data for any telemetry signal (metrics, logs, or traces).

## Component health

`otelcol.exporter.datadog` is only reported as unhealthy if given an invalid
configuration.

## Debug information

`otelcol.exporter.datadog` does not expose any component-specific debug
information.

## Example

This example forwards prometheus metrics from Grafana Alloy itself through a receiver for conversion to Open Telemetry format before finally sending them to Datadog.
Note that only the `api` field is required for the Exporter to function (if using the US Datadog APIs):

```alloy
prometheus.exporter.self "default" {
}

prometheus.scrape "metamonitoring" {
  targets    = prometheus.exporter.self.default.targets
  forward_to = [otelcol.receiver.prometheus.default.receiver]
}

otelcol.receiver.prometheus "default" {
  output {
    metrics = [otelcol.exporter.datadog.default.input]
  }
}


otelcol.exporter.datadog "default" {
    api {
        api_key = "API_KEY"
    }

     metrics {
        endpoint = "https://api.ap1.datadoghq.com"
        resource_attributes_as_tags = true
    }
}
```

<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`otelcol.exporter.datadog` has exports that can be consumed by the following components:

- Components that consume [OpenTelemetry `otelcol.Consumer`](../../../compatibility/#opentelemetry-otelcolconsumer-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
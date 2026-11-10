---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.preview/
description: Measure a candidate metrics pipeline and send summary metrics without forwarding the original samples
labels:
  stage: general-availability
  products:
    - oss
title: prometheus.preview
---

# `prometheus.preview`

`prometheus.preview` receives Prometheus metrics, measures their sample volume and series cardinality, and discards the original samples.
It sends generated summary metrics to the receivers in `report_to`.
You can send these summaries to Grafana Cloud with `prometheus.remote_write` and query them in Grafana.
`prometheus.preview` doesn't provide a dedicated report UI or calculate prices.

Use `prometheus.preview` to measure a candidate pipeline before you enable ingestion of its original metrics.
The summaries are ordinary metrics: their transmission and ingestion can incur costs.
Collection still runs and puts load on scrape targets and Grafana Alloy.

You can specify multiple `prometheus.preview` components by giving them different labels.

## Usage

```alloy
prometheus.preview "<LABEL>" {
	report_to = <RECEIVER_LIST>
}
```

## Arguments

You can use the following arguments with `prometheus.preview`:

| Name | Type | Description | Default | Required |
| ---- | ---- | ----------- | ------- | -------- |
| `report_to` | `list(MetricsReceiver)` | Receivers for generated summary metrics, not original samples. | | yes |
| `group_by` | `list(string)` | Input label names used for report breakdowns. | `[]` | no |
| `max_groups` | `number` | Maximum number of named groups retained during an observation. | `100` | no |
| `max_series` | `number` | Maximum number of series fingerprints tracked per window and per input transaction. | `100000` | no |
| `report_interval` | `duration` | Interval between reports. | `"1m"` | no |
| `window` | `duration` | Minimum duration of each non-overlapping cardinality measurement window. | `"30m"` | no |

The `report_to` argument must contain at least one receiver.
Keep the report path separate from the input path: don't route reports back through any `prometheus.preview` component.

The `report_interval` argument must be positive.
The `window` argument must be at least `report_interval`.
The `max_groups` and `max_series` arguments must be positive integers.

### Measurement windows

An observation starts when the component is created.
The first report arrives after `report_interval`.
Until then, the component collects data but sends no report.

Each report includes cumulative sample counters and the distinct series observed so far in the current window.
A window closes on the first report at or after its configured duration.
That report includes the final counts and sets `prometheus_preview_window_complete` to `1`.
The next window starts immediately after that snapshot.
For example, a `"30m"` window with a `"1m"` report interval normally produces 29 partial reports followed by one complete report.
Slow report destinations can extend the window; use the reported duration rather than assuming the configured duration.

Window boundaries use local observation time, not source sample timestamps.
An input transaction contributes its observations only when it commits, to the window open at commit time.
Rolled-back transactions don't contribute.
Sample counters continue across window boundaries.

The `window` argument isn't a preview expiry time.
`prometheus.preview` continues to measure and discard original samples until you change or remove the configuration.
It never enables ingestion automatically.

### Cardinality and resource limits

`prometheus.preview` tracks 64-bit fingerprints of complete input label sets, including metric names.
Counts are distinct fingerprints, not backend active-series counts.
Hash collisions can undercount series.

When a window or input transaction exceeds `max_series`, sample counters continue, but the component stops tracking additional fingerprints for that window or transaction.
Affected cardinality reports set `prometheus_preview_series_limited` to `1` and represent lower bounds.
Increase `max_series` and start a new observation if you need to measure a larger candidate pipeline without this limit.

The component stores bounded aggregates rather than original sample values or histogram buckets.
It enforces limits independently for the component and for each in-flight input transaction.
Memory use therefore also depends on the number of concurrent transactions.
Raising either limit increases resource use.

### Group breakdowns

The `group_by` argument creates an additional report for each admitted combination of label values.
For example, `["job", "namespace"]` groups by the combination of both labels, not by each label independently.
Missing labels and empty label values belong to the same group.

You can specify at most 16 unique label names.
Each name must contain at most 128 ASCII letters, digits, or underscores, and can't start with a digit.
Input label names become report labels prefixed with `preview_group_`.
For example, `namespace` becomes `preview_group_namespace`, and `__name__` becomes `preview_group___name__`.

The component retains at most `max_groups` named groups for the entire observation, including across window boundaries.
These aren't top-K groups and don't have a guaranteed admission order.
Transactions also enforce `max_groups` independently.
Combinations that don't fit either group limit, or whose combined label values exceed 4096 bytes, contribute to the `overflow` scope instead.
If overflow changes the attribution of an already tracked series, the affected group's cardinality is marked as a lower bound.

Group labels are opt-in data disclosure: the selected values leave Alloy in reports.
Don't group by secrets, personal information, or labels with unbounded values such as request IDs.
With no `group_by`, reports contain no input labels or metric names.

### Resets and delivery

Restarting Alloy, recreating the component, or changing `window`, `group_by`, `max_groups`, or `max_series` starts a new observation.
This resets counters, group admission, and cardinality tracking.
An in-flight transaction from the previous configuration fails at commit rather than mixing observations.
Changing only `report_to` or `report_interval` preserves the observation.

The component stores observations in memory, not in a Write-Ahead Log (WAL).
It doesn't retry failed reports or guarantee a final report when you remove it or stop Alloy.
Later reports still contain cumulative sample counters, but a failed final window report can lose the completed window's cardinality result.
`prometheus.remote_write` provides its normal buffering and delivery behavior after it accepts a report.
A report failure never causes the component to forward original samples.

### Interpretation limits

Reports describe what reaches this component:

- Counts include float samples and native histogram samples separately. Classic histogram buckets are float samples.
- Counts exclude stale markers, exemplars, metadata, and synthetic start-time zero samples.
- Counts don't simulate backend rejection of duplicate, out-of-order, or otherwise invalid samples.
- Counts don't include transformations performed later by `prometheus.remote_write`, including external labels and write relabeling. Place candidate transformations upstream when you need to measure their effect.
- Series counts don't measure the overlap with metrics already in your Grafana Cloud stack. Don't interpret them as additional billable series.
- Reports don't estimate compressed network bytes, stored bytes, histogram bucket volume, or monetary cost.
- A short observation doesn't capture every deployment, scheduled job, or traffic peak.

## Blocks

The `prometheus.preview` component doesn't support any blocks.
You can configure this component with arguments.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name | Type | Description |
| ---- | ---- | ----------- |
| `receiver` | `MetricsReceiver` | Receives original samples for measurement and disposal. |

## Component health

`prometheus.preview` is only reported as unhealthy if given an invalid configuration.
Report delivery failures are logged and increment `prometheus_preview_report_failures_total`.
They don't change component health or fail input transactions.
Check downstream receiver health and report freshness as well as component health.

## Debug information

`prometheus.preview` exposes its latest generated report for local inspection through the component's debug information.
`report_to` remains required and is the primary report delivery path, including for Grafana Cloud.
Debug information is an additional view of the same report, not a replacement for delivery.

Open or refresh the `prometheus.preview` component's detail page in the Alloy UI to inspect these fields:

| Field | Description |
| ----- | ----------- |
| `report_available` | `false` until the first report is generated, or after a measurement reset; otherwise `true`. |
| `generated_at` | UTC timestamp of report generation in RFC 3339 format. Present when a report is available. |
| `metric` | Repeated blocks containing each report metric's `name`, `type`, `labels`, and `value`. |

The metric names, types, labels, and values match the [report metrics][report-metrics] sent to `report_to`.
The view updates when the component generates a report, normally every `report_interval`, rather than on every input sample or page request.
Use `generated_at` to check freshness.
Reading debug information doesn't generate or send a report, reset counters, or advance a measurement window.

The component retains only the latest report, not a history.
A completed-window report remains visible until the next report replaces it.
A measurement reset clears the view until a new report is generated; changing only `report_to` or `report_interval` preserves it.
A report remains available locally even if delivery fails, so its presence doesn't confirm that Grafana Cloud received it.
Check the failure counter, logs, and downstream receiver health separately.

The view contains generated summaries, not original sample values or unselected input labels.
Configured grouping values are visible locally just as they are in reports.
This view doesn't add report metrics to Alloy's `/metrics` endpoint or expose a new scrape target.

### Access debug information through the HTTP API

For a top-level component named `prometheus.preview.candidate`, request the same component-detail endpoint used by the UI:

```sh
curl --fail --silent --show-error \
  http://localhost:12345/api/v0/web/components/prometheus.preview.candidate \
  | jq '.debugInfo'
```

Adjust the server address, any configured HTTP prefix, and component ID for your deployment.
The `debugInfo` field contains Alloy's structured JSON representation of attributes and blocks, not a flat metric-name-to-value map.
This endpoint is an internal UI API; don't assume its response schema is a stable external integration contract.

### Access debug information through GraphQL

If you enable Alloy's [GraphQL API][graphql-api] with `--server.http.enable-graphql`, you can also request the component's `debugInfo` field:

```sh
curl --fail --silent --show-error http://localhost:12345/graphql \
  -H 'Content-Type: application/json' \
  --data '{"query":"{ component(id: \"prometheus.preview.candidate\") { id debugInfo } }"}' \
  | jq '.data.component.debugInfo | fromjson'
```

The GraphQL API is experimental and disabled by default, independently of the component's GA stability.
Its `debugInfo` field is a JSON-encoded string, so this example decodes it with `fromjson`.
Both APIs expose the same cached report as the UI and don't change how `report_to` operates.

[report-metrics]: #report-metrics
[graphql-api]: ../../../http/graphql/

## Debug metrics

The following Prometheus metrics are exposed on Alloy's metrics endpoint:

| Name | Type | Description |
| ---- | ---- | ----------- |
| `prometheus_preview_report_failures_total` | `counter` | Reports that fail to reach one or more receivers. |

### Report metrics

The following metrics go directly to `report_to`; you don't need to scrape Alloy to send them.
Every report metric has a `preview_id` label containing the full component ID, such as `prometheus.preview.candidate`.

| Name | Type | Description |
| ---- | ---- | ----------- |
| `prometheus_preview_samples_total` | `counter` | Committed samples since the observation started, with `sample_type="float"` or `sample_type="histogram"`. |
| `prometheus_preview_series` | `gauge` | Distinct series fingerprints observed so far in the current window. |
| `prometheus_preview_series_limited` | `gauge` | `1` if the corresponding series count is a lower bound due to resource limits; otherwise `0`. |
| `prometheus_preview_observation_start_time_seconds` | `gauge` | Unix timestamp of the observation's start or latest measurement reset. |
| `prometheus_preview_window_start_time_seconds` | `gauge` | Unix timestamp of the current report's window start. |
| `prometheus_preview_window_duration_seconds` | `gauge` | Elapsed observation time in the current report's window. |
| `prometheus_preview_window_complete` | `gauge` | `1` for a final window report; `0` for a partial report. |

The first three metrics also have a `preview_scope` label:

- `total` measures all input samples, regardless of grouping.
- `group` measures an admitted group and includes the configured `preview_group_` labels.
- `overflow` measures samples whose groups exceed the limits and contains no input label values.

Reports include `group` and `overflow` scopes only when you configure `group_by`.
Don't sum `total` together with the other scopes: the total already includes their samples.
Without grouping, each report contains eight metric series.
With grouping, each observation produces at most `4 * (max_groups + 2) + 4` report series: four per named group, total, and overflow, plus four observation fields.
A reset can admit different groups and introduce additional report series in your backend.

Use distinct external labels on your report writer for each Alloy instance.
Component IDs alone aren't globally unique.
Otherwise, multiple instances with the same component ID can write conflicting samples to the same report series.
Don't sum series counts across instances to estimate unique fleet cardinality without accounting for overlap.

## Examples

In these examples, preview branches send only summary metrics to Grafana Cloud.
Existing production branches continue to send their original metrics.
The examples require reachable scrape targets.

### Preview a candidate pipeline in Grafana Cloud

Set these environment variables for the Alloy process:

- Set `GRAFANA_CLOUD_PROMETHEUS_URL` to your stack's Prometheus remote-write URL.
- Set `GRAFANA_CLOUD_METRICS_USER` to your stack's metrics username.
- Set `GRAFANA_CLOUD_API_KEY` to an access policy token with `metrics:write` permission.
- Set `ALLOY_INSTANCE_ID` to a non-empty identifier unique to this Alloy instance.

Save this configuration as `preview.alloy`:

```alloy
prometheus.scrape "candidate" {
	targets = [{
		"__address__" = "localhost:9100",
		"namespace"   = "candidate",
	}]
	scrape_interval = "15s"
	forward_to      = [prometheus.relabel.candidate.receiver]
}

prometheus.relabel "candidate" {
	rule {
		source_labels = ["__name__"]
		regex         = "node_.+"
		action        = "keep"
	}
	forward_to = [prometheus.preview.candidate.receiver]
}

prometheus.preview "candidate" {
	report_to       = [prometheus.remote_write.reports.receiver]
	report_interval = "1m"
	window          = "30m"
	group_by        = ["namespace"]
	max_groups      = 100
	max_series      = 100000
}

prometheus.remote_write "reports" {
	external_labels = {
		"alloy_instance" = sys.env("ALLOY_INSTANCE_ID"),
	}

	endpoint {
		url = sys.env("GRAFANA_CLOUD_PROMETHEUS_URL")

		basic_auth {
			username = sys.env("GRAFANA_CLOUD_METRICS_USER")
			password = sys.env("GRAFANA_CLOUD_API_KEY")
		}
	}
}
```

This example scrapes a node exporter on `localhost:9100`, keeps metrics whose names start with `node_`, and measures them without sending their values to Grafana Cloud.
Replace the address and relabel rule to match your candidate system.
The `namespace` target label supplies the grouping dimension; `group_by` doesn't discover Kubernetes metadata.
The report writer is separate from any existing production pipeline.

Validate and run the configuration:

```sh
alloy validate preview.alloy
alloy run preview.alloy
```

### Read the report in Grafana Cloud

Open **Explore** in your Grafana Cloud stack and select its Prometheus data source.
After the first report arrives, query the sample throughput across both sample types:

```promql
sum by (alloy_instance, preview_id) (
  rate(prometheus_preview_samples_total{preview_scope="total"}[5m])
)
```

With the default report interval, allow several minutes for the rate query to have enough samples.
To inspect the current window's cardinality, use:

```promql
prometheus_preview_series{preview_scope="total"}
```

Before interpreting that cardinality, check the limit flag and elapsed observation time:

```promql
prometheus_preview_series_limited{preview_scope="total"}
prometheus_preview_window_duration_seconds
```

Run these as separate queries.
A limit flag of `1` means the count is a lower bound.
Use `prometheus_preview_window_complete` and `prometheus_preview_window_start_time_seconds` to distinguish final results from partial windows in a dashboard.
Use `prometheus_preview_observation_start_time_seconds` to identify resets.

To inspect namespace sample throughput, including groups that exceed the limit, use:

```promql
sum by (alloy_instance, preview_id, preview_scope, preview_group_namespace) (
  rate(prometheus_preview_samples_total{preview_scope=~"group|overflow"}[5m])
)
```

To check report age in seconds, use:

```promql
time() - timestamp(prometheus_preview_window_duration_seconds)
```

If reports are absent, check that Alloy is running and the remote-write credentials and endpoint are correct.
The component also emits zero-valued total reports without input; these show that reporting works, not that the candidate scrape succeeded.
Check `prometheus.scrape` health and target status separately.

### Add a payments job to an existing production pipeline

This example starts with an Alloy instance that already sends node metrics to Grafana Cloud.
You want to add a payments application's `/metrics` endpoint without immediately ingesting its metrics.
You preview the job, identify a high-cardinality metric, preview a filter, and then enable ingestion.

The service addresses and metric names below describe an example deployment.
Replace them with your application and exporter addresses and metric names.
Use the Grafana Cloud environment variables from [Preview a candidate pipeline in Grafana Cloud][cloud-example], and set `ALLOY_INSTANCE_ID` to `alloy-prod-1` for this instance.
The measurements below are illustrative, not results collected from your system.

#### Start with the existing job

Your running `config.alloy` contains this complete pipeline:

```alloy
prometheus.scrape "node" {
	targets         = [{"__address__" = "node-exporter:9100"}]
	job_name        = "node"
	scrape_interval = "15s"
	forward_to      = [prometheus.remote_write.production.receiver]
}

prometheus.remote_write "production" {
	external_labels = {
		"alloy_instance" = sys.env("ALLOY_INSTANCE_ID"),
	}

	endpoint {
		url = sys.env("GRAFANA_CLOUD_PROMETHEUS_URL")

		basic_auth {
			username = sys.env("GRAFANA_CLOUD_METRICS_USER")
			password = sys.env("GRAFANA_CLOUD_API_KEY")
		}
	}
}
```

Keep both blocks unchanged during preview.
In this example, your acceptance limits for the new job are 5,000 observed series per 30-minute window and 400 samples per second.
These are your operational limits, not Grafana Cloud billing thresholds.

#### Add the new job in preview mode

Append these three blocks to the same `config.alloy`:

```alloy
prometheus.scrape "payments" {
	targets         = [{"__address__" = "payments:8080"}]
	job_name        = "payments"
	scrape_interval = "15s"
	forward_to      = [prometheus.relabel.payments.receiver]
}

prometheus.relabel "payments" {
	forward_to = [prometheus.preview.payments.receiver]
}

prometheus.preview "payments" {
	report_to       = [prometheus.remote_write.production.receiver]
	report_interval = "1m"
	window          = "30m"
	group_by        = ["__name__"]
	max_groups      = 100
	max_series      = 100000
}
```

The payments application exposes metrics at `http://payments:8080/metrics`.
The relabel component initially applies no rules; it provides a place to test a filter later.
Grouping by `__name__` sends metric names in reports, but not the other application label values.

The two branches now behave as follows:

```text
node scrape ───────────────────────────────────→ production remote_write → Grafana Cloud
payments scrape → payments relabel → preview ──→ production remote_write → Grafana Cloud
                                     │           (summary metrics only)
                                     └─ discard original payments samples
```

Validate the file and reload the existing Alloy process; don't start a second process against the same targets.
For a local Alloy HTTP server listening on port 12345, use:

```sh
alloy validate config.alloy
curl --fail --request POST http://localhost:12345/-/reload
```

Run the reload only if validation succeeds.
Use your deployment's reload mechanism if its HTTP server isn't available at this address.
Confirm that the node job still sends metrics and that the payments scrape target is healthy.
Wait for at least one complete 30-minute preview window during representative application traffic.

#### Read the report and decide what to change

In Grafana Cloud **Explore**, select the Prometheus data source and a time range that includes the completed window.
Use a range query to show cardinality only at completed report timestamps.
Set its query step to one minute or less so you don't skip the final report between partial reports:

```promql
prometheus_preview_series{
  alloy_instance="alloy-prod-1",
  preview_id="prometheus.preview.payments",
  preview_scope="total"
}
and on (alloy_instance, preview_id)
(
  prometheus_preview_window_complete{
    alloy_instance="alloy-prod-1",
    preview_id="prometheus.preview.payments"
  } == 1
)
```

Query sample throughput separately:

```promql
sum (
  rate(prometheus_preview_samples_total{
    alloy_instance="alloy-prod-1",
    preview_id="prometheus.preview.payments",
    preview_scope="total"
  }[5m])
)
```

At the completed report timestamp, check that `prometheus_preview_series_limited` is `0` for the total and any groups you use to decide.
Check the reported window duration, observation start time, and report freshness as described in [Read the report in Grafana Cloud][read-report].
Don't treat an empty query, a failed scrape, a partial window, or a limited count as evidence that the job fits your limits.

Suppose the completed report shows:

| Measurement | Observed value | Your limit | Decision |
| ----------- | -------------- | ---------- | -------- |
| Window cardinality | About 12,000 series | 5,000 series | Don't enable ingestion yet. |
| Sample throughput | About 800 samples per second | 400 samples per second | Reduce the candidate volume. |

To find which metric contributes the most series, query the breakdown at the same report timestamp:

```promql
topk(5, prometheus_preview_series{
  alloy_instance="alloy-prod-1",
  preview_id="prometheus.preview.payments",
  preview_scope="group"
})
```

Suppose `preview_group___name__="payments_request_debug_total"` accounts for about 9,000 series.
If significant volume falls into the `overflow` scope, you don't have a complete named breakdown; adjust the grouping limits and start a new observation before attributing that volume.

In this example, the application exposes `payments_request_debug_total` with a `request_id` label for temporary diagnostics.
After confirming that you don't need this metric for production dashboards or alerts, you choose to drop the entire metric.
Don't merely remove `request_id`: that can give distinct samples identical label sets rather than safely aggregating them.

#### Preview the filtered job in a new observation

Replace the `prometheus.relabel "payments"` block with the version below.
Remove `prometheus.preview "payments"` and add `prometheus.preview "payments_filtered"`:

```alloy
prometheus.relabel "payments" {
	rule {
		source_labels = ["__name__"]
		regex         = "payments_request_debug_total"
		action        = "drop"
	}
	forward_to = [prometheus.preview.payments_filtered.receiver]
}

prometheus.preview "payments_filtered" {
	report_to       = [prometheus.remote_write.production.receiver]
	report_interval = "1m"
	window          = "30m"
	group_by        = ["__name__"]
	max_groups      = 100
	max_series      = 100000
}
```

Leave both scrape blocks and the production remote writer unchanged.
Validate and reload again.
Using a new preview component ID starts a clean observation and keeps the two reports distinguishable in Grafana Cloud.
Changing only an upstream relabel rule doesn't reset an existing preview's counters or its current window, so don't compare mixed pre-filter and post-filter results.

Repeat the queries with `preview_id="prometheus.preview.payments_filtered"` after a complete window under comparable traffic.
Suppose this report shows about 3,000 series and 200 samples per second, with no cardinality limit flags and healthy scraping.
Both measurements fit your operational limits, so you decide to enable the filtered job.
This comparison isn't an exact billing forecast: workload changes and overlap with existing tenant data can affect the eventual ingestion impact.

#### Enable ingestion for the filtered job

Change `prometheus.relabel.payments.forward_to` to the production writer and remove `prometheus.preview "payments_filtered"`.
Preserve the filter and all unrelated production components.
For this example, the complete final configuration is:

```alloy
prometheus.scrape "node" {
	targets         = [{"__address__" = "node-exporter:9100"}]
	job_name        = "node"
	scrape_interval = "15s"
	forward_to      = [prometheus.remote_write.production.receiver]
}

prometheus.scrape "payments" {
	targets         = [{"__address__" = "payments:8080"}]
	job_name        = "payments"
	scrape_interval = "15s"
	forward_to      = [prometheus.relabel.payments.receiver]
}

prometheus.relabel "payments" {
	rule {
		source_labels = ["__name__"]
		regex         = "payments_request_debug_total"
		action        = "drop"
	}
	forward_to = [prometheus.remote_write.production.receiver]
}

prometheus.remote_write "production" {
	external_labels = {
		"alloy_instance" = sys.env("ALLOY_INSTANCE_ID"),
	}

	endpoint {
		url = sys.env("GRAFANA_CLOUD_PROMETHEUS_URL")

		basic_auth {
			username = sys.env("GRAFANA_CLOUD_METRICS_USER")
			password = sys.env("GRAFANA_CLOUD_API_KEY")
		}
	}
}
```

Validate and reload once more.
The node job remains unchanged, and the payments job now sends its retained original metrics through the existing production writer.
Neither preview component remains in the configuration.
Keep the production writer: both jobs still need it.
Previously discarded payments samples aren't replayed.

In Grafana Cloud, confirm that `up{job="node", alloy_instance="alloy-prod-1"}` and `up{job="payments", alloy_instance="alloy-prod-1"}` are `1`, and that an expected retained payments metric arrives.
`up` confirms scrape success, not the presence of every application metric, so check both.
If the filtered preview doesn't meet your limits, keep it in preview while you investigate, or remove only the payments scrape, relabel, and preview blocks to abandon the new job.

[cloud-example]: #preview-a-candidate-pipeline-in-grafana-cloud
[read-report]: #read-the-report-in-grafana-cloud

### Insert preview into an existing pipeline

To preview an existing candidate branch, replace its final `forward_to` destination with the preview receiver.
For the pipeline above, the final processing component points only to:

```alloy
forward_to = [prometheus.preview.candidate.receiver]
```

Add the `prometheus.preview` block and a report writer from the complete example.
Leave unrelated production pipelines unchanged.
Original samples already queued in an existing remote writer can still be delivered after the change.

{{< admonition type="warning" >}}
Don't list both the preview receiver and a production remote-write receiver in the candidate's `forward_to` list unless you intend to ingest the original samples too.
Preview can't prevent another branch from sending them.
Discarded samples aren't buffered for later promotion and can't be recovered.
{{< /admonition >}}

### Enable ingestion or stop the preview

To enable ingestion after reviewing the report:

1. Wait for the report you need to arrive in Grafana Cloud. Removing the component doesn't guarantee a final report.
1. Change the candidate branch's final `forward_to` to your production remote-write receiver.
1. Remove the now-unused `prometheus.preview` block.
1. Remove the report writer if nothing else uses it.
1. Validate and reload your configuration.

For the complete example, you can reuse its remote writer by changing the `prometheus.relabel.candidate` destination to:

```alloy
forward_to = [prometheus.remote_write.reports.receiver]
```

Then remove `prometheus.preview.candidate`, but keep `prometheus.remote_write.reports`.
Despite its instance label `reports`, that writer now sends the original retained `node_` metrics.
Only newly collected samples are available; preview doesn't replay previously discarded data.

To stop preview without enabling ingestion, remove the candidate scrape and any processing components used only by it, together with the preview component.
Don't replace the preview destination with a remote writer.

For related configuration, refer to [prometheus.scrape][], [prometheus.relabel][], and [prometheus.remote_write][].

[prometheus.scrape]: ../prometheus.scrape/
[prometheus.relabel]: ../prometheus.relabel/
[prometheus.remote_write]: ../prometheus.remote_write/
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`prometheus.preview` can accept arguments from the following components:

- Components that export [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-exporters)

`prometheus.preview` has exports that can be consumed by the following components:

- Components that consume [Prometheus `MetricsReceiver`](../../../compatibility/#prometheus-metricsreceiver-consumers)

{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
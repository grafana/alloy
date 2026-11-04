---
canonical: https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.route/
description: Learn about prometheus.route
labels:
  stage: experimental
  products:
    - oss
title: prometheus.route
---

# `prometheus.route`

{{< docs/shared lookup="stability/experimental.md" source="alloy" version="<ALLOY_VERSION>" >}}

`prometheus.route` forwards Prometheus metrics to receivers based on series labels.
It evaluates every route and forwards each series to all matching routes without changing its labels.
Series that don't match any route go to the default receivers.

You can specify multiple `prometheus.route` components by giving them different labels.

## Usage

```alloy
prometheus.route "<LABEL>" {
	default_forward_to = <RECEIVER_LIST>
}
```

## Arguments

You can use the following arguments with `prometheus.route`:

| Name                 | Type                    | Description                                        | Default | Required |
| -------------------- | ----------------------- | -------------------------------------------------- | ------- | -------- |
| `default_forward_to` | `list(MetricsReceiver)` | Receivers for series that don't match any route.   |         | yes      |

Set `default_forward_to = []` to drop unmatched series.
If you don't configure any `route` blocks, all series go to `default_forward_to`.
The default receivers aren't failover destinations: errors from a matching route don't cause forwarding to the default receivers.

`prometheus.route` doesn't buffer samples or retry failed appends.
It attempts every selected receiver and returns downstream errors to the upstream component.
Forwarding to multiple receivers isn't atomic.

An appender transaction uses the routing configuration that was active when the transaction started.
New transactions use the updated configuration.
Configuration updates don't move previously buffered data between destinations or generate stale markers for old destinations.
Incoming stale markers follow the same routing rules as other samples.

## Blocks

You can use the following blocks with `prometheus.route`:

{{< docs/alloy-config >}}

| Block              | Description                         | Required |
| ------------------ | ----------------------------------- | -------- |
| [`route`][route]   | Select series and their receivers. | no       |

[route]: #route

{{< /docs/alloy-config >}}

### `route`

Each unnamed `route` block selects series using a Prometheus metric selector and forwards them to its receivers.

The following arguments are supported:

| Name         | Type                    | Description                                      | Default | Required |
| ------------ | ----------------------- | ------------------------------------------------ | ------- | -------- |
| `forward_to` | `list(MetricsReceiver)` | Receivers for series that match this route.      |         | yes      |
| `selector`   | `string`               | Prometheus metric selector to match series.      |         | yes      |

The `forward_to` list must contain at least one entry.
Every matching route forwards to every entry in its `forward_to` list.
Receivers aren't deduplicated: if a receiver appears in two matching routes, or twice in one list, the component forwards the sample to it twice.
A downstream receiver or backend may deduplicate identical samples.

Selectors support metric names and label matchers with `=`, `!=`, `=~`, and `!~`.
All matchers within a selector must match.
Regular expressions match the entire label value, and missing labels match as empty strings.
The selector `{}` matches all series.

For example, the following route matches series whose `tenant` label is `a` and whose `__name__` label starts with `http_`:

```alloy
route {
	selector   = "{tenant=\"a\", __name__=~\"http_.*\"}"
	forward_to = [prometheus.remote_write.tenant_a.receiver]
}
```

Selectors don't support full PromQL expressions such as `rate(http_requests_total[5m])`, sample-value comparisons, or LogQL expressions.
Invalid selectors cause a configuration error.

Routing applies to individual series, not metric families.
For example, selecting classic histogram `_bucket` series doesn't automatically select their `_sum` or `_count` series.
Float samples, native histograms, metadata updates, exemplars, and start-timestamp operations use the parent series labels for routing.
Exemplar labels aren't routing inputs.
To change series labels, use [`prometheus.relabel`](../prometheus.relabel/) before or after `prometheus.route`.

## Exported fields

The following fields are exported and can be referenced by other components:

| Name       | Type              | Description                       |
| ---------- | ----------------- | --------------------------------- |
| `receiver` | `MetricsReceiver` | Receives metrics to route.       |

## Component health

`prometheus.route` is only reported as unhealthy if given an invalid configuration.
Invalid updates retain the last valid routing configuration.

## Debug information

`prometheus.route` doesn't expose any component-specific debug information.

## Debug metrics

The following Prometheus metrics are exposed:

| Name                                             | Type        | Description                                                                 |
| ------------------------------------------------ | ----------- | --------------------------------------------------------------------------- |
| `alloy_prometheus_route_samples_processed_total` | `counter`   | Number of float and histogram samples processed.                           |
| `alloy_prometheus_route_samples_forwarded_total` | `counter`   | Number of successful downstream sample appends, including duplicates.      |
| `alloy_prometheus_route_samples_dropped_total`   | `counter`   | Number of samples with no available destination.                           |
| `prometheus_fanout_latency`                     | `histogram` | Write latency through the routing component and its downstream components. |
| `prometheus_forwarded_samples_total`            | `counter`   | Number of successful float-sample appends into the routing component.      |

The route counters exclude exemplar, metadata, and start-timestamp operations.
The forwarded counter counts each successful receiver append separately, before the transaction commits.
It doesn't indicate delivery to a remote backend.
The dropped counter includes unmatched samples when `default_forward_to` is empty and samples whose selected receivers aren't available yet.

## Example

This example sends tenant `a` metrics to one destination and critical metrics to another.
Metrics that match both routes go to both destinations.
Unmatched metrics go to the default destination:

```alloy
prometheus.scrape "app" {
	targets    = [{"__address__" = "app:8080"}]
	forward_to = [prometheus.route.app.receiver]
}

prometheus.route "app" {
	default_forward_to = [prometheus.remote_write.other.receiver]

	route {
		selector   = "{tenant=\"a\"}"
		forward_to = [prometheus.remote_write.tenant_a.receiver]
	}

	route {
		selector   = "{priority=\"critical\"}"
		forward_to = [prometheus.remote_write.critical.receiver]
	}
}

prometheus.remote_write "tenant_a" {
	endpoint {
		url = "https://tenant-a.example.com/api/v1/write"
	}
}

prometheus.remote_write "critical" {
	endpoint {
		url = "https://critical.example.com/api/v1/write"
	}
}

prometheus.remote_write "other" {
	endpoint {
		url = "https://other.example.com/api/v1/write"
	}
}
```

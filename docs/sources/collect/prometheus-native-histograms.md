---
canonical: https://grafana.com/docs/alloy/latest/collect/prometheus-native-histograms/
description: Learn how to scrape and forward Prometheus native histograms with Grafana Alloy
title: Collect Prometheus native histograms
docTitle: Collect Prometheus native histograms
weight: 310
---

# Collect Prometheus native histograms

[Prometheus native histograms][] need configuration on **both** the scrape side and the write side.
If only one side is enabled, samples are silently dropped or never requested from the target.

This topic shows a minimal end-to-end pipeline that:

* Scrapes native histograms with [`prometheus.scrape`][prometheus.scrape]
* Forwards them with [`prometheus.remote_write`][prometheus.remote_write]

## Components used in this topic

* [`prometheus.remote_write`][prometheus.remote_write]
* [`prometheus.scrape`][prometheus.scrape]

## Before you begin

* A scrape target that exposes native histograms over the Prometheus Protobuf format.
* A remote endpoint that accepts native histograms, for example Grafana Mimir or Grafana Cloud Metrics with native histograms enabled.

## Configure delivery

Native histogram samples only leave {{< param "PRODUCT_NAME" >}} when `send_native_histograms` is `true` on the remote write endpoint:

```alloy
prometheus.remote_write "mimir" {
  endpoint {
    url = "https://mimir.example.com/api/v1/push"

    send_native_histograms = true
  }
}
```

Without `send_native_histograms = true`, classic metrics still flow, but native histogram samples are not sent.

## Configure scraping

Native histograms are negotiated through the Prometheus Protobuf exposition format.
Set `scrape_native_histograms = true` and put `PrometheusProto` first in `scrape_protocols`:

```alloy
prometheus.scrape "app" {
  targets = [
    {"__address__" = "app.default.svc.cluster.local:8080"},
  ]

  scrape_native_histograms = true
  scrape_protocols = [
    "PrometheusProto",
    "OpenMetricsText1.0.0",
    "OpenMetricsText0.0.1",
    "PrometheusText1.0.0",
    "PrometheusText0.0.4",
  ]

  forward_to = [prometheus.remote_write.mimir.receiver]
}
```

When `scrape_native_histograms` is `true`, {{< param "PRODUCT_NAME" >}} already defaults `scrape_protocols` to start with `PrometheusProto`. Setting the list explicitly makes the requirement obvious in the config.

## Full example

```alloy
prometheus.remote_write "mimir" {
  endpoint {
    url = "https://mimir.example.com/api/v1/push"

    send_native_histograms = true
  }
}

prometheus.scrape "app" {
  targets = [
    {"__address__" = "app.default.svc.cluster.local:8080"},
  ]

  scrape_native_histograms = true
  scrape_protocols = [
    "PrometheusProto",
    "OpenMetricsText1.0.0",
    "OpenMetricsText0.0.1",
    "PrometheusText1.0.0",
    "PrometheusText0.0.4",
  ]

  forward_to = [prometheus.remote_write.mimir.receiver]
}
```

## Checklist

| Component | Setting | Why |
|-----------|---------|-----|
| `prometheus.scrape` | `scrape_native_histograms = true` | Request native histograms from the target |
| `prometheus.scrape` | `scrape_protocols` starts with `PrometheusProto` | Native histograms use the Protobuf exposition format |
| `prometheus.remote_write` endpoint | `send_native_histograms = true` | Forward native histogram samples to the backend |

Other pipeline components such as `prometheus.relabel` do not need extra flags for native histograms.

## Related

* [`prometheus.scrape` native histogram arguments][prometheus.scrape]
* [`prometheus.remote_write` `send_native_histograms`][prometheus.remote_write]
* [Collect Prometheus metrics][]

[Prometheus native histograms]: https://prometheus.io/docs/specs/native_histograms/
[Collect Prometheus metrics]: ../prometheus-metrics/
[prometheus.remote_write]: ../../reference/components/prometheus/prometheus.remote_write/
[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/

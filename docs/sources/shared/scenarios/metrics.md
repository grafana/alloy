---
canonical: https://grafana.com/docs/alloy/latest/shared/scenarios/metrics/
description: Shared content, Alloy scenarios, Metrics
headless: true
---

<!--
This file is generated from scenarios.yml in https://github.com/grafana/alloy-scenarios.
Don't edit it directly. Edit scenarios.yml in alloy-scenarios. A weekly workflow
in grafana/alloy copies the change into the Alloy documentation.
-->

| Scenario | Description |
| -------- | ----------- |
| [Blackbox probing][] | Monitor endpoint availability and response times with synthetic HTTP probes. |
| [Datadog receiver migration][] | Ingest Datadog Agent metrics and traces with `otelcol.receiver.datadog` and forward them to Prometheus and Tempo. |
| [DNS service discovery][] | Discover and scrape two Prometheus targets from DNS SRV records with `discovery.dns`. |
| [Prometheus Operator Probes][] | Scrape Prometheus Operator `Probe` resources with Alloy as the blackbox prober (`/probe` path). |
| [Metric cardinality control][] | Compare original Prometheus metrics with a cardinality-controlled `prometheus.relabel` path that drops noisy series and volatile labels and normalizes dynamic routes. |
| [OpenTelemetry SDK metrics across languages][] | Instrument five languages with the OpenTelemetry metrics SDK and push them through Alloy to Prometheus. |
| [Prometheus client metrics across languages][] | Expose `/metrics` with native Prometheus client libraries in five languages and scrape them with Alloy. The pull-model counterpart to the OpenTelemetry SDK scenario. |
| [Prometheus Pushgateway][] | Scrape metrics retained for short-lived batch jobs by Prometheus Pushgateway and remote-write them through Alloy. |
| [OTel metrics pipeline][] | Forward OpenTelemetry metrics from applications through Alloy. Alloy batches and transforms samples before it sends them to Prometheus. |
| [StatsD metrics][] | Receive legacy StatsD counter, gauge, and timer packets in Alloy and remote-write mapped Prometheus metrics. |

Refer to the [alloy-scenarios][] repository for the full list of scenarios.

[Blackbox probing]: https://github.com/grafana/alloy-scenarios/tree/main/blackbox-probing
[Datadog receiver migration]: https://github.com/grafana/alloy-scenarios/tree/main/datadog-receiver-migration
[DNS service discovery]: https://github.com/grafana/alloy-scenarios/tree/main/dns-service-discovery
[Prometheus Operator Probes]: https://github.com/grafana/alloy-scenarios/tree/main/k8s/prometheus-operator-probes
[Metric cardinality control]: https://github.com/grafana/alloy-scenarios/tree/main/metric-cardinality-control
[OpenTelemetry SDK metrics across languages]: https://github.com/grafana/alloy-scenarios/tree/main/app-instrumentation/metrics/opentelemetry-sdk
[Prometheus client metrics across languages]: https://github.com/grafana/alloy-scenarios/tree/main/app-instrumentation/metrics/prometheus-client
[Prometheus Pushgateway]: https://github.com/grafana/alloy-scenarios/tree/main/prometheus-pushgateway
[OTel metrics pipeline]: https://github.com/grafana/alloy-scenarios/tree/main/otel-metrics-pipeline
[StatsD metrics]: https://github.com/grafana/alloy-scenarios/tree/main/statsd-metrics
[alloy-scenarios]: https://github.com/grafana/alloy-scenarios

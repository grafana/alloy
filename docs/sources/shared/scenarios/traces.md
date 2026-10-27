---
canonical: https://grafana.com/docs/alloy/latest/shared/scenarios/traces/
description: Shared content, Alloy scenarios, Traces
headless: true
---

<!--
This file is generated from scenarios.yml in https://github.com/grafana/alloy-scenarios.
Don't edit it directly. Edit scenarios.yml in alloy-scenarios. A weekly workflow
in grafana/alloy copies the change into the Alloy documentation.
-->

| Scenario | Description |
| -------- | ----------- |
| [Distributed tracing][] | Learn distributed tracing through a sofa delivery workflow from order to doorstep. |
| [Game of tracing][] | Play an interactive strategy game that teaches distributed tracing, sampling, and service graphs. |
| [OpenTelemetry basic tracing][] | Collect and visualize OpenTelemetry traces with Alloy and Tempo. |
| [OpenTelemetry Collector to Alloy migration][] | Compare equivalent OTLP pipelines in the OpenTelemetry Collector and Alloy with matching telemetry. |
| [OpenTelemetry SDK traces across languages][] | Instrument five languages with the OpenTelemetry tracing SDK and collect standalone traces through Alloy into Tempo. |
| [OpenTelemetry Jaeger and Zipkin receivers][] | Ingest Jaeger and Zipkin trace formats with Alloy and forward them to Tempo over OTLP. |
| [Zero-code eBPF instrumentation][] | Auto-instrument an unmodified Go HTTP service with `beyla.ebpf` -- no OpenTelemetry SDK, no agent, no code changes. Produces RED metrics and traces from eBPF probes alone. |
| [OpenTelemetry load balancing][] | Shard traces across two tail-sampling Alloy instances with `otelcol.exporter.loadbalancing` -- same trace ID, same backend. |
| [OpenTelemetry service graphs][] | Generate service graphs with the Alloy `servicegraph` connector. |
| [OpenTelemetry span metrics][] | Generate RED metrics from OpenTelemetry traces with the span metrics connector. Request rate, error rate, and duration. |
| [OpenTelemetry tail sampling][] | Apply tail sampling policies to OpenTelemetry traces with Alloy and Tempo. |
| [Trace correlation with exemplars][] | Jump from a latency histogram to the exact trace behind it with OpenMetrics exemplars flowing through Alloy into Prometheus and Tempo. |

Refer to the [alloy-scenarios][] repository for the full list of scenarios.

[Distributed tracing]: https://github.com/grafana/alloy-scenarios/tree/main/trace-delivery
[Game of tracing]: https://github.com/grafana/alloy-scenarios/tree/main/game-of-tracing
[OpenTelemetry basic tracing]: https://github.com/grafana/alloy-scenarios/tree/main/otel-basic-tracing
[OpenTelemetry Collector to Alloy migration]: https://github.com/grafana/alloy-scenarios/tree/main/otel-collector-to-alloy-migration
[OpenTelemetry SDK traces across languages]: https://github.com/grafana/alloy-scenarios/tree/main/app-instrumentation/traces/opentelemetry-sdk
[OpenTelemetry Jaeger and Zipkin receivers]: https://github.com/grafana/alloy-scenarios/tree/main/otel-jaeger-zipkin-receiver
[Zero-code eBPF instrumentation]: https://github.com/grafana/alloy-scenarios/tree/main/beyla-zero-code-instrumentation
[OpenTelemetry load balancing]: https://github.com/grafana/alloy-scenarios/tree/main/otel-loadbalancing
[OpenTelemetry service graphs]: https://github.com/grafana/alloy-scenarios/tree/main/otel-tracing-service-graphs
[OpenTelemetry span metrics]: https://github.com/grafana/alloy-scenarios/tree/main/otel-span-metrics
[OpenTelemetry tail sampling]: https://github.com/grafana/alloy-scenarios/tree/main/otel-tail-sampling
[Trace correlation with exemplars]: https://github.com/grafana/alloy-scenarios/tree/main/trace-log-correlation-exemplars
[alloy-scenarios]: https://github.com/grafana/alloy-scenarios

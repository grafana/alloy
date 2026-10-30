---
canonical: https://grafana.com/docs/alloy/latest/shared/scenarios/opentelemetry-engine/
description: Shared content, Alloy scenarios, OpenTelemetry Engine
headless: true
---

<!--
This file is generated from scenarios.yml in https://github.com/grafana/alloy-scenarios.
Don't edit it directly. Edit scenarios.yml in alloy-scenarios. A weekly workflow
in grafana/alloy copies the change into the Alloy documentation.
-->

| Scenario | Description |
| -------- | ----------- |
| [Cost control][] | Drop health checks, filter debug logs, and apply probabilistic sampling to cut telemetry volume. |
| [Count connector][] | Derive request rate and error rate metrics from traces and logs with the `count` connector. |
| [File log processing][] | Collect and parse mixed-format log files with the OTel `filelog` receiver and operator chains. |
| [Host metrics][] | Collect CPU, memory, disk, and network metrics with the `hostmetrics` receiver. |
| [Kafka buffer][] | Buffer traces through Kafka for durability and `backpressure` control. |
| [Multi-pipeline fan-out][] | Send traces to two backends. Each destination runs its own process path. |
| [Multi-tenant routing][] | Route logs to different Loki tenants based on resource attributes with fan-out and filter. |
| [OTTL transform cookbook][] | A cookbook of OpenTelemetry Transformation Language patterns for JSON parsing, severity mapping, attribute promotion, and truncation. |
| [PII redaction][] | Scrub personally identifiable information, credit cards, emails, and IP addresses from traces and logs with OpenTelemetry Transformation Language `replace_pattern`. |
| [Resource enrichment][] | Attach host, OS, and Docker metadata to all signals with `resourcedetection`. |

Refer to the [alloy-scenarios][] repository for the full list of scenarios.

[Cost control]: https://github.com/grafana/alloy-scenarios/tree/main/otel-examples/cost-control
[Count connector]: https://github.com/grafana/alloy-scenarios/tree/main/otel-examples/count-connector
[File log processing]: https://github.com/grafana/alloy-scenarios/tree/main/otel-examples/filelog-processing
[Host metrics]: https://github.com/grafana/alloy-scenarios/tree/main/otel-examples/host-metrics
[Kafka buffer]: https://github.com/grafana/alloy-scenarios/tree/main/otel-examples/kafka-buffer
[Multi-pipeline fan-out]: https://github.com/grafana/alloy-scenarios/tree/main/otel-examples/multi-pipeline-fanout
[Multi-tenant routing]: https://github.com/grafana/alloy-scenarios/tree/main/otel-examples/routing-multi-tenant
[OTTL transform cookbook]: https://github.com/grafana/alloy-scenarios/tree/main/otel-examples/ottl-transform
[PII redaction]: https://github.com/grafana/alloy-scenarios/tree/main/otel-examples/pii-redaction
[Resource enrichment]: https://github.com/grafana/alloy-scenarios/tree/main/otel-examples/resource-enrichment
[alloy-scenarios]: https://github.com/grafana/alloy-scenarios

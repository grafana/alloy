---
canonical: https://grafana.com/docs/alloy/latest/shared/scenarios/logs/
description: Shared content, Alloy scenarios, Logs
headless: true
---

<!--
This file is generated from scenarios.yml in https://github.com/grafana/alloy-scenarios.
Don't edit it directly. Edit scenarios.yml in alloy-scenarios. A weekly workflow
in grafana/alloy copies the change into the Alloy documentation.
-->

| Scenario | Description |
| -------- | ----------- |
| [Amazon Data Firehose logs][] | Ingest Amazon Data Firehose deliveries with `loki.source.awsfirehose`. Uses a local sender. No AWS account required. |
| [Azure Event Hubs logs][] | Ingest Azure Event Hubs messages with `loki.source.azure_event_hubs`. Uses a self-managed broker speaking the same wire protocol. No Azure subscription required. |
| [Faro frontend observability][] | Collect frontend web telemetry, including logs, errors, and web vitals, from browser applications with the Faro Web SDK. |
| [GELF log ingestion][] | Ingest Graylog Extended Log Format logs over `UDP`. |
| [Kafka logs][] | Consume and process logs from Apache Kafka topics. |
| [Log API gateway][] | Use Alloy as a centralized log gateway that accepts logs via a Loki-compatible push API endpoint. |
| [Log routing][] | Route logs from multiple sources to different Loki tenants based on log content and origin. |
| [Log secret filtering][] | Redact sensitive credentials and secrets from logs with pattern rules before storage. |
| [Log-to-metrics][] | Extract a request counter and latency histogram from application logs while forwarding the raw logs to Loki. |
| [Logs from file][] | Tail log files with Alloy. |
| [Logs over TCP][] | Receive and process TCP logs in JSON format. |
| [Popular logging frameworks][] | Parse logs from popular logging frameworks across 7 programming languages. |
| [Promtail to Alloy migration][] | Run Promtail (EOL March 2026) and its `alloy convert` equivalent side by side against one log file and verify identical results in Loki. |
| [Structured log parsing][] | Parse structured logs into labels and structured metadata. |
| [Syslog monitoring][] | Monitor non-RFC5424 compliant syslog messages with `rsyslog` and Alloy. |
| [systemd journal][] | Forward systemd journal entries to Loki with filters and labels tuned for fast queries. |
| [Windows security events][] | Forward Windows Security event logs to Loki with filters and field extraction for security operations center workflows. |

Refer to the [alloy-scenarios][] repository for the full list of scenarios.

[Amazon Data Firehose logs]: https://github.com/grafana/alloy-scenarios/tree/main/aws-firehose-logs
[Azure Event Hubs logs]: https://github.com/grafana/alloy-scenarios/tree/main/azure-event-hubs-logs
[Faro frontend observability]: https://github.com/grafana/alloy-scenarios/tree/main/faro-frontend-observability
[GELF log ingestion]: https://github.com/grafana/alloy-scenarios/tree/main/gelf-log-ingestion
[Kafka logs]: https://github.com/grafana/alloy-scenarios/tree/main/kafka
[Log API gateway]: https://github.com/grafana/alloy-scenarios/tree/main/log-api-gateway
[Log routing]: https://github.com/grafana/alloy-scenarios/tree/main/routing
[Log secret filtering]: https://github.com/grafana/alloy-scenarios/tree/main/log-secret-filtering
[Log-to-metrics]: https://github.com/grafana/alloy-scenarios/tree/main/log-to-metrics
[Logs from file]: https://github.com/grafana/alloy-scenarios/tree/main/logs-file
[Logs over TCP]: https://github.com/grafana/alloy-scenarios/tree/main/logs-tcp
[Popular logging frameworks]: https://github.com/grafana/alloy-scenarios/tree/main/app-instrumentation/logging/popular-logging-frameworks
[Promtail to Alloy migration]: https://github.com/grafana/alloy-scenarios/tree/main/promtail-to-alloy-migration
[Structured log parsing]: https://github.com/grafana/alloy-scenarios/tree/main/mail-house
[Syslog monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/syslog
[systemd journal]: https://github.com/grafana/alloy-scenarios/tree/main/systemd-journal
[Windows security events]: https://github.com/grafana/alloy-scenarios/tree/main/windows-events
[alloy-scenarios]: https://github.com/grafana/alloy-scenarios

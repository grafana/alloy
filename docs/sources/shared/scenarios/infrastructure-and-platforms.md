---
canonical: https://grafana.com/docs/alloy/latest/shared/scenarios/infrastructure-and-platforms/
description: Shared content, Alloy scenarios, Infrastructure and platforms
headless: true
---

<!--
This file is generated from scenarios.yml in https://github.com/grafana/alloy-scenarios.
Don't edit it directly. Edit scenarios.yml in alloy-scenarios. A weekly workflow
in grafana/alloy copies the change into the Alloy documentation.
-->

| Scenario | Description |
| -------- | ----------- |
| [Amazon CloudWatch metrics][] | Pull metrics from Amazon CloudWatch into Prometheus with `prometheus.exporter.cloudwatch`. Uses LocalStack for offline reproducibility. No AWS account required. |
| [Docker monitoring][] | Monitor Docker container metrics and logs. |
| [Kubernetes][] | Scenarios for Alloy on Kubernetes with the k8s-monitoring Helm chart or plain manifests. See subdirectories for logs, metrics, profiling, tracing, cluster events, and standalone kube-state-metrics + cAdvisor scraping. |
| [Linux monitoring][] | Collect Linux system metrics, journal entries, and log files with Alloy. |
| [NGINX monitoring][] | Monitor NGINX access and error logs plus `stub_status` metrics with Alloy. |
| [SNMP monitoring][] | Monitor devices with the Alloy SNMP exporter for Simple Network Management Protocol. |
| [Windows monitoring][] | Monitor Windows system metrics and Event Logs. |

Refer to the [alloy-scenarios][] repository for the full list of scenarios.

[Amazon CloudWatch metrics]: https://github.com/grafana/alloy-scenarios/tree/main/cloudwatch-metrics
[Docker monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/docker-monitoring
[Kubernetes]: https://github.com/grafana/alloy-scenarios/tree/main/k8s
[Linux monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/linux
[NGINX monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/nginx-monitoring
[SNMP monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/snmp
[Windows monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/windows
[alloy-scenarios]: https://github.com/grafana/alloy-scenarios

---
canonical: https://grafana.com/docs/alloy/latest/shared/scenarios/alloy-operations/
description: Shared content, Alloy scenarios, Alloy operations
headless: true
---

<!--
This file is generated from scenarios.yml in https://github.com/grafana/alloy-scenarios.
Don't edit it directly. Edit scenarios.yml in alloy-scenarios. A weekly workflow
in grafana/alloy copies the change into the Alloy documentation.
-->

| Scenario | Description |
| -------- | ----------- |
| [Alloy clustering][] | Run a three-node Alloy cluster that consistent-hashes `prometheus.scrape` targets across nodes. Stop a node and its targets redistribute to the survivors within seconds. |
| [Self-monitoring][] | Configure Alloy to monitor itself and collect its own metrics and logs. |
| [Vault secrets][] | Pull `prometheus.remote_write` `basic_auth` credentials from HashiCorp Vault at runtime with `remote.vault`. Credentials reload on rotation. |

Refer to the [alloy-scenarios][] repository for the full list of scenarios.

[Alloy clustering]: https://github.com/grafana/alloy-scenarios/tree/main/alloy-clustering
[Self-monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/self-monitoring
[Vault secrets]: https://github.com/grafana/alloy-scenarios/tree/main/vault-secrets
[alloy-scenarios]: https://github.com/grafana/alloy-scenarios

---
canonical: https://grafana.com/docs/alloy/latest/shared/scenarios/databases-and-caches/
description: Shared content, Alloy scenarios, Databases and caches
headless: true
---

<!--
This file is generated from scenarios.yml in https://github.com/grafana/alloy-scenarios.
Don't edit it directly. Edit scenarios.yml in alloy-scenarios. A weekly workflow
in grafana/alloy copies the change into the Alloy documentation.
-->

| Scenario | Description |
| -------- | ----------- |
| [Elasticsearch monitoring][] | Monitor Elasticsearch cluster health, node status, and performance metrics. |
| [Memcached monitoring][] | Monitor Memcached instance metrics, including connections, memory usage, and command performance. |
| [Microsoft SQL Server monitoring][] | Monitor SQL Server file sizes, batch requests, and connections with `prometheus.exporter.mssql`, plus app-specific metrics from a custom `query_config`. |
| [MongoDB monitoring][] | Monitor MongoDB op-counters, connection pool, and replica-set replication metrics with `prometheus.exporter.mongodb`. Runs a single-node replica set with an insert load generator. |
| [MySQL monitoring][] | Monitor MySQL database server metrics and performance indicators. |
| [PostgreSQL monitoring][] | Monitor PostgreSQL transaction statistics, connections, and server configuration. |
| [RabbitMQ monitoring][] | Monitor RabbitMQ queue, connection, and channel metrics plus broker container logs. |
| [Redis monitoring][] | Monitor Redis instance metrics, including connections, memory usage, and command throughput. |

Refer to the [alloy-scenarios][] repository for the full list of scenarios.

[Elasticsearch monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/elasticsearch-monitoring
[Memcached monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/memcached-monitoring
[Microsoft SQL Server monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/mssql-monitoring
[MongoDB monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/mongodb-monitoring
[MySQL monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/mysql-monitoring
[PostgreSQL monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/postgres-monitoring
[RabbitMQ monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/rabbitmq-monitoring
[Redis monitoring]: https://github.com/grafana/alloy-scenarios/tree/main/redis-monitoring
[alloy-scenarios]: https://github.com/grafana/alloy-scenarios

---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-tcp-logs/
description: Learn how to use Grafana Alloy to monitor TCP logs
menuTitle: Monitor TCP logs
title: Monitor TCP logs with Grafana Alloy
weight: 300
---

# Monitor TCP logs with {{% param "FULL_PRODUCT_NAME" %}}

## Before you begin

* Docker
* Docker compose
* Git

## Clone the repository

Clone the {{< param "PRODUCT_NAME" >}} scenarios repository.

```shell
git clone https://github.com/grafana/alloy-scenarios.git
```

## Deploy the monitoring stack

Start Docker to deploy the monitoring stack.

```shell
cd alloy-scenarios/logs-tcp
docker-compose up -d
```

## Access the {{% param "PRODUCT_NAME" %}} UI

Open your browser and navigate to [`http://localhost:12345`][http://localhost:12345].

## Access the Grafana UI

Open your browser and navigate to [`http://localhost:3000`][http://localhost:3000].

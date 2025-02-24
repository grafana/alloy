---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-syslog-messages/
description: Learn how to use Grafana Alloy to monitor non-RFC5424 compliant syslog messages
menuTitle: Monitor syslog messages
title: Monitor non-RFC5424 compliant syslog messages with Grafana Alloy
weight: 500
---

# Monitor non-RFC5424 compliant syslog messages with {{% param "FULL_PRODUCT_NAME" %}}

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
cd alloy-scenarios/syslog
docker-compose up -d
```

## Access the {{% param "PRODUCT_NAME" %}} UI

Open your browser and navigate to [`http://localhost:12345`](http://localhost:12345).

## Access the Grafana UI

Open your browser and navigate to [`http://localhost:3000`](http://localhost:3000).

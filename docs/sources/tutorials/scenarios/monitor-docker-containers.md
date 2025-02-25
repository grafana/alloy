---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-docker-containers/
description: Learn how to use Grafana Alloy to monitor Docker containers
menuTitle: Monitor Docker
title: Monitor Docker containers with Grafana Alloy
weight: 200
---

# Monitor Docker containers with {{% param "FULL_PRODUCT_NAME" %}}

## Before you begin

* Docker
* Git

## Clone the repository

Clone the {{< param "PRODUCT_NAME" >}} scenarios repository.

```shell
git clone https://github.com/grafana/alloy-scenarios.git
```

## Deploy the Grafana stack

Start Docker to deploy the monitoring stack.

```shell
cd alloy-scenarios/docker-monitoring
docker compose up -d
```

## Access the {{% param "PRODUCT_NAME" %}} UI

Open your browser and navigate to [`http://localhost:12345`](http://localhost:12345).

## Access the Grafana UI

Open your browser and navigate to [`http://localhost:3000`](http://localhost:3000).

## Shut down the Grafana stack

Stop docker to shut down the Grafana stack.

```shell
docker compose down
```

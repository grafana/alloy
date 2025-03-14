---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-tcp-logs/
description: Learn how to use Grafana Alloy to monitor TCP logs
menuTitle: Monitor TCP logs
title: Monitor TCP logs with Grafana Alloy
weight: 300
---

# Monitor TCP logs with {{% param "FULL_PRODUCT_NAME" %}}

## Before you begin

This example requires:

* Docker
* Git

{{< admonition type="note" >}}
The `docker` commands require administrator privileges.
{{< /admonition >}}

## Clone the repository

Clone the {{< param "PRODUCT_NAME" >}} scenarios repository.

```shell
git clone https://github.com/grafana/alloy-scenarios.git
```

## Deploy the Grafana stack

Start Docker to deploy the Grafana stack.

```shell
cd alloy-scenarios/logs-tcp
docker compose up -d
```

You can check the status of the Docker containers by running the following command.

```shell
docker ps
```

## Monitor the health of your {{% param "PRODUCT_NAME" %}} deployment

To monitor the health of your {{< param "PRODUCT_NAME" >}} deployment, open your browser and navigate to [`http://localhost:12345`](http://localhost:12345).

Refer to [Debug Grafana Alloy](https://grafana.com/docs/alloy/latest/troubleshoot/debug/) for more information about the {{< param "PRODUCT_NAME" >}} UI.

## Visualise your data

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualise your metrics and logs, open your browser and navigate to [`http://localhost:3000/dashboards`](http://localhost:3000/dashboards).

To explore metrics, open your browser and navigate to [http://localhost:3000/explore/metrics](http://localhost:3000/explore/metrics).

To use the Grafana Logs Drilldown, open your browser and navigate to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

## Shut down the Grafana stack

Stop Docker to shut down the Grafana stack.

```shell
docker compose down
```

## Understand the {{% param "PRODUCT_NAME" %}} configuration

```alloy
livedebugging {
  enabled = true
}

loki.source.api "loki_push_api" {
    http {
        listen_address = "0.0.0.0"
        listen_port = 9999
    }
    forward_to = [
        loki.process.lables.receiver,
    ]
}

loki.process "lables" {
    stage.json {
      expressions = { "extracted_service" = "service_name", 
                      "extracted_code_line" = "code_line", 
                      "extracted_server" = "server_id", 
                    }
    }

  stage.labels {
    values = {
      "service_name" = "extracted_service",
    }
  }

  stage.structured_metadata {
    values = {
      "code_line" = "extracted_code_line",
      "server" = "extracted_server",
    }
}

forward_to = [loki.write.local.receiver]

}

loki.write "local" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
```

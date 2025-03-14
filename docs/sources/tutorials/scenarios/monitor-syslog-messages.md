---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-syslog-messages/
description: Learn how to use Grafana Alloy to monitor non-RFC5424 compliant syslog messages
menuTitle: Monitor syslog messages
title: Monitor non-RFC5424 compliant syslog messages with Grafana Alloy
weight: 500
---

# Monitor non-RFC5424 compliant syslog messages with {{% param "FULL_PRODUCT_NAME" %}}

## Before you begin

This example requires

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
cd alloy-scenarios/syslog
docker compose up -d
```

## View the {{% param "PRODUCT_NAME" %}} UI

Open your browser and navigate to [`http://localhost:12345`](http://localhost:12345).

With the Alloy UI, you can monitor the health of your Alloy deployment.
Refer to [Debug Grafana Alloy](https://grafana.com/docs/alloy/latest/troubleshoot/debug/) for more information about the {{< param "PRODUCT_NAME" >}} UI.

## Use the Grafana UI

Open your browser and navigate to [`http://localhost:3000`](http://localhost:3000).

With the Grafana UI, you can create your own dashboards to create queries and visualize any aspect of your Docker container metrics and logs.
Refer to [Build your first dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) for detailed information about dashboards in Grafana.

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

loki.source.syslog "local" {
  listener {
    address  = "0.0.0.0:51893"
    labels   = { component = "loki.source.syslog", protocol = "tcp" }
  }

  listener {
    address  = "0.0.0.0:51898"
    protocol = "udp"
    labels   = { component = "loki.source.syslog", protocol = "udp"}
  }

  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
```

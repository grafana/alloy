---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-syslog-messages/
description: Learn how to use Grafana Alloy to monitor non-RFC5424 compliant syslog messages
menuTitle: Monitor syslog messages
title: Monitor non-RFC5424 compliant syslog messages with Grafana Alloy
weight: 500
---

# Monitor non-RFC5424 compliant syslog messages with {{% param "FULL_PRODUCT_NAME" %}}

Non-RFC5424 compliant syslog messages are messages that deviate from the structured format defined in RFC 5424.
This is often due to legacy systems or custom implementations, and can cause parsing issues.
You can use {{< param "PRODUCT_NAME" >}} to collect these non-RFC5424 compliant syslog messages.

The `alloy-scenarios` repository provides series of complete working examples of {{< param "PRODUCT_NAME" >}} deployments.
You can clone the repository and use the example deployments to understand how {{< param "PRODUCT_NAME" >}} can collect, process, and export telemetry signals.

In this example scenario, {{< param "PRODUCT_NAME" >}} listens for syslog messages over TCP or UDP connections and forwards them to a Loki destination.

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

You can check the status of the Docker containers by running the following command.

```shell
docker ps
```

## Monitor the health of your {{% param "PRODUCT_NAME" %}} deployment

To monitor the health of your {{< param "PRODUCT_NAME" >}} deployment, open your browser and navigate to [http://localhost:12345](http://localhost:12345).

Refer to [Debug Grafana Alloy](https://grafana.com/docs/alloy/latest/troubleshoot/debug/) for more information about the {{< param "PRODUCT_NAME" >}} UI.

## Visualise your data

To explore metrics, open your browser and navigate to [http://localhost:3000/explore/metrics](http://localhost:3000/explore/metrics).

To use the Grafana Logs Drilldown, open your browser and navigate to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualise your metrics and logs, open your browser and navigate to [`http://localhost:3000/dashboards`](http://localhost:3000/dashboards).

## Shut down the Grafana stack

Stop Docker to shut down the Grafana stack.

```shell
docker compose down
```

## Understand the {{% param "PRODUCT_NAME" %}} configuration

This example uses a `config.alloy` file to configure the {{< param "PRODUCT_NAME" >}} components for logging.
You can find the `config.alloy` file used in this example in your cloned repository at `alloy-scenarios/syslog/`.

`livedebugging` is included in the configuration so you can stream real-time data to the {{< param "PRODUCT_NAME" >}} UI.

### Configure debugging

Livedebugging streams real-time data from your components directly to the Alloy UI.
Refer to the [Troubleshooting documentation][troubleshooting] for more information about how you can use this feature in the {{< param "PRODUCT_NAME" >}} UI.

[troubleshooting]: https://grafana.com/docs/alloy/latest/troubleshoot/debug/#live-debugging-page

#### `livedebugging`

`livedebugging` is disabled by default.
It must be explicitly enabled through the `livedebugging` configuration block to make the debugging data visible in the {{< param "PRODUCT_NAME" >}} UI.

```alloy
livedebugging {
  enabled = true
}
```

### Configure logging

The logging configuration in this example requires two components, `loki.source.syslog` and `loki.write`.

#### `loki.source.syslog`

The [`loki.source.syslog`][loki.source.syslog] component listens for syslog messages over TCP or UDP connections and forwards them to other Loki components.
In this example, the component needs the following arguments:

* `address`: The host and port address to listen to for syslog messages.
* `protocol`: The protocol to listen to for syslog messages. The default is TCP.
* `labels`: The labels to associate with each received syslog record.
* `forward_to`: The list of receivers to send log entries to.

```alloy
loki.source.syslog "local" {
  listener {
    address  = "0.0.0.0:51893"
    labels   = { component = "loki.source.syslog", protocol = "tcp" }
  }

  listener {
    address  = "0.0.0.0:51898"
    protocol = "udp"
    labels   = { component = "loki.source.syslog", protocol = "udp" }
  }

  forward_to = [loki.write.local.receiver]
}
```

#### `loki.write`

The [`loki.write`][loki.write] component writes the logs out to a Loki destination.
In this example, the component needs the following argument:

* `url`: Defines the full URL endpoint in Loki to send logs to.

```alloy
loki.write "local" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
```

[loki.source.syslog]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.source.syslog/
[loki.write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.write/

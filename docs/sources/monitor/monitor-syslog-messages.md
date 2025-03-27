---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-syslog-messages/
description: Learn how to use Grafana Alloy to monitor RFC5424 compliant syslog messages
menuTitle: Monitor syslog messages
title: Monitor RFC5424 compliant syslog messages with Grafana Alloy
weight: 400
---

# Monitor RFC5424 compliant syslog messages with {{% param "FULL_PRODUCT_NAME" %}}

RFC5424 compliant syslog messages follow a well defined structured and standardized way to log messages.
These logs include common fields such as priority, timestamp, hostname, application name, process ID, message ID, structured data, and the actual message.
You can use {{< param "PRODUCT_NAME" >}} to collect your logs, forward them to a Grafana stack, and create a Grafana dashboard to monitor your system behavior.

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

## Clone and deploy the example

Perform the following steps to clone the scenarios repository and deploy the monitoring example.

1. Clone the {{< param "PRODUCT_NAME" >}} scenarios repository.

   ```shell
   git clone https://github.com/grafana/alloy-scenarios.git
   ```

1. Start Docker to deploy the Grafana stack.

   ```shell
   cd alloy-scenarios/syslog
   docker compose up -d
   ```

   You can check the status of the Docker containers by running the following command.

   ```shell
   docker ps
   ``

1. (Optional) When you finish exploring this example, you can stop Docker to shut down the Grafana stack.

   ```shell
   docker compose down
   ```

## Monitor and visualize your data

You can use Grafana to view the health of your deployment and visualize your data.

### Monitor {{% param "PRODUCT_NAME" %}}

To monitor the health of your {{< param "PRODUCT_NAME" >}} deployment, open your browser and navigate to [http://localhost:12345](http://localhost:12345).

Refer to [Debug Grafana Alloy](https://grafana.com/docs/alloy/latest/troubleshoot/debug/) for more information about the {{< param "PRODUCT_NAME" >}} UI.

### Visualise your data

To use the Grafana Logs Drilldown, open your browser and navigate to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualise your metrics and logs, open your browser and navigate to [`http://localhost:3000/dashboards`](http://localhost:3000/dashboards).

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

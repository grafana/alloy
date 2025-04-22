---
canonical: https://grafana.com/docs/alloy/latest/monitor/monitor-syslog-messages/
description: Learn how to use Grafana Alloy to monitor RFC5424 compliant syslog messages
menuTitle: Monitor syslog messages
title: Monitor RFC5424-compliant syslog messages with Grafana Alloy
weight: 400
---

# Monitor RFC5424-compliant syslog messages with {{% param "FULL_PRODUCT_NAME" %}}

RFC5424-compliant syslog messages follow a well-defined, standardized structure for logging.
These logs include fields such as priority, timestamp, hostname, application name, process ID, message ID, structured data, and the actual message.
With {{< param "PRODUCT_NAME" >}}, you can collect your logs, forward them to a Grafana stack, and create dashboards to monitor your system behavior.

The [`alloy-scenarios`][scenarios] repository contains complete examples of {{< param "PRODUCT_NAME" >}} deployments.
Clone the repository and use the examples to understand how {{< param "PRODUCT_NAME" >}} collects, processes, and exports telemetry signals.

In this example scenario, {{< param "PRODUCT_NAME" >}} listens for syslog messages over TCP or UDP connections and forwards them to a Loki destination.

[scenarios]: https://github.com/grafana/alloy-scenarios/

## Before you begin

Ensure you have the following:

* [Docker](https://www.docker.com/)
* [Git](https://git-scm.com/)

{{< admonition type="note" >}}
You need administrator privileges to run `docker` commands.
{{< /admonition >}}

## Clone and deploy the example

Follow these steps to clone the scenarios repository and deploy the monitoring example:

1. Clone the {{< param "PRODUCT_NAME" >}} scenarios repository.

   ```shell
   git clone https://github.com/grafana/alloy-scenarios.git
   ```

1. Start Docker to deploy the Grafana stack.

   ```shell
   cd alloy-scenarios/syslog
   docker compose up -d
   ```

   Verify the status of the Docker containers:

   ```shell
   docker ps
   ```

1. (Optional) Stop Docker to shut down the Grafana stack when you finish exploring this example.

   ```shell
   docker compose down
   ```

## Monitor and visualize your data

Use Grafana to monitor your deployment's health and visualize your data.

### Monitor {{% param "PRODUCT_NAME" %}}

To monitor the health of your {{< param "PRODUCT_NAME" >}} deployment, open your browser and go to [http://localhost:12345](http://localhost:12345).

For more information about the {{< param "PRODUCT_NAME" >}} UI, refer to [Debug Grafana Alloy](https://grafana.com/docs/alloy/latest/troubleshoot/debug/).

### Visualize your data

To use the Grafana Logs Drilldown, open your browser and go to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualize metrics and logs, open your browser and go to [http://localhost:3000/dashboards](http://localhost:3000/dashboards).

## Understand the {{% param "PRODUCT_NAME" %}} configuration

This example uses a `config.alloy` file to configure {{< param "PRODUCT_NAME" >}} components for logging.
You can find the `config.alloy` file in the cloned repository at `alloy-scenarios/syslog/`.

The configuration includes `livedebugging` to stream real-time data to the {{< param "PRODUCT_NAME" >}} UI.

### Configure `livedebugging`

Livedebugging streams real-time data from components directly to the {{< param "PRODUCT_NAME" >}} UI.
Refer to the [Troubleshooting documentation][troubleshooting] for more details about this feature.

[troubleshooting]: https://grafana.com/docs/alloy/latest/troubleshoot/debug/#live-debugging-page

#### `livedebugging`

`livedebugging` is disabled by default.
Enable it explicitly through the `livedebugging` configuration block to make debugging data visible in the {{< param "PRODUCT_NAME" >}} UI.

```alloy
livedebugging {
  enabled = true
}
```

### Configure logging

The logging configuration in this example requires two components:

* `loki.source.syslog`
* `loki.write`

#### `loki.source.syslog`

The [`loki.source.syslog`][loki.source.syslog] component listens for syslog messages over TCP or UDP connections and forwards them to other Loki components.
In this example, the component requires the following arguments:

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

The [`loki.write`][loki.write] component writes logs to a Loki destination.
In this example, the component requires the following arguments:

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

---
canonical: https://grafana.com/docs/alloy/latest/monitor/monitor-logs-over-tcp/
description: Learn how to use Grafana Alloy to monitor TCP logs.
menuTitle: Monitor TCP logs
title: Monitor TCP logs with Grafana Alloy
weight: 350
---

# Monitor TCP logs with {{% param "FULL_PRODUCT_NAME" %}}

Writing logs over a network using raw TCP is a widely used method for transmitting log data between systems.
This method provides a direct, connection-oriented way to send logs from a client, such as an application or logging agent, to a remote server, such as a log aggregator or central logging system.
With {{< param "PRODUCT_NAME" >}}, you can collect your logs, forward them to a Grafana stack, and create dashboards to monitor your system behavior.

The [`alloy-scenarios`][scenarios] repository contains complete working examples of {{< param "PRODUCT_NAME" >}} deployments.
Clone the repository and use the example deployments to understand how {{< param "PRODUCT_NAME" >}} collects, processes, and exports telemetry signals.

In this example scenario, {{< param "PRODUCT_NAME" >}} uses a TCP endpoint to collect logs written by an application as a JSON payload and forwards them to a Loki destination.

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

1. Clone the {{< param "PRODUCT_NAME" >}} scenarios repository:

   ```shell
   git clone https://github.com/grafana/alloy-scenarios.git
   ```

2. Start Docker to deploy the Grafana stack:

   ```shell
   cd alloy-scenarios/logs-tcp
   docker compose up -d
   ```

   Verify the status of the Docker containers:

   ```shell
   docker ps
   ```

3. (Optional) Stop Docker to shut down the Grafana stack when you finish exploring this example:

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

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualize your metrics and logs, open your browser and go to [http://localhost:3000/dashboards](http://localhost:3000/dashboards).

## Understand the {{% param "PRODUCT_NAME" %}} configuration

This example uses a `config.alloy` file to configure the {{< param "PRODUCT_NAME" >}} components for logging.
You can find the `config.alloy` file in the cloned repository at `alloy-scenarios/logs-tcp/`.

The configuration includes `livedebugging` to stream real-time data to the {{< param "PRODUCT_NAME" >}} UI.

### Configure `livedebugging`

`livedebugging` streams real-time data from your components directly to the {{< param "PRODUCT_NAME" >}} UI.
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

The logging configuration in this example requires three components:

* `loki.source.api`
* `loki.process`
* `loki-write`

#### `loki.source.api`

The [`loki.source.api`][loki.source.api] component receives log entries over HTTP and forwards them to other Loki components.
In this example, the component requires the following arguments:

* `listen_address`: The network address the server listens to for new connections. Setting this argument to `0.0.0.0` tells the server to listen on all IP addresses.
* `listen_port`: The port the server listens to for new connections.
* `forward_to`: The list of receivers to send log entries to.

```alloy
loki.source.api "loki_push_api" {
    http {
        listen_address = "0.0.0.0"
        listen_port = 9999
    }
    forward_to = [
        loki.process.labels.receiver,
    ]
}
```

#### `loki.process`

The [`loki.process`][loki.process] component receives log entries from other Loki components, applies processing stages, and forwards the results to the list of receivers.
In this example, the component requires the following arguments:

* `expressions`: Key-value pairs defining the name of the data extracted and the value it's populated with.
* `values`: Key-value pairs defining the label to set and how to look them up.
* `forward_to`: The list of receivers to send log entries to.

```alloy
loki.process "labels" {
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
```

#### `loki-write`

The [`loki.write`][loki.write] component writes the logs to a Loki destination.
In this example, the component requires the following argument:

* `url`: Defines the full URL endpoint in Loki to send logs to.

```alloy
loki.write "local" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
```

[loki.source.api]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.source.api/
[loki.process]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.process/
[loki.write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.write/

---
canonical: https://grafana.com/docs/alloy/latest/monitor/monitor-windows/
description: Learn how to use Grafana Alloy to monitor Windows servers and desktops
menuTitle: Monitor Windows
title: Monitor Microsoft Windows servers and desktops with Grafana Alloy
weight: 250
---

# Monitor Microsoft Windows servers and desktops with {{% param "FULL_PRODUCT_NAME" %}}

Microsoft Windows provides tools like Performance Monitor and Event Viewer to track system performance metrics and event logs.
With {{< param "PRODUCT_NAME" >}}, you can collect your performance metrics and event logs, forward them to a Grafana stack, and create dashboards to monitor your Windows performance and events.

The [`alloy-scenarios`][scenarios] repository contains complete examples of {{< param "PRODUCT_NAME" >}} deployments.
Clone the repository and use the examples to understand how {{< param "PRODUCT_NAME" >}} collects, processes, and exports telemetry signals.

In this example scenario, {{< param "PRODUCT_NAME" >}} collects Windows performance metrics and event logs and forwards them to a Loki destination.

[scenarios]: https://github.com/grafana/alloy-scenarios/

## Before you begin

Ensure you have the following:

* [Docker](https://www.docker.com/)
* [Git](https://git-scm.com/)
* A Windows Server or Desktop. This scenario monitors a computer running Windows.
* Windows administrator access. You need administrator access to install {{< param "PRODUCT_NAME" >}} and configure it to collect metrics and logs.

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
   cd alloy-scenarios/windows
   docker compose up -d
   ```

   Verify the status of the Docker containers:

   ```shell
   docker ps
   ```

1. [Install {{< param "PRODUCT_NAME" >}}][install] on Windows.

1. Replace the default `config.alloy` file with the preconfigured `config.alloy` file included in the `alloy-scenarios/windows` directory.
   For detailed steps explaining how to stop and start the {{< param "PRODUCT_NAME" >}} service, refer to [Configure {{< param "PRODUCT_NAME" >}} on Windows][configure].

   1. Stop the {{< param "PRODUCT_NAME" >}} service.
   1. Replace the `config.alloy` file in `C:\Program Files\GrafanaLabs\Alloy` with the `config.alloy` file from the `alloy-scenarios/windows` directory.
   1. Start the {{< param "PRODUCT_NAME" >}} service.

1. (Optional) To access the {{< param "PRODUCT_NAME" >}} UI from a remote computer, add `--server.http.listen-addr=0.0.0.0:12345` to the {{< param "PRODUCT_NAME" >}} runtime arguments.
   For detailed steps explaining how to update this command-line argument, refer to [Expose the UI to other machines][expose].
   This step makes the {{< param "PRODUCT_NAME" >}} UI available at `http://<WINDOWS_IP_ADDRESS>:12345`.

1. (Optional) Stop Docker to shut down the Grafana stack when you finish exploring this example.

   ```shell
   docker compose down
   ```

[install]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/set-up/install/windows/
[expose]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/configure/windows/#expose-the-ui-to-other-machines
[configure]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/configure/windows/

## Monitor and visualize your data

Use Grafana to monitor your deployment's health and visualize your data.

### Monitor {{% param "PRODUCT_NAME" %}}

To monitor the health of your {{< param "PRODUCT_NAME" >}} deployment, open your browser and go to [http://localhost:12345](http://localhost:12345).

For more information about the {{< param "PRODUCT_NAME" >}} UI, refer to [Debug Grafana Alloy](https://grafana.com/docs/alloy/latest/troubleshoot/debug/).

### Visualize your data

To explore metrics, open your browser and go to [http://localhost:3000/explore/metrics](http://localhost:3000/explore/metrics).

To use the Grafana Logs Drilldown, open your browser and go to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualize metrics and logs, open your browser and go to [http://localhost:3000/dashboards](http://localhost:3000/dashboards).

## Understand the {{% param "PRODUCT_NAME" %}} configuration

This example uses a `config.alloy` file to configure the {{< param "PRODUCT_NAME" >}} components for metrics and logging.
You can find the `config.alloy` file in the cloned repository at `alloy-scenarios/windows/`.

The configuration includes `livedebugging` to stream real-time data to the {{< param "PRODUCT_NAME" >}} UI.

### Configure metrics

The metrics configuration in this example requires three components:

* `prometheus.exporter.windows`
* `prometheus.scrape`
* `prometheus.remote_write`

#### `prometheus.exporter.windows`

The [`prometheus.exporter.windows`][prometheus.exporter.windows] component exposes hardware and OS metrics for Windows-based systems.
In this example, the component requires the following arguments:

* `enabled_collectors`: The list of collectors to enable.

```alloy
prometheus.exporter.windows "default" {
  enabled_collectors = ["cpu","cs","logical_disk","net","os","service","system", "memory", "scheduled_task", "tcp"]
}
```

#### `prometheus.scrape`

The [`prometheus.scrape`][prometheus.scrape] component scrapes Windows metrics and forwards them to a receiver.
In this example, the component requires the following arguments:

* `targets`: The target to scrape metrics from.
* `forward_to`: The destination to forward metrics to.

```alloy
prometheus.scrape "example" {
  targets    = prometheus.exporter.windows.default.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}
```

#### `prometheus.remote_write`

The [`prometheus.remote_write`][prometheus.remote_write] component sends metrics to a Prometheus server.
In this example, the component requires the following argument:

* `url`: Defines the full URL endpoint to send metrics to.

```alloy
prometheus.remote_write "demo" {
  endpoint {
    url = "http://localhost:9090/api/v1/write"
  }
}
```

[prometheus.exporter.windows]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.exporter.windows/
[prometheus.scrape]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.scrape/
[prometheus.remote_write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.remote_write/

### Configure logging

The logging configuration in this example requires three components:

* `loki.source.windowsevent`
* `loki.process`
* `loki.write`

#### `loki.source.windowsevent`

The [`loki.source.windowsevent`][loki.source.windowsevent] component reads events from Windows Event Logs and forwards them to other Loki components.
In this example, the component requires the following arguments:

* `eventlog_name`: The event log to read from.
* `use_incoming_timestamp`: Assigns the current timestamp to the log.
* `forward_to`: The list of receivers to send log entries to.

```alloy
loki.source.windowsevent "application"  {
    eventlog_name = "Application"
    use_incoming_timestamp = true
    forward_to = [loki.process.endpoint.receiver]
}
```

```alloy
loki.source.windowsevent "System"  {
    eventlog_name = "System"
    use_incoming_timestamp = true
    forward_to = [loki.process.endpoint.receiver]
}
```

#### `loki.process`

The [`loki.process`][loki.process] component receives log entries from other Loki components, applies one or more processing stages, and forwards the results to the list of receivers.
In this example, the component requires the following arguments:

* `forward_to`: The list of receivers to send log entries to.
* `expressions`: The key-value pairs that define the name of the data extracted and the value that it's populated with.
* `values`: The key-value pairs that define the label to set and how to look them up.
* `source`: Name from extracted values map to use for the timestamp.
* `overwrite_existing`: Overwrite the existing extracted data fields.

```alloy
loki.process "endpoint" {
  forward_to = [loki.write.endpoint.receiver]
  stage.json {
      expressions = {
          message = "",
          Overwritten = "",
          source = "",
          computer = "",
          eventRecordID = "",
          channel = "",
          component_id = "",
          execution_processId = "",
          execution_processName = "",
      }
  }

  stage.structured_metadata {
      values = {
          "eventRecordID" = "",
          "channel" = "",
          "component_id" = "",
          "execution_processId" = "",
          "execution_processName" = "",
      }
  }

  stage.eventlogmessage {
      source = "message"
      overwrite_existing = true
  }

  stage.labels {
      values = {
          "service_name" = "source",
      }
  }

  stage.output {
    source = "message"
  }

}
```

#### `loki.write`

The [`loki.write`][loki.write] component writes the logs out to a Loki destination.
In this example, the component requires the following argument:

* `url`: Defines the full URL endpoint in Loki to send logs to.

```alloy
loki.write "endpoint" {
    endpoint {
        url ="http://localhost:3100/loki/api/v1/push"
    }
}
```

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

[loki.source.windowsevent]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.source.windowsevent/
[loki.process]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.process/
[loki.write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.write/

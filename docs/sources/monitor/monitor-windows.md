---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-windows/
description: Learn how to use Grafana Alloy to monitor Windows servers and desktops
menuTitle: Monitor Windows
title: Monitor Microsoft Windows servers and desktops with Grafana Alloy
weight: 250
---

# Monitor Microsoft Windows servers and desktops with {{% param "FULL_PRODUCT_NAME" %}}

Microsoft Windows provides tools like Performance Monitor and Event Viewer to track system performance metrics and event logs.
You can use {{< param "PRODUCT_NAME" >}} to collect your performance metrics and event logs, forward them to a Grafana stack, and create a Grafana dashboard to monitor your Windows performance and events.

The `alloy-scenarios` repository provides series of complete working examples of {{< param "PRODUCT_NAME" >}} deployments.
You can clone the repository and use the example deployments to understand how {{< param "PRODUCT_NAME" >}} can collect, process, and export telemetry signals.

In this example scenario {{< param "PRODUCT_NAME" >}} collects Windows performance metrics and Windows event logs, and forwards them to a Loki destination..

## Before you begin

This example requires:

* Docker
* Git
* Windows Server or Desktop. This scenario monitors a computer running Windows.
* Windows administrator access. You use administrator access to install {{< param "PRODUCT_NAME" >}} and configure it to collect metrics and logs.

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
   cd alloy-scenarios/windows
   docker compose up -d
   ```

   You can check the status of the containers by running the following command:

   ```shell
   docker ps
   ```

1. [Install {{< param "PRODUCT_NAME" >}}][install] on Windows.

1. (Optional) To access the {{< param "PRODUCT_NAME" >}} UI from a remote machine you must [update the command line arguments][change] for the {{< param "PRODUCT_NAME" >}} service to add the `--server.http.listen-addr` flag.

   ```shell
   run
   C:\Program Files\GrafanaLabs\Alloy\config.alloy
   --storage.path=C:\ProgramData\GrafanaLabs\Alloy\data
   --server.http.listen-addr=0.0.0.0:12345
   ```

   After you restart the services, you can access the {{< param "PRODUCT_NAME" >}} UI from a remote machine by going to `http://<windows-machine-ip>:12345`.

[install]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/set-up/install/windows/
[change]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/configure/windows/#change-command-line-arguments

## Configure {{% param "PRODUCT_NAME" %}} to monitor Windows

Replace the default `config.alloy` file with the preconfigured `config.alloy` file included in the `alloy-scenarios/windows` directory.

1. Stop the {{< param "PRODUCT_NAME" >}} service.

   1. Open the Windows Services manager.

      1. Right click on the Start Menu and select **Run**.
      1. Type `services.msc` and click **OK**.

   1. Right click on the service called **Alloy**.
   1. Click on **All Tasks > Stop**.

1. Replace the `config.alloy` file in `C:\Program Files\GrafanaLabs\Alloy` with the `config.alloy` file from the `alloy-scenarios/windows` directory.
1. Start the {{< param "PRODUCT_NAME" >}} service.

   1. Open the Windows Services manager.

      1. Right click on the Start Menu and select **Run**.
      1. Type `services.msc` and click **OK**.

   1. Right click on the service called **Alloy**.
   1. Click on **All Tasks > Start**.

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

To explore metrics, open your browser and navigate to [http://localhost:3000/explore/metrics](http://localhost:3000/explore/metrics).

To use the Grafana Logs Drilldown, open your browser and navigate to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualise your metrics and logs, open your browser and navigate to [`http://localhost:3000/dashboards`](http://localhost:3000/dashboards).

## Understand the {{% param "PRODUCT_NAME" %}} configuration

This example uses a `config.alloy` file to configure the {{< param "PRODUCT_NAME" >}} components for metrics and logging.
You can find the `config.alloy` file used in this example in your cloned repository at `alloy-scenarios/windows/`.

`livedebugging` is included in the configuration so you can stream real-time data to the {{< param "PRODUCT_NAME" >}} UI.

### Configure metrics

The metrics configuration in this example requires three components:

* `prometheus.exporter.windows`
* `prometheus.scrape`
* `prometheus.remote_write`

#### `prometheus.exporter.windows`

The [`prometheus.exporter.windows`][prometheus.exporter.windows] component exposes the hardware and OS metrics for Windows-based systems.
In this example, the component needs the following argument:

* `enabled_collectors`: The list of collectors to enable.

```alloy
prometheus.exporter.windows "default" {
  enabled_collectors = ["cpu","cs","logical_disk","net","os","service","system", "memory", "scheduled_task", "tcp"]
}
```

#### `prometheus.scrape`

The [`prometheus.scrape`][prometheus.scrape] component scrapes the Windows metrics and forwards them to a receiver.
In this example, the component needs the following arguments:

* `targets`: The target to scrape the metrics from.
* `forward_to`: The destination to forward the metrics to.

```alloy
prometheus.scrape "example" {
  targets    = prometheus.exporter.windows.default.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}
```

#### `prometheus.remote_write`

The [`prometheus.remote_write`][] component sends metrics to a Prometheus server.
In this example, the component needs the following arguments:

* `url`: Defines the full URL endpoint to send metrics to.

```alloy
prometheus.remote_write "demo" {
  endpoint {
    url = "http://localhost:9090/api/v1/write"
  }
}
```

### Configure logging

The logging configuration in this example requires three components:

* `loki.source.windowsevent`
* `loki.process`
* `loki.write`

#### `loki.source.windowsevent`

The [`loki.source.windowsevent`][loki.source.windowsevent] component reads events from Windows Event Logs and forwards them to other Loki components.
In this example, the component needs the following arguments:

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
In this example, the component needs the following arguments:

* `forward_to`: The list of receivers to send log entries to.
* `expressions`: The key-value pairs that define the name of the data extracted and the value that it's populated with.
* `values`: The key-value pairs that define the label to set and how to look them up.
* `souce`: Name from extracted values map to use for the timestamp.
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
In this example, the component needs the following argument:

* `url`: Defines the full URL endpoint in Loki to send logs to.

```alloy
loki.write "endpoint" {
    endpoint {
        url ="http://localhost:3100/loki/api/v1/push"
    }
}
```

### Configure `livedebugging`

`livedebugging` streams real-time data from your components directly to the Alloy UI.
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

[loki.source.windowsevent]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.source.windowsevent/
[loki.process]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.process/
[loki.write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.write/

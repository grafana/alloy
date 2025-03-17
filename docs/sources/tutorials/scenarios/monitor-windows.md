---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-syslog-messages/
description: Learn how to use Grafana Alloy to monitor Windows servers and desktops
menuTitle: Monitor Windows
title: Monitor Microsoft Windows servers and desktops with Grafana Alloy
weight: 400
---

# Monitor Microsoft Windows servers and desktops with {{% param "FULL_PRODUCT_NAME" %}}

You can use {{< param "FULL_PRODUCT_NAME" >}} to monitor Microsoft Windows servers and desktops.
This example shows you how to install {{< param "PRODUCT_NAME" >}} in Windows and configure {{< param "PRODUCT_NAME" >}} to monitor the following system attributes:

* Windows performance metrics
* Windows event logs

## Before you begin

This example requires:

* Docker
* Git
* Windows Server or Desktop. This scenario monitors a computer running Windows.
* Windows administrator access. You use administrator access to install {{< param "PRODUCT_NAME" >}} and configure it to collect metrics and logs.

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
cd alloy-scenarios/windows
docker compose up -d
```

You can check the status of the containers by running the following command:

```shell
docker ps
```

## Install {{% param "PRODUCT_NAME" %}} in Windows

Follow the instructions in the [Grafana Alloy documentation](https://grafana.com/docs/alloy/latest/set-up/install/windows/) to install {{< param "PRODUCT_NAME" >}} on your Windows machine.

Recommended steps:

* Install {{< param "PRODUCT_NAME" >}} as a Windows service.
* Use Windows Installer to install {{< param "PRODUCT_NAME" >}}.

Make sure to also checkout the [Grafana Alloy configuration](https://grafana.com/docs/alloy/latest/set-up/configuration/) documentation.

### Configure remote access to the {{% param "PRODUCT_NAME" %}} UI

If you would like access the {{< param "PRODUCT_NAME" >}} UI from a remote machine you must change the runtime arguments of the {{< param "PRODUCT_NAME" >}} service.

To configure the runtime arguments:

1. Open Registry Editor.
1. Navigate to `HKEY_LOCAL_MACHINE\SOFTWARE\GrafanaLabs\Alloy`.
1. Double click on `Arguments`
1. Change the contents to the following:

   ```shell
   run
   C:\Program Files\GrafanaLabs\Alloy\config.alloy
   --storage.path=C:\ProgramData\GrafanaLabs\Alloy\data
   --server.http.listen-addr=0.0.0.0:12345
   ```

1. Restart the {{< param "PRODUCT_NAME" >}} service.
   Search for `Services` in the start menu, find `Grafana Alloy`, right click and restart.

You should be able to access the {{< param "PRODUCT_NAME" >}} UI from a remote machine by going to `http://<windows-machine-ip>:12345`.

## Configure {{% param "PRODUCT_NAME" %}} to monitor Windows

Now that you have Grafana Alloy installed, you need to configure it to monitor your Windows machine.
Grafana Alloy will currently be running a default configuration file.
This needs to be replaced with the `config.alloy` file that is included in the `alloy-scenarios/windows` directory.
To do this:

1. Stop the {{< param "PRODUCT_NAME" >}} service.
1. Replace the `config.alloy` file in `C:\Program Files\GrafanaLabs\Alloy` with the `config.alloy` file from the `alloy-scenarios/windows` directory.
1. Start the {{< param "PRODUCT_NAME" >}} service.

## Monitor the health of your {{% param "PRODUCT_NAME" %}} deployment

To monitor the health of your {{< param "PRODUCT_NAME" >}} deployment, open your browser and navigate to [http://localhost:12345](http://localhost:12345).

Refer to [Debug Grafana Alloy](https://grafana.com/docs/alloy/latest/troubleshoot/debug/) for more information about the {{< param "PRODUCT_NAME" >}} UI.

## Visualise your data

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) to visualise your metrics and logs, open your browser and navigate to [`http://localhost:3000/dashboards`](http://localhost:3000/dashboards).

To explore metrics, open your browser and navigate to [http://localhost:3000/explore/metrics](http://localhost:3000/explore/metrics).

To use the Grafana Logs Drilldown, open your browser and navigate to [http://localhost:3000/a/grafana-lokiexplore-app](http://localhost:3000/a/grafana-lokiexplore-app).

## Understand the {{% param "PRODUCT_NAME" %}} configuration

This example requires you to configure components for metrics and logging.
`livedebugging` is included in the configuration so you can stream real-time data to the {{< param "PRODUCT_NAME" >}} UI.

### Configure Windows server metrics

The metrics configuration in this example requires three components:

* `prometheus.exporter.windows`
* `prometheus.scrape`
* `prometheus.remote_write`

#### `prometheus.exporter.windows`

```alloy
prometheus.exporter.windows "default" {
  enabled_collectors = ["cpu","cs","logical_disk","net","os","service","system", "memory", "scheduled_task", "tcp"]
}
```

#### `prometheus.scrape`

Configure a prometheus.scrape component to collect Windows metrics.

```alloy
prometheus.scrape "example" {
  targets    = prometheus.exporter.windows.default.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}
```

#### `prometheus.remote_write`

```alloy
prometheus.remote_write "demo" {
  endpoint {
    url = "http://localhost:9090/api/v1/write"
  }
}
```

### Configure Windows server logs

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

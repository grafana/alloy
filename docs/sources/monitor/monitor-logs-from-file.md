---
canonical: https://grafana.com/docs/alloy/latest/monitor/monitor-logs-from-file/
description: Learn how to use Grafana Alloy to monitor logs from a file
menuTitle: Monitor log files
title: Monitor logs from a local file with Grafana Alloy
weight: 300
---

# Monitor logs from a local file with {{% param "FULL_PRODUCT_NAME" %}}

Log files record events, activities, and usage patterns within a system, application, or network.
These files are essential for monitoring, troubleshooting, and understanding system behavior.
With {{< param "PRODUCT_NAME" >}}, you can collect your logs, forward them to a Grafana stack, and create dashboards to monitor your system behavior.

The [`alloy-scenarios`][scenarios] repository contains complete examples of {{< param "PRODUCT_NAME" >}} deployments.
Clone the repository and use the examples to understand how {{< param "PRODUCT_NAME" >}} collects, processes, and exports telemetry signals.

In this example scenario, {{< param "PRODUCT_NAME" >}} collects logs from a local file and forwards them to a Loki destination.

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
   cd alloy-scenarios/logs-file
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

To create a [dashboard](https://grafana.com/docs/grafana/latest/getting-started/build-first-dashboard/#create-a-dashboard) for visualizing metrics and logs, open your browser and go to [http://localhost:3000/dashboards](http://localhost:3000/dashboards).

## Understand the {{% param "PRODUCT_NAME" %}} configuration

This example uses a `config.alloy` file to configure the {{< param "PRODUCT_NAME" >}} components for logging.
You can find this file in the cloned repository at `alloy-scenarios/logs-file/`.

The configuration includes `livedebugging` to stream real-time data to the {{< param "PRODUCT_NAME" >}} UI.

### Configure `livedebugging`

`livedebugging` streams real-time data from your components directly to the {{< param "PRODUCT_NAME" >}} UI.
Refer to the [Troubleshooting documentation][troubleshooting] for more details about this feature.

[troubleshooting]: https://grafana.com/docs/alloy/latest/troubleshoot/debug/#live-debugging-page

#### `livedebugging`

By default, `livedebugging` is disabled.
Enable it explicitly through the `livedebugging` configuration block to make debugging data visible in the {{< param "PRODUCT_NAME" >}} UI.

```alloy
livedebugging {
  enabled = true
}
```

### Configure logging

The logging configuration in this example requires three components:

* `local.file_match`
* `loki.source.file`
* `loki.write`

#### `local.file_match`

The [`local.file_match`][local.file_match] component discovers files on the local filesystem using glob patterns.
In this example, the component requires the following arguments:

* `path_targets`: Targets to expand. Looks for glob patterns on the `__path__` key.
* `sync_period`: How often to sync the filesystem and targets. When files are added or removed, the list of targets is automatically updated based on this configuration. When files are removed, the stored position is removed in the next filesystem sync.

```alloy
local.file_match "local_files" {
    path_targets = [{"__path__" = "/temp/logs/*.log", "job" = "python", "hostname" = constants.hostname}]
    sync_period  = "5s"
}
```

#### `loki.source.file`

The [`loki.source.file`][loki.source.file] component reads log entries from files and forwards them to other Loki components.
In this example, the component requires the following arguments:

* `targets`: The list of files to read logs from.
* `forward_to`: The list of receivers to send log entries to.
* `tail_from_end`: Whether a log file is tailed from the end if a stored position isn't found.

```alloy
loki.source.file "log_scrape" {
    targets    = local.file_match.local_files.targets
    forward_to = [loki.write.local.receiver]
    tail_from_end = true
}
```

#### `loki.write`

The [`loki.write`][loki.write] component writes logs to a Loki destination.
In this example, the component requires the following argument:

* `url`: Defines the full URL endpoint in Loki to send logs to.

```alloy
loki.write "local" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
```

[local.file_match]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/local/local.file_match/
[loki.source.file]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.source.file/
[loki.write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.write/

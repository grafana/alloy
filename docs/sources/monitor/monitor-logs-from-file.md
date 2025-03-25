---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-logs-from-file/
description: Learn how to use Grafana Alloy to monitor logs from a file
menuTitle: Monitor log files
title: Monitor logs from a local file with Grafana Alloy
weight: 300
---

# Monitor logs from a local file with {{% param "FULL_PRODUCT_NAME" %}}

Log files are generated records that document events, activities, and usage patterns within a system, application, or network.
These log files serve as a crucial tool for monitoring, troubleshooting, and understanding system behavior.
You can use {{< param "PRODUCT_NAME" >}} to collect your logs, forward them to a Grafana stack, and create a Grafana dashboard to monitor your system behavior.

The `alloy-scenarios` repository provides series of complete working examples of {{< param "PRODUCT_NAME" >}} deployments.
You can clone the repository and use the example deployments to understand how {{< param "PRODUCT_NAME" >}} can collect, process, and export telemetry signals.

In this example scenario, {{< param "PRODUCT_NAME" >}} collects logs from a local file and forwards them to a Loki destination.

## Before you begin

This example requires:

* Docker
* Git

{{< admonition type="note" >}}
The `docker` commands require administrator privileges.
{{< /admonition >}}

## Clone the scenarios repository

Clone the {{< param "PRODUCT_NAME" >}} scenarios repository.

```shell
git clone https://github.com/grafana/alloy-scenarios.git
```

## Deploy the Grafana stack

Start Docker to deploy the Grafana stack.

```shell
cd alloy-scenarios/logs-file
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
You can find the `config.alloy` file used in this example in your cloned repository at `alloy-scenarios/logs-file/`.

### Configure `livedebugging`

`livedebugging` streams real-time data from your components directly to the {{< param "PRODUCT_NAME" >}} UI.
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

The logging configuration in this example requires three components:

* `local.fule_match`
* `loki.source.file`
* `loki.write`

#### `local.file_match`

The [`local.file_match`][local.file_match] component discovers files on the local filesystem using glob patterns.
In this example, the component needs the following arguments:

* `path_targets`: Targets to expand. Looks for glob patterns on the `__path__` key.
* `sync_period`: How often to sync filesystem and targets.

```alloy
local.file_match "local_files" {
    path_targets = [{"__path__" = "/temp/logs/*.log", "job" = "python", "hostname" = constants.hostname}]
    sync_period  = "5s"
}
```

#### `loki.source.file`

The [`loki.source.file`][loki.source.file] component reads log entries from files and forwards them to other Loki components.
In this example, the component needs the following arguments:

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

[local.file_match]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/local/local.file_match/
[loki.source.file]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.source.file/
[loki.write]: https://grafana.com/docs/alloy/<ALLOY__VERSION>/reference/components/loki/loki.write/

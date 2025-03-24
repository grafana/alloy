---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-logs-from-file/
description: Learn how to use Grafana Alloy to monitor logs froma  file
menuTitle: Monitor log files
title: Monitor logs from a local file with Grafana Alloy
weight: 350
---

# Monitor logs from a local file with {{% param "FULL_PRODUCT_NAME" %}}


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

This example uses a `config.alloy` file to configure the {{< param "PRODUCT_NAME" >}} components for metrics and logging.
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

* `discovery.docker`
* `discovery.relabel`
* `loki.source.docker`
* `loki.write`

#### `discovery.docker`

The [`discovery.docker`][discovery.docker] component discovers the Docker containers and extracts the metadata.
In this example, the component needs the following argument:

* `host`: Defines the address of the Docker Daemon to connect to.

```alloy
discovery.docker "linux" {
  host = "unix:///var/run/docker.sock"
}
```

#### `discovery.relabel`

The [`discovery.relabel`][discovery.relabel] component defines a relabeling rule to create a service name from the container name.
In this example, the component needs the following arguments:

* `targets`: The targets to relabel.
  In this example, the `discovery.relabel` component is used only for its exported `relabel_rules` in the `loki.source.docker` component.
  No targets are modified, so the `targets` argument is an empty array.
* `source_labels`: The list of labels to select for relabeling.
* `regex`: A regular expression argument that, in this case, matches any string after `/`.
  Docker container names often appear with a leading slash (/) in the Prometheus automatic discovery labels.
  This expression keeps the container name.
* `target_label`: The label that's written to the target.

```alloy
discovery.relabel "logs_integrations_docker" {
  targets = []

  rule {
    source_labels = ["__meta_docker_container_name"]
    regex = "/(.*)"
    target_label = "service_name"
    }
  }
```

#### `loki.source.docker`

The [`loki.source.docker`][loki.source.docker] component collects the logs from the Docker containers.
In this example, the component needs the following arguments:

* `host`: The address of the Docker daemon.
* `targets`: The list of containers to read logs from.
* `labels`: The default set of labels to apply on entries.
* `relabel_rules`: The relabeling rules to apply on log entries.
* `forward_to`: The list of receivers to send log entries to.

```alloy
loki.source.docker "default" {
  host       = "unix:///var/run/docker.sock"
  targets    = discovery.docker.linux.targets
  labels     = {"platform" = "docker"}
  relabel_rules = discovery.relabel.logs_integrations_docker.rules
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

[discovery.docker]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/discovery/discovery.docker/
[discovery.relabel]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/discovery/discovery.relabel/
[loki.source.docker]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/loki/loki.source.docker/
[loki.write]: https://grafana.com/docs/alloy/<ALLOY__VERSION>/reference/components/loki/loki.write/

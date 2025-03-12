---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-docker-containers/
description: Learn how to use Grafana Alloy to monitor Docker containers
menuTitle: Monitor Docker
title: Monitor Docker containers with Grafana Alloy
weight: 200
---

# Monitor Docker containers with {{% param "FULL_PRODUCT_NAME" %}}

Docker containers provide statistics and logs.
You can use the `docker stats` and `docker logs` commands to show the metrics and logs but this just shows the output in a terminal and it's a fixed snapshot in time.
If you use {{< param "PRODUCT_NAME" >}} to collect the metrics and logs and forward them to a Grafana stack, you can create a Grafana dashboard to monitor your Docker container.

## Before you begin

* Install Docker
* Install Git

## Clone the scenarios repository

Clone the {{< param "PRODUCT_NAME" >}} scenarios repository.

```shell
git clone https://github.com/grafana/alloy-scenarios.git
```

## Deploy the Grafana stack

Start Docker to deploy the monitoring stack.

```shell
cd alloy-scenarios/docker-monitoring
docker compose up -d
```

## View the {{% param "PRODUCT_NAME" %}} UI

Open your browser and navigate to [`http://localhost:12345`](http://localhost:12345).

<!-- 
Add info about what the user can see in this view and a link to the relevant docs
-->

## Access the Grafana UI

Open your browser and navigate to [`http://localhost:3000`](http://localhost:3000).

<!--
Add info about dashboards
Add steps to create a basic dashboard to monitor some stats/logs that woudl be interesting to a user
-->

## Shut down the Grafana stack

Stop Docker to shut down the Grafana stack.

```shell
docker compose down
```

## Understand the {{% param "PRODUCT_NAME" %}} configuration

The {{< param "PRODUCT_NAME" >}} configuration file is split into two parts, the metrics configuration and the logging configuration.

### Configure metrics

The metrics configuration in this example requires three components, `prometheus.exporter.cadvisor`, `prometheus.scrape`, and `prometheus.remote_write`.

#### `prometheus.exporter.cadvisor`

The [`prometheus.exporter.cadvisor`][prometheus.exporter.cadvisor] component exposes the Docker container metrics.
In this example, this component needs the following arguments:

* `docker_host`: Defines the Docker endpoint.
* `storage_duration`: Sets the time that data is stored in memory.

```alloy
prometheus.exporter.cadvisor "example" {
  docker_host = "unix:///var/run/docker.sock"

  storage_duration = "5m"
}
```

#### `prometheus.scrape`

The [`prometheus.scrape`][prometheus.scrape] component scrapes the cAdvisor metrics and forwards them to a receiver.
In this example, the component needs the following arguments:

* `targets`: The target to scrape the metrics from.
* `forward_to`: The destination to forward the metrics to.
* `scrape_interval`: How frequently to scrape the target.

```alloy
prometheus.scrape "scraper" {
  targets    = prometheus.exporter.cadvisor.example.targets
  forward_to = [ prometheus.remote_write.demo.receiver ]


  scrape_interval = "10s"
}
```

#### `prometheus.remote_write`

The [`prometheus.remote_write`][prometheus.remote_write] component sends metrics to a Prometheus server.
In this example, the component needs the following arguments:

* `url`: Defines the full URL endpoint to send metrics to.

```alloy
prometheus.remote_write "demo" {
  endpoint {
    url = "http://prometheus:9090/api/v1/write"
  }
}
```

[prometheus.exporter.cadvisor]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.exporter.cadvisor/
[prometheus.scrape]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.scrape/
[prometheus.remote_write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.remote_write/

### Configure logging

The logging configuration in this example requires four components, `discovery.docker`, `discovery.relabel`, `loki.source.docker`, and `loki.write`.

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
* `regex`: A regular expression argument that, in this case, matches any string, including an empty string.
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
[loki.write]: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.write/

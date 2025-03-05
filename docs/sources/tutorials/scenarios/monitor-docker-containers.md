---
canonical: https://grafana.com/docs/alloy/latest/tutorials/scenarios/monitor-docker-containers/
description: Learn how to use Grafana Alloy to monitor Docker containers
menuTitle: Monitor Docker
title: Monitor Docker containers with Grafana Alloy
weight: 200
---

# Monitor Docker containers with {{% param "FULL_PRODUCT_NAME" %}}

## Before you begin

* Docker
* Git

## Clone the repository

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

## Understand the {{% param "PRODUCT_NAME" %}} configuration

The {{< param "PRODUCT_NAME" >}} configuration file is split into two parts, the metrics configuration and the logging configuration.

### Configure metrics

The metrics configuration requires three components, `prometheus.exporter.cadvisor`, `prometheus.scrape`, and `prometheus.remote_write`.

#### `prometheus.exporter.cadvisor`

You can use the [`prometheus.exporter.cadvisor`][prometheus.exporter.cadvisor] component to expose the Docker container metrics.
This component needs the following arguments:

* `docker_host`: Defines the Docker endpoint.
* `storage_duration`: Sets the time that data is stored in memory.

```alloy
prometheus.exporter.cadvisor "example" {
  docker_host = "unix:///var/run/docker.sock"

  storage_duration = "5m"
}
```

#### `prometheus.scrape`

You can use the [`prometheus.scrape`][prometheus.scrape] component to scrape the cAdvisor metrics and forward them to a receiver.
This component needs the following arguments:

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

You can use the [`prometheus.remote_write`][prometheus.remote_write] component to send metrics to a Prometheus server.
This component needs the following arguments:

* `url`: Defines the full URL endpoint that {{< param "PRODUCT_NAME" >}} can the send metrics to.

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

The logging configuration requires four components, `discovery.docker`, `discovery.relabel`, `loki.source.docker`, and `loki.write`.

#### `discovery.docker`

You can use the [`discovery.docker`][discovery.docker] component to discover the Docker containers and extract the metadata.
This component needs the following argument:

* `host`: Defines the address of the Docker Daemon that {{< param "PRODUCT_NAME" >}} can connect to.

```alloy
discovery.docker "linux" {
  host = "unix:///var/run/docker.sock"
}
```

#### `discovery.relabel`

You can use the [`discovery.relabel`][discovery.relabel] component to define a relabeling rule to create a service name from the container name.
This component needs the following arguments:

* `targets`: argument is left empty. XXXXXXXXXX
* `source_labels`: argument tells Alloy what label it needs to select for relabeling.
* `regex`: argument matches any string, including an empty string.
* `target_label`: XXXXXXXXXXXX

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

You can use the [`loki.source.docker`][loki.source.docker] component to collect the logs from the Docker containers.
This component needs the following arguments:

* `host`: argument XXXX
* `targets`: argument XXXX
* `labels`: argument XXXXX
* `relabel_rules`: argument XXXXXX
* `forward_to`: argument XXXXX

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

You can use the [`loki.write`][loki.write] component to tell {{< param "PRODUCT_NAME" >}} to write the logs out to a Loki destination.
This component needs the following argument:

* `url`: Defines the full URL endpoint in Loki that {{< param "PRODUCT_NAME" >}} can the send logs to.

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

## Access the {{% param "PRODUCT_NAME" %}} UI

Open your browser and navigate to [`http://localhost:12345`](http://localhost:12345).

## Access the Grafana UI

Open your browser and navigate to [`http://localhost:3000`](http://localhost:3000).

## Shut down the Grafana stack

Stop docker to shut down the Grafana stack.

```shell
docker compose down
```

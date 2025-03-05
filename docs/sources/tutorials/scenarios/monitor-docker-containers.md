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

The first part of the metrics configuration sets up the [`prometheus.exporter.cadvisor`][prometheus.exporter.cadvisor] component to expose the Docker container metrics.
The `docker_host` argument defines the Docker endpoint.
The `storage_duration` argument sets the time that data is stored in memory to `"5m"`.

```alloy
prometheus.exporter.cadvisor "example" {
  docker_host = "unix:///var/run/docker.sock"

  storage_duration = "5m"
}
```

The second part of the metrics configuration sets up the [`prometheus.scrape`][prometheus.scrape] component to scrape the cAdvisor metrics and forward them to a receiver.
The `targets` argument to scrape the metrics from the `prometheus.exporter.cadvisor` component.
The `forward_to` argument forwards the metrics to the `prometheus.remote_write` component.
The `scrape_interval` tells {{< param "PRODUCT_NAME" >}} how frequently it should scrape the target.

```alloy
prometheus.scrape "scraper" {
  targets    = prometheus.exporter.cadvisor.example.targets
  forward_to = [ prometheus.remote_write.demo.receiver ]


  scrape_interval = "10s"
}
```

The third part of the metrics configuration sets up the [`prometheus.remote_write`][prometheus.remote_write] component to send metrics to a Prometheus server.
The `url` argument in the `endpoint` block defines the full URL endpoint that {{< param "PRODUCT_NAME" >}} can the send metrics to.

```alloy
prometheus.remote_write "demo" {
  endpoint {
    url = "http://prometheus:9090/api/v1/write"
  }
}
```

[prometheus.exporter.cadvisor]: https://grafana.com/docs/alloy/<ALLOY_VERSION>>/reference/components/prometheus/prometheus.exporter.cadvisor/
[prometheus.scrape]: https://grafana.com/docs/alloy/<ALLOY_VERSION>>/reference/components/prometheus/prometheus.scrape/
[prometheus.remote_write]: https://grafana.com/docs/alloy/<ALLOY_VERSION>/reference/components/prometheus/prometheus.remote_write/

### Configure logging

The first part of the logging configuration sets up the [`discovery.docker`][discovery.docker] component to discover the Docker containers and extract the metadata.
The `host` argument defines the address of the Docker Daemon that {{< param "PRODUCT_NAME" >}} can connect to.

```alloy
discovery.docker "linux" {
  host = "unix:///var/run/docker.sock"
}
```

The second part of the logging configuration sets up the [`discovery.relabel`][discovery.relabel] component to define a relabeling rule to create a service name from the container name.
The `targets` argument is left empty. XXXXXXXXXX
The rule block defines the relabeling rules.
The `source_labels` argument tells Alloy what label it needs to select for relabeling.
The `regex` argument matches any string, including an empty string.
The `target_label` XXXXXXXXXXXX

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

The third part of the logging configuration sets up a [`loki.source.docker`][loki.source.docker] component to collect the logs from the Docker containers.
The `host` argument XXXX
The `targets` argument XXXX
The `labels` argument XXXXX
The `relabel_rules` argument XXXXXX
The `forward_to` argument XXXXX


```alloy
loki.source.docker "default" {
  host       = "unix:///var/run/docker.sock"
  targets    = discovery.docker.linux.targets
  labels     = {"platform" = "docker"}
  relabel_rules = discovery.relabel.logs_integrations_docker.rules
  forward_to = [loki.write.local.receiver]
}
```

The final part of the logging configuration sets up a [`loki.write`][loki.write] component to tell {{< param "PRODUCT_NAME" >}} to write the logs out to a Loki destination.
The `url` argument in the `endpoint` block defines the full URL endpoint in Loki that {{< param "PRODUCT_NAME" >}} can the send logs to.

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

## Access the {{% param "PRODUCT_NAME" %}} UI

Open your browser and navigate to [`http://localhost:12345`](http://localhost:12345).

## Access the Grafana UI

Open your browser and navigate to [`http://localhost:3000`](http://localhost:3000).

## Shut down the Grafana stack

Stop docker to shut down the Grafana stack.

```shell
docker compose down
```

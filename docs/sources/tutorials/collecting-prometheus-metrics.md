---
aliases:
- ./collecting-prometheus-metrics/
- /docs/grafana-cloud/agent/flow/tutorials/collecting-prometheus-metrics/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tutorials/collecting-prometheus-metrics/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tutorials/collecting-prometheus-metrics/
- /docs/grafana-cloud/send-data/agent/flow/tutorials/collecting-prometheus-metrics/
canonical: https://grafana.com/docs/agent/latest/flow/tutorials/collecting-prometheus-metrics/
description: Learn how to collect Prometheus metrics
menuTitle: Collect Prometheus metrics
title: Collect Prometheus metrics
weight: 200
---

# Collect Prometheus metrics

{{< param "PRODUCT_ROOT_NAME" >}} is a telemetry collector with the primary goal of moving telemetry data from one location to another. In this tutorial, you'll set up {{< param "PRODUCT_NAME" >}}.

## Prerequisites

* [Docker][]

## Run the example

Run the following command in a terminal window:

```bash
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/runt.sh -O && bash ./runt.sh agent.river
```

The `runt.sh` script does:

1. Downloads the configurations necessary for Mimir, Grafana, and {{< param "PRODUCT_ROOT_NAME" >}}.
2. Downloads the docker image for {{< param "PRODUCT_ROOT_NAME" >}} explicitly.
3. Runs the docker-compose up command to bring all the services up.

Allow {{< param "PRODUCT_ROOT_NAME" >}} to run for two minutes, then navigate to [Grafana][].

![Dashboard showing agent_build_info metrics](/media/docs/agent/screenshot-grafana-agent-collect-metrics-build-info.png)

This example scrapes the {{< param "PRODUCT_NAME" >}} `http://localhost:12345/metrics` endpoint and pushes those metrics to the Mimir instance.

Navigate to `http://localhost:12345/graph` to view the {{< param "PRODUCT_NAME" >}} UI.

![The User Interface](/media/docs/agent/screenshot-grafana-agent-collect-metrics-graph.png)

{{< param "PRODUCT_ROOT_NAME" >}} displays the component pipeline in a dependency graph. See [Scraping component](#scraping-component) and [Remote Write component](#remote-write-component) for details about the components used in this configuration.
Click the nodes to navigate to the associated component page. There, you can view the state, health information, and, if applicable, the debug information.

![Component information](/media/docs/agent/screenshot-grafana-agent-collect-metrics-comp-info.png)

## Scraping component

The [`prometheus.scrape`][prometheus.scrape] component is responsible for scraping the metrics of a particular endpoint and passing them on to another component.

```river
// prometheus.scrape is the name of the component and "default" is its label.
prometheus.scrape "default" {
    // Tell the scraper to scrape at http://localhost:12345/metrics.
    // The http:// and metrics are implied but able to be overwritten.
    targets = [{"__address__" = "localhost:12345"}]
    // Forward the scrape results to the receiver. In general,
    // Flow uses forward_to to tell which receiver to send results to.
    // The forward_to is an argument of prometheus.scrape.default and
    // the receiver is an exported field of prometheus.remote_write.prom.
    forward_to = [prometheus.remote_write.prom.receiver]
}
```

The `prometheus.scrape "default"` annotation indicates the name of the component, `prometheus.scrape`, and its label, `default`. All components must have a unique combination of name and if applicable label.

The `targets` [attribute][] is an [argument][]. `targets` is a list of labels that specify the target via the special key `__address__`. The scraper is targeting the {{< param "PRODUCT_NAME" >}} `/metrics` endpoint. Both `http` and `/metrics` are implied but can be overridden.

The `forward_to` attribute is an argument that references the [export][] of the `prometheus.remote_write.prom` component. This is where the scraper will send the metrics for further processing.

## Remote Write component

The [`prometheus.remote_write`][prometheus.remote_write] component is responsible for writing the metrics to a Prometheus-compatible endpoint (Mimir).

```river
prometheus.remote_write "prom" {
    endpoint {
        url = "http://mimir:9009/api/v1/push"
    }
}
```

## Running without Docker

To try out {{< param "PRODUCT_ROOT_NAME" >}} without using Docker:
1. Download {{< param "PRODUCT_ROOT_NAME" >}}.
1. Set the environment variable `AGENT_MODE=flow`.
1. Run the {{< param "PRODUCT_ROOT_NAME" >}} with `grafana-agent run <path_to_flow_config>`.


[Docker]: https://www.docker.com/products/docker-desktop
[Grafana]: http://localhost:3000/explore?orgId=1&left=%5B%22now-1h%22,%22now%22,%22Mimir%22,%7B%22refId%22:%22A%22,%22instant%22:true,%22range%22:true,%22exemplar%22:true,%22expr%22:%22agent_build_info%7B%7D%22%7D%5D

{{% docs/reference %}}
[prometheus.scrape]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.scrape.md"
[prometheus.scrape]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.scrape.md"
[attribute]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/config-language/#attributes"
[attribute]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/config-language/#attributes"
[argument]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/components"
[argument]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/components"
[export]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/concepts/components"
[export]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/concepts/components"
[prometheus.remote_write]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.remote_write.md"
[prometheus.remote_write]: "/docs/grafana-cloud/ -> /docs/grafana-cloud/send-data/agent/flow/reference/components/prometheus.remote_write.md"
{{% /docs/reference %}}
